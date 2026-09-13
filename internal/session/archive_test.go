package session

import (
	"context"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

func TestMemoryArchiveOnReset(t *testing.T) {
	base := NewMemory()
	a := base.(*memoryStore)
	store := WithArchiveOnReset(base, a)
	if err := store.Append(context.Background(), "s", schema.UserMessage("salom")); err != nil {
		t.Fatal(err)
	}
	if err := store.Reset(context.Background(), "s"); err != nil {
		t.Fatal(err)
	}
	history, err := base.History(context.Background(), "s")
	if err != nil || history != nil {
		t.Fatalf("reset history = %v, err=%v", history, err)
	}
	if a.ArchivedCount() != 1 {
		t.Fatalf("archived = %d, want 1", a.ArchivedCount())
	}
	if err := a.Archive(context.Background(), "missing", "reset"); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryCleanupTTL(t *testing.T) {
	base := NewMemory().(*memoryStore)
	if err := base.Append(context.Background(), "old", schema.UserMessage("eski")); err != nil {
		t.Fatal(err)
	}
	base.updated["old"] = time.Now().Add(-48 * time.Hour)
	if err := base.Append(context.Background(), "new", schema.UserMessage("yangi")); err != nil {
		t.Fatal(err)
	}
	archived, deleted, err := base.Cleanup(context.Background(), 24*time.Hour)
	if err != nil || archived != 1 || deleted != 0 {
		t.Fatalf("cleanup = %d,%d,%v", archived, deleted, err)
	}
	if h, _ := base.History(context.Background(), "new"); len(h) != 1 {
		t.Fatal("yangi sessiya arxivlandi")
	}
	_, deleted, err = base.Cleanup(context.Background(), -time.Hour)
	if err != nil || deleted == 0 {
		t.Fatalf("arxiv cleanup deleted=%d err=%v", deleted, err)
	}
}
