// Package curtains Alora domen qatlami: katalog cache, hisob, qidiruv, holat.
// LLM, gRPC va Eino'dan mutlaqo mustaqil (rules.md §2) — faqat stdlib va decimal.
package curtains

import "time"

// Clock — vaqt manbai. Kodda time.Now() bevosita chaqirilmaydi (rules.md §3.8);
// testlarda FakeClock bilan muzlatiladi. RealClock — yagona ishlab chiqarish implementatsiyasi.
type Clock interface {
	Now() time.Time
}

// RealClock tizim vaqtini qaytaradi (ishlab chiqarish). Qiymatni saqlamaydi — state'siz.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }
