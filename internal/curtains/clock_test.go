package curtains

import (
	"sync"
	"testing"
	"time"
)

func TestRealClock_Now(t *testing.T) {
	var c Clock = RealClock{}
	before := time.Now()
	got := c.Now()
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Errorf("RealClock.Now() = %v, joriy vaqt oralig'ida bo'lishi kerak [%v, %v]", got, before, after)
	}
}

func TestFakeClock_Now_Initial(t *testing.T) {
	base := time.Date(2026, 9, 13, 10, 30, 0, 0, time.UTC)
	var c Clock = NewFakeClock(base)

	if got := c.Now(); !got.Equal(base) {
		t.Errorf("Now() = %v, want %v", got, base)
	}
}

func TestFakeClock_Set(t *testing.T) {
	base := time.Date(2026, 9, 13, 10, 30, 0, 0, time.UTC)
	other := time.Date(2026, 9, 14, 8, 0, 0, 0, time.UTC)

	fc := NewFakeClock(base)
	fc.Set(other)

	if got := fc.Now(); !got.Equal(other) {
		t.Errorf("Set() dan keyin Now() = %v, want %v", got, other)
	}
}

func TestFakeClock_Advance(t *testing.T) {
	base := time.Date(2026, 9, 13, 10, 30, 0, 0, time.UTC)
	fc := NewFakeClock(base)

	fc.Advance(10 * time.Minute)
	want := base.Add(10 * time.Minute)
	if got := fc.Now(); !got.Equal(want) {
		t.Errorf("Advance(10m) dan keyin Now() = %v, want %v", got, want)
	}

	fc.Advance(-5 * time.Minute)
	want = want.Add(-5 * time.Minute)
	if got := fc.Now(); !got.Equal(want) {
		t.Errorf("Advance(-5m) dan keyin Now() = %v, want %v", got, want)
	}
}

// TestFakeClock_Concurrent -race ostida ko'p goroutine bir vaqtda o'qishi/yozishi
// xavfsiz ekanini tekshiradi (katalog testlarida parallel faqat o'qish bor).
func TestFakeClock_Concurrent(t *testing.T) {
	fc := NewFakeClock(time.Now())

	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				_ = fc.Now()
			}
		}()
	}
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				fc.Advance(1 * time.Second)
			}
		}()
	}
	wg.Wait()

	if fc.Now().IsZero() {
		t.Fatal("soat nol qiymatga tushib qoldi")
	}
}

func TestFakeClock_ImplementsClock(t *testing.T) {
	var _ Clock = (*FakeClock)(nil)
	var _ Clock = NewFakeClock(time.Now())
	var _ Clock = RealClock{}
}
