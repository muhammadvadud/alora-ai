package curtains

import (
	"runtime"
	"testing"
	"time"
)

func largeSnapshot() Snapshot {
	s := Snapshot{Version: 1, Products: make([]Product, 10000), Variants: make([]Variant, 30000)}
	for i := range s.Products {
		id := "p" + formatInt(i)
		s.Products[i] = Product{ID: id, Name: id, VariantIDs: []string{"v" + formatInt(i*3), "v" + formatInt(i*3+1), "v" + formatInt(i*3+2)}}
		for j := 0; j < 3; j++ {
			s.Variants[i*3+j] = Variant{ID: "v" + formatInt(i*3+j), ProductID: id}
		}
	}
	return s
}

func formatInt(x int) string {
	if x == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for x > 0 {
		i--
		buf[i] = byte('0' + x%10)
		x /= 10
	}
	return string(buf[i:])
}

func BenchmarkView(b *testing.B) {
	c := NewCatalog(NewFakeClock(time.Now()), time.Hour)
	c.ApplySnapshot(largeSnapshot())
	b.ResetTimer()
	for range b.N {
		_ = c.View()
	}
}

func BenchmarkApplyDelta(b *testing.B) {
	c := NewCatalog(NewFakeClock(time.Now()), time.Hour)
	c.ApplySnapshot(largeSnapshot())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.ApplyDelta(Delta{Version: int64(i + 2), Entity: EntityProduct, EntityID: "p0", Action: ActionUpdate, Payload: Product{ID: "p0", Name: "changed"}})
	}
}

func TestCatalog_MemoryBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("xotira benchmarki short rejimida o'tkazilmaydi")
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	c := NewCatalog(NewFakeClock(time.Now()), time.Hour)
	c.ApplySnapshot(largeSnapshot())
	runtime.GC()
	runtime.ReadMemStats(&after)
	var used uint64
	if after.Alloc > before.Alloc {
		used = after.Alloc - before.Alloc
	}
	if used > 200*1024*1024 {
		t.Fatalf("katalog xotirasi %d MB, limit 200 MB", used/1024/1024)
	}
}
