package curtains

import (
	"sync"
	"time"
)

// FakeClock — testlar uchun sozlanadigan vaqt. Set bilan aniqlangan daqiqani qo'yadi,
// Advance bilan oldinga suradi. Mu — bir nechta goroutine o'qisa ham xavfsiz (-race).
type FakeClock struct {
	mu sync.Mutex
	t  time.Time
}

// NewFakeClock belgilangan paytdan boshlanadigan soxta soat yaratadi.
func NewFakeClock(t time.Time) *FakeClock {
	return &FakeClock{t: t}
}

// Now qo'yilgan vaqtni qaytaradi.
func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

// Set soatni aniq paytga o'rnatadi (test ssenariysining boshlanish nuqtasi).
func (c *FakeClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = t
}

// Advance soatni d orqaga oldinga suradi — "stale bo'ldi" holatini simulyatsiya qiladi.
func (c *FakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

var _ Clock = (*FakeClock)(nil)
