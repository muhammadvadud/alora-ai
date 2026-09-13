package session

import (
	"context"
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
