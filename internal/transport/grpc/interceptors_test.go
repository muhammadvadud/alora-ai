package grpc

import (
	"testing"
	"time"
)

func TestRateLimiter_UsesConfiguredLimitAndSweeps(t *testing.T) {
	r := newRateLimiter(20)
	for i := 0; i < 20; i++ {
		if !r.allow("same") {
			t.Fatalf("request %d rejected", i)
		}
	}
	if r.allow("same") {
		t.Fatal("21st request accepted")
	}
	for i := 0; i < 1001; i++ {
		r.entries[string(rune(i+1))] = rateEntry{started: time.Now().Add(-2 * time.Minute)}
	}
	if !r.allow("new") {
		t.Fatal("new request rejected")
	}
	if len(r.entries) > 2 {
		t.Fatalf("sweep entries = %d", len(r.entries))
	}
}

func TestRateLimiter_OneInstanceSharesUnaryAndStreamBudget(t *testing.T) {
	r := newRateLimiter(20)
	for i := 0; i < 10; i++ {
		if !r.allow("session") {
			t.Fatal("unary request rejected")
		}
	}
	for i := 0; i < 10; i++ {
		if !r.allow("session") {
			t.Fatal("stream request rejected")
		}
	}
	if r.allow("session") {
		t.Fatal("shared 21st request accepted")
	}
}
