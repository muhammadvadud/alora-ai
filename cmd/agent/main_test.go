package main

import (
	"context"
	"strings"
	"testing"
)

// TestNewStoreModeGate — gRPC (prod) rejimida DATABASE_URL yo'q bo'lsa jimgina
// in-memory store'ga o'tish TAQIQLANADI: sessiya izolyatsiyasi yo'qolardi va
// konfiguratsiya xatosi ishlayotgan prod'da sezilmasdi.
func TestNewStoreModeGate(t *testing.T) {
	ctx := context.Background()
	store, err := newStore(ctx, "", false, "grpc")
	if err == nil {
		t.Fatal("grpc rejimida DATABASE_URL bo'sh bo'lsa xato berishi kerak")
	}
	if store != nil {
		t.Fatal("xatoda store qaytarilmasligi kerak")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("xato matni sozlamani aytsin: %v", err)
	}

	for _, mode := range []string{"cli", "http"} {
		store, err := newStore(ctx, "", false, mode)
		if err != nil {
			t.Fatalf("%s rejimida in-memory ruxsat etiladi: %v", mode, err)
		}
		if store == nil {
			t.Fatalf("%s rejimida store nil", mode)
		}
	}
}
