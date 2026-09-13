package session

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

func TestPostgresStoreLifecycle(t *testing.T) {
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
	defer store.(*pgStore).Close()

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
