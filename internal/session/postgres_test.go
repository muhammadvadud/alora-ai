package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

// openTestPostgres — DATABASE_URL berilgan va javob berse store ochadi (migratsiya
// idempotentligi ham shu yerda tekshiriladi), aks holda test skip qilinadi.
func openTestPostgres(t *testing.T) Store {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL berilmagan")
	}
	ctx := context.Background()
	first, err := NewPostgres(ctx, dsn, PostgresOptions{AutoMigrate: true})
	if err != nil {
		t.Skipf("Postgres mavjud emas: %v", err)
	}
	first.(*pgStore).Close()
	store, err := NewPostgres(ctx, dsn, PostgresOptions{AutoMigrate: true})
	if err != nil {
		t.Fatalf("idempotent migratsiya: %v", err)
	}
	t.Cleanup(func() { store.(*pgStore).Close() })
	return store
}

func TestPostgresStoreLifecycle(t *testing.T) {
	ctx := context.Background()
	store := openTestPostgres(t)

	id := fmt.Sprintf("session-test-%d", time.Now().UnixNano())
	owner := curtains.WithUser(ctx, curtains.UserInfo{UserID: "user-1", TenantID: "tenant-1"})
	if err := store.Append(owner, id, schema.UserMessage("bir"), schema.AssistantMessage("ikki", nil)); err != nil {
		t.Fatal(err)
	}
	want := curtains.NewSessionState()
	want.Stage = curtains.StageMeasure
	if err := store.SetState(owner, id, want); err != nil {
		t.Fatal(err)
	}
	history, err := store.History(owner, id)
	if err != nil || len(history) != 2 || history[0].Content != "bir" || history[1].Content != "ikki" {
		t.Fatalf("History = %#v, err=%v", history, err)
	}
	state, err := store.State(owner, id)
	if err != nil || state.Stage != curtains.StageMeasure {
		t.Fatalf("State = %+v, err=%v", state, err)
	}

	other := curtains.WithUser(ctx, curtains.UserInfo{UserID: "user-2", TenantID: "tenant-1"})
	private, err := store.History(other, id)
	if err != nil || private != nil {
		t.Fatalf("begona egaga tarix ochildi: %#v, err=%v", private, err)
	}
	if err := store.Reset(owner, id); err != nil {
		t.Fatal(err)
	}
	state, err = store.State(owner, id)
	if err != nil || state.Stage != curtains.StageNeed {
		t.Fatalf("Reset state = %+v, err=%v", state, err)
	}
}

// TestPostgresStoreOwnerIsolation — sessiya izolyatsiyasi: faqat ega (user_id VA
// tenant_id) o'qiy oladi. Bo'sh user_id EGALISIZ emas, egasiz sessiyaga ham
// kira olmaydi — aks holda har qanday bo'sh so'rov boshqa suhbatni o'qirdi.
func TestPostgresStoreOwnerIsolation(t *testing.T) {
	ctx := context.Background()
	store := openTestPostgres(t)
	stamp := time.Now().UnixNano()
	id := fmt.Sprintf("session-owner-%d", stamp)
	anonID := fmt.Sprintf("session-anon-%d", stamp)
	for _, sessionID := range []string{id, anonID} {
		_, err := store.(*pgStore).pool.Exec(ctx, `DELETE FROM agent.sessions WHERE session_id=$1`, sessionID)
		if err != nil {
			t.Fatalf("tozalash: %v", err)
		}
	}
	t.Cleanup(func() {
		for _, sessionID := range []string{id, anonID} {
			_, _ = store.(*pgStore).pool.Exec(context.Background(), `DELETE FROM agent.sessions WHERE session_id=$1`, sessionID)
		}
	})

	owner := curtains.WithUser(ctx, curtains.UserInfo{UserID: "user-1", TenantID: "tenant-1"})
	if err := store.Append(owner, id, schema.UserMessage("maxfiy")); err != nil {
		t.Fatal(err)
	}
	stage := curtains.NewSessionState()
	stage.Stage = curtains.StageCalc
	if err := store.SetState(owner, id, stage); err != nil {
		t.Fatal(err)
	}

	for name, ctxOther := range map[string]context.Context{
		"boshqa sotuvchi":    curtains.WithUser(ctx, curtains.UserInfo{UserID: "user-2", TenantID: "tenant-1"}),
		"boshqa tenant":      curtains.WithUser(ctx, curtains.UserInfo{UserID: "user-1", TenantID: "tenant-2"}),
		"bo'sh user_id":      ctx,
		"faqat bo'sh bo'luq": curtains.WithUser(ctx, curtains.UserInfo{UserID: "   ", TenantID: "tenant-1"}),
	} {
		history, err := store.History(ctxOther, id)
		if err != nil || history != nil {
			t.Fatalf("%s: tarix ochildi: %#v, err=%v", name, history, err)
		}
		state, err := store.State(ctxOther, id)
		if err != nil || state.Stage != curtains.StageNeed {
			t.Fatalf("%s: state ochildi: %+v, err=%v", name, state, err)
		}
		if err := store.Append(ctxOther, id, schema.UserMessage("begona")); !errors.Is(err, errSessionOwnerMismatch) {
			t.Fatalf("%s: Append = %v, %v kutililgan", name, err, errSessionOwnerMismatch)
		}
		if err := store.SetState(ctxOther, id, stage); !errors.Is(err, errSessionOwnerMismatch) {
			t.Fatalf("%s: SetState = %v, %v kutililgan", name, err, errSessionOwnerMismatch)
		}
	}

	// Rad etilgan urinishlar ma'lumotni buzmasligi kerak.
	history, err := store.History(owner, id)
	if err != nil || len(history) != 1 || history[0].Content != "maxfiy" {
		t.Fatalf("ega tarixi buzilgan: %#v, err=%v", history, err)
	}
	state, err := store.State(owner, id)
	if err != nil || state.Stage != curtains.StageCalc {
		t.Fatalf("ega state buzilgan: %+v, err=%v", state, err)
	}
	if got, err := store.Owner(owner, id); err != nil || got != "user-1" {
		t.Fatalf("Owner = %q, err=%v", got, err)
	}

	// Egasiz sessiya: bo'sh so'rov yaratadi va faqat bo'sh so'rov o'qiy oladi.
	if err := store.Append(ctx, anonID, schema.UserMessage("egasisiz")); err != nil {
		t.Fatal(err)
	}
	if h, err := store.History(ctx, anonID); err != nil || len(h) != 1 {
		t.Fatalf("egasisiz sessiya = %#v, err=%v", h, err)
	}
	identified := curtains.WithUser(ctx, curtains.UserInfo{UserID: "user-9", TenantID: "tenant-1"})
	if h, err := store.History(identified, anonID); err != nil || h != nil {
		t.Fatalf("egasisiz sessiya identifikatsiyalangan so'rovga oqildi: %#v, err=%v", h, err)
	}
}

// TestOwnerAllowsRule — egalik qarorining yagona qoidasi (DB kerak emas): yozuv yo'q
// — ruxsat, yozuv bor — faqat aniq (user_id, tenant_id) mosligi.
func TestOwnerAllowsRule(t *testing.T) {
	stored := sessionOwner{user: "u1", tenant: "t1"}
	cases := []struct {
		name     string
		stored   sessionOwner
		hasOwner bool
		want     sessionOwner
		allowed  bool
	}{
		{"yangi sessiya — hech kimga bog'lanmagan", sessionOwner{}, false, stored, true},
		{"ega o'zi", stored, true, stored, true},
		{"boshqa sotuvchi, o'sha tenant", stored, true, sessionOwner{user: "u2", tenant: "t1"}, false},
		{"o'sha sotuvchi, boshqa tenant", stored, true, sessionOwner{user: "u1", tenant: "t2"}, false},
		{"bo'sh so'rov egasiz sessiyaga", sessionOwner{}, true, sessionOwner{}, true},
		{"bo'sh so'rov egali sessiyaga (fail-open emas)", stored, true, sessionOwner{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ownerAllows(tc.stored, tc.hasOwner, tc.want); got != tc.allowed {
				t.Fatalf("ownerAllows = %v, want %v", got, tc.allowed)
			}
		})
	}
}
