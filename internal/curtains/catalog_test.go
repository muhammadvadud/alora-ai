package curtains

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func catalogFixture() Snapshot {
	return Snapshot{
		Version:    1,
		Products:   []Product{{ID: "p1", Name: "Atlas", VariantIDs: []string{"v1"}}},
		Variants:   []Variant{{ID: "v1", ProductID: "p1", SKU: "A-1", IsActive: true}},
		Categories: []Category{{ID: "c1", Name: "Mato"}},
		Services:   []Service{{Key: "install", Name: "Montaj", Price: decimal.NewFromInt(10), IsActive: true}},
		Policy:     &DiscountPolicy{ManualMaxPercentEmployee: decimal.NewFromInt(5)},
		Prompt:     &PromptVersion{Version: 1, Content: "base"},
	}
}

func TestCatalog_SnapshotAndRead(t *testing.T) {
	clock := NewFakeClock(time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC))
	c := NewCatalog(clock, 10*time.Minute)
	c.ApplySnapshot(catalogFixture())
	v := c.View()
	if !v.Ready() || v.Version != 1 || c.Version() != 1 {
		t.Fatal("snapshot tayyor bo'lishi kerak")
	}
	if p, ok := v.Product("p1"); !ok || p.Name != "Atlas" {
		t.Fatal("mahsulot o'qilmadi")
	}
	if got := v.VariantsOf("p1"); len(got) != 1 || got[0].ID != "v1" {
		t.Fatal("variant indeksi noto'g'ri")
	}
	if _, ok := v.Service("install"); !ok {
		t.Fatal("service o'qilmadi")
	}
	if _, ok := v.DiscountPolicy(); !ok {
		t.Fatal("policy o'qilmadi")
	}
	if got, ok := v.Prompt(); !ok || got.Content != "base" {
		t.Fatal("prompt o'qilmadi")
	}
}

func TestCatalog_DeltaCreateUpdateDelete(t *testing.T) {
	c := NewCatalog(NewFakeClock(time.Now()), time.Hour)
	c.ApplySnapshot(catalogFixture())
	requireDelta := func(d Delta) {
		t.Helper()
		if err := c.ApplyDelta(d); err != nil {
			t.Fatal(err)
		}
	}
	requireDelta(Delta{Version: 2, Entity: EntityProduct, EntityID: "p2", Action: ActionCreate, Payload: Product{ID: "p2", Name: "Tul"}})
	requireDelta(Delta{Version: 3, Entity: EntityVariant, EntityID: "v2", Action: ActionCreate, Payload: Variant{ID: "v2", ProductID: "p2", IsActive: true}})
	requireDelta(Delta{Version: 4, Entity: EntityServicePrice, EntityID: "sewing", Action: ActionCreate, Payload: Service{Key: "sewing", Name: "Tikuv"}})
	requireDelta(Delta{Version: 5, Entity: EntityDiscountPolicy, Action: ActionUpdate, Payload: DiscountPolicy{ManualMaxPercentEmployee: decimal.NewFromInt(7)}})
	requireDelta(Delta{Version: 6, Entity: EntityPrompt, Action: ActionUpdate, Payload: PromptVersion{Version: 2, Content: "new"}})
	if got := c.View().VariantsOf("p2"); len(got) != 1 {
		t.Fatalf("create variant: %d", len(got))
	}
	requireDelta(Delta{Version: 7, Entity: EntityVariant, EntityID: "v2", Action: ActionDelete, Payload: VariantDeleteRef{ProductID: "p2"}})
	if got := c.View().VariantsOf("p2"); len(got) != 0 {
		t.Fatalf("delete variant: %d", len(got))
	}
	requireDelta(Delta{Version: 8, Entity: EntityServicePrice, EntityID: "sewing", Action: ActionDelete})
	if _, ok := c.View().Service("sewing"); ok {
		t.Fatal("service o'chmadi")
	}
}

func TestCatalog_DeltaVersionRules(t *testing.T) {
	c := NewCatalog(NewFakeClock(time.Now()), time.Hour)
	c.ApplySnapshot(catalogFixture())
	if err := c.ApplyDelta(Delta{Version: 3, Entity: EntityProduct, Action: ActionCreate, Payload: Product{ID: "p3"}}); !errors.Is(err, ErrVersionGap) {
		t.Fatalf("gap = %v", err)
	}
	if err := c.ApplyDelta(Delta{Version: 1, Entity: EntityProduct, Action: ActionUpdate, Payload: Product{ID: "changed"}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := c.View().Product("changed"); ok {
		t.Fatal("takroriy version qo'llandi")
	}
}

func TestCatalog_StaleAndReady(t *testing.T) {
	clock := NewFakeClock(time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC))
	c := NewCatalog(clock, 10*time.Minute)
	if c.View().Ready() || c.View().Stale {
		t.Fatal("bo'sh katalog noto'g'ri")
	}
	c.ApplySnapshot(catalogFixture())
	clock.Advance(10*time.Minute + time.Nanosecond)
	if !c.View().Stale {
		t.Fatal("stale aniqlanmadi")
	}
}

func TestCatalog_ConcurrentViewAndDelta(t *testing.T) {
	c := NewCatalog(NewFakeClock(time.Now()), time.Hour)
	c.ApplySnapshot(catalogFixture())
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 5000 {
				_ = c.View().Products()
				_ = c.View().VariantsOf("p1")
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := int64(2); i < 100; i++ {
			_ = c.ApplyDelta(Delta{Version: i, Entity: EntityProduct, EntityID: "p1", Action: ActionUpdate, Payload: Product{ID: "p1", Name: "Atlas"}})
		}
	}()
	wg.Wait()
}

func TestCatalog_VariantsOfDeterministic(t *testing.T) {
	s := catalogFixture()
	s.Variants = append(s.Variants, Variant{ID: "v0", ProductID: "p1"})
	c := NewCatalog(NewFakeClock(time.Now()), time.Hour)
	c.ApplySnapshot(s)
	got := c.View().VariantsOf("p1")
	if len(got) != 2 || got[0].ID != "v0" || got[1].ID != "v1" {
		t.Fatalf("variant tartibi: %#v", got)
	}
}

func TestCatalog_TouchResetsStale(t *testing.T) {
	clock := NewFakeClock(time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC))
	c := NewCatalog(clock, 10*time.Minute)
	c.Touch() // tayyor bo'lmagan katalogga ta'sir qilmaydi
	if c.View().Ready() {
		t.Fatal("Touch bo'sh katalogni tayyor qilmasligi kerak")
	}
	c.ApplySnapshot(Snapshot{Version: 1})
	clock.Advance(10*time.Minute + time.Nanosecond)
	if !c.View().Stale {
		t.Fatal("stale bo'lishi kerak edi")
	}
	c.Touch()
	if c.View().Stale {
		t.Fatal("Touch dan keyin stale bo'lmasligi kerak")
	}
	if c.Version() != 1 {
		t.Fatalf("Touch versiyani o'zgartirmasligi kerak: %d", c.Version())
	}
}

func TestView_Limits(t *testing.T) {
	v := &View{
		calcRules: map[string]decimal.Decimal{
			"max_cornice_length_m": decimal.RequireFromString("30"),
			"min_cornice_length_m": decimal.RequireFromString("2.0"),
			"max_height_m":         decimal.RequireFromString("5"),
			"min_height_m":         decimal.RequireFromString("0.1"),
			"max_coef":             decimal.RequireFromString("5.0"),
			"min_coef":             decimal.RequireFromString("1.5"),
		},
	}

	l := v.Limits()
	if !l.MaxLength.Equal(decimal.RequireFromString("30")) {
		t.Errorf("MaxLength xato: %v", l.MaxLength)
	}
	if !l.MinLength.Equal(decimal.RequireFromString("2.0")) {
		t.Errorf("MinLength xato: %v", l.MinLength)
	}

	// Xato chegaralar: min > max
	vBad := &View{
		calcRules: map[string]decimal.Decimal{
			"max_cornice_length_m": decimal.RequireFromString("1"),
			"min_cornice_length_m": decimal.RequireFromString("2.0"),
		},
	}
	lBad := vBad.Limits()
	if !lBad.MaxLength.Equal(MaxLengthM) {
		t.Errorf("MaxLength fallback xato: %v", lBad.MaxLength)
	}
	if !lBad.MinLength.Equal(MinLengthM) {
		t.Errorf("MinLength fallback xato: %v", lBad.MinLength)
	}
}
