package session

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/shopspring/decimal"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

func TestMemoryStore(t *testing.T) {
	ctx := context.Background()
	s := NewMemory()

	mustHistory := func(id string) []*schema.Message {
		h, err := s.History(ctx, id)
		if err != nil {
			t.Fatalf("History(%q): %v", id, err)
		}
		return h
	}

	if h := mustHistory("a"); h != nil {
		t.Fatalf("bo'sh sessiya nil bo'lishi kerak, got %v", h)
	}

	_ = s.Append(ctx, "a", schema.UserMessage("salom"))
	_ = s.Append(ctx, "a", schema.AssistantMessage("va alaykum", nil))
	if h := mustHistory("a"); len(h) != 2 {
		t.Fatalf("history uzunligi = %d, want 2", len(h))
	}

	// Sessiyalar izolyatsiyasi.
	if h := mustHistory("b"); h != nil {
		t.Errorf("boshqa sessiya bo'sh bo'lishi kerak, got %v", h)
	}

	// Qaytarilgan nusxa ichki holatga ta'sir qilmasligi kerak.
	h := mustHistory("a")
	h[0] = schema.UserMessage("buzilgan")
	if mustHistory("a")[0].Content != "salom" {
		t.Error("History nusxa qaytarishi kerak, ichki holat o'zgardi")
	}

	_ = s.Reset(ctx, "a")
	if h := mustHistory("a"); h != nil {
		t.Errorf("Reset'dan keyin bo'sh bo'lishi kerak, got %v", h)
	}
}

func TestMemoryStoreStateReset(t *testing.T) {
	ctx := context.Background()
	s := NewMemory()
	want := curtains.NewSessionState()
	want.Stage = curtains.StageCalc
	want.CorniceLengthM = decimal.NewFromInt(4)
	if err := s.SetState(ctx, "state", want); err != nil {
		t.Fatal(err)
	}
	got, err := s.State(ctx, "state")
	if err != nil {
		t.Fatal(err)
	}
	if got.Stage != curtains.StageCalc || !got.CorniceLengthM.Equal(want.CorniceLengthM) {
		t.Fatalf("state = %+v, want %+v", got, want)
	}
	if err := s.Reset(ctx, "state"); err != nil {
		t.Fatal(err)
	}
	got, err = s.State(ctx, "state")
	if err != nil {
		t.Fatal(err)
	}
	if got.Stage != curtains.StageNeed || !got.CorniceLengthM.IsZero() {
		t.Fatalf("Reset state'ni tozalamadi: %+v", got)
	}
}

// TestMemoryStoreOwnerIsolation — in-memory store ham Postgres bilan bir xil qoidani
// qo'llaydi: birinchi yozuvda (user_id, tenant_id) egasi qayd etiladi, keyingi
// o'qish/yozishda mosligi majburiy. Aks holda dev/e2e yotgan kod prod'dan yanada
// zaif bo'lardi va xavfsizlik nuqsoni faqat prod'da ko'rinardi.
func TestMemoryStoreOwnerIsolation(t *testing.T) {
	ctx := context.Background()
	s := NewMemory()
	owner := curtains.WithUser(ctx, curtains.UserInfo{UserID: "user-1", TenantID: "tenant-1"})
	if err := s.Append(owner, "own", schema.UserMessage("maxfiy")); err != nil {
		t.Fatal(err)
	}
	stage := curtains.NewSessionState()
	stage.Stage = curtains.StageCalc
	if err := s.SetState(owner, "own", stage); err != nil {
		t.Fatal(err)
	}

	history, err := s.History(owner, "own")
	if err != nil || len(history) != 1 || history[0].Content != "maxfiy" {
		t.Fatalf("ega tarixi = %#v, err=%v", history, err)
	}
	if state, err := s.State(owner, "own"); err != nil || state.Stage != curtains.StageCalc {
		t.Fatalf("ega state = %+v, err=%v", state, err)
	}
	if got, err := s.Owner(owner, "own"); err != nil || got != "user-1" {
		t.Fatalf("Owner = %q, err=%v", got, err)
	}

	others := map[string]context.Context{
		"boshqa sotuvchi":    curtains.WithUser(ctx, curtains.UserInfo{UserID: "user-2", TenantID: "tenant-1"}),
		"boshqa tenant":      curtains.WithUser(ctx, curtains.UserInfo{UserID: "user-1", TenantID: "tenant-2"}),
		"bo'sh user_id":      ctx,
		"faqat bo'sh bo'luq": curtains.WithUser(ctx, curtains.UserInfo{UserID: "   ", TenantID: "tenant-1"}),
	}
	for name, ctxOther := range others {
		history, err := s.History(ctxOther, "own")
		if err != nil || history != nil {
			t.Fatalf("%s: tarix ochildi: %#v, err=%v", name, history, err)
		}
		state, err := s.State(ctxOther, "own")
		if err != nil || state.Stage != curtains.StageNeed {
			t.Fatalf("%s: state ochildi: %+v, err=%v", name, state, err)
		}
		if err := s.Append(ctxOther, "own", schema.UserMessage("begona")); !errors.Is(err, errSessionOwnerMismatch) {
			t.Fatalf("%s: Append = %v, %v kutililgan", name, err, errSessionOwnerMismatch)
		}
		if err := s.SetState(ctxOther, "own", stage); !errors.Is(err, errSessionOwnerMismatch) {
			t.Fatalf("%s: SetState = %v, %v kutililgan", name, err, errSessionOwnerMismatch)
		}
	}

	// Rad etilgan urinishlar ma'lumotni buzmasligi kerak.
	history, err = s.History(owner, "own")
	if err != nil || len(history) != 1 || history[0].Content != "maxfiy" {
		t.Fatalf("ega tarixi buzilgan: %#v, err=%v", history, err)
	}
	if state, err := s.State(owner, "own"); err != nil || state.Stage != curtains.StageCalc {
		t.Fatalf("ega state buzilgan: %+v, err=%v", state, err)
	}

	// Yangi sessiya — begona so'rov ham o'z egasini belgilaydi.
	if err := s.Append(others["boshqa sotuvchi"], "other", schema.UserMessage("yangi")); err != nil {
		t.Fatal(err)
	}
	if h, err := s.History(owner, "other"); err != nil || h != nil {
		t.Fatalf("yangi sessiya begona so'rovga ko'rindi: %#v, err=%v", h, err)
	}

	// Reset egani saqlaydi (Postgres qatori o'chmaydi) — transport ruxsat darajasi
	// shu qiymatga tayanadi, session id boshqaga berilmasin.
	if err := s.Reset(owner, "own"); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Owner(owner, "own"); err != nil || got != "user-1" {
		t.Fatalf("Reset dan keyin Owner = %q, err=%v", got, err)
	}
	if h, err := s.History(owner, "own"); err != nil || h != nil {
		t.Fatalf("Reset dan keyin tarix = %#v, err=%v", h, err)
	}
}

// TestMemoryStoreOwnerlessSession — egasiz (bo'sh user_id) so'rov egaliksiz sessiya
// yaratadi va uni faqat egasiz so'rov o'qiy oladi.
func TestMemoryStoreOwnerlessSession(t *testing.T) {
	ctx := context.Background()
	s := NewMemory()
	if err := s.Append(ctx, "anon", schema.UserMessage("egasisiz")); err != nil {
		t.Fatal(err)
	}
	history, err := s.History(ctx, "anon")
	if err != nil || len(history) != 1 {
		t.Fatalf("egasisiz sessiya = %#v, err=%v", history, err)
	}
	identified := curtains.WithUser(ctx, curtains.UserInfo{UserID: "user-1", TenantID: "tenant-1"})
	if h, err := s.History(identified, "anon"); err != nil || h != nil {
		t.Fatalf("egasisiz sessiya identifikatsiyalangan so'rovga oqildi: %#v, err=%v", h, err)
	}
	if err := s.Append(identified, "anon", schema.UserMessage("egamga")); !errors.Is(err, errSessionOwnerMismatch) {
		t.Fatalf("Append = %v, %v kutililgan", err, errSessionOwnerMismatch)
	}
}
