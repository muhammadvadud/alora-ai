package curtains

import (
	"reflect"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func variantTestCatalog() *Catalog {
	d := decimal.RequireFromString
	c := NewCatalog(RealClock{}, time.Hour)
	c.ApplySnapshot(Snapshot{
		Version:    1,
		Categories: []Category{{ID: "acc", Slug: "accessories"}},
		Products: []Product{
			{ID: "eco", Name: "Eco", Segment: SegmentEconomy, Transparency: "semi", CategoryID: "f", VariantIDs: []string{"eco-v"}},
			{ID: "std", Name: "Std", Segment: SegmentStandard, Transparency: "semi", CategoryID: "f", VariantIDs: []string{"std-v"}},
			{ID: "tul", Name: "Tul", Segment: SegmentStandard, Transparency: "transparent", CategoryID: "f", VariantIDs: []string{"tul-v"}},
			{ID: "pre", Name: "Premium", Segment: SegmentPremium, Transparency: "semi", CategoryID: "f", VariantIDs: []string{"pre-v"}},
			{ID: "black", Name: "Astar", Segment: SegmentPremium, Transparency: "blackout", CategoryID: "f", VariantIDs: []string{"black-v"}},
			{ID: "acc-p", Name: "Podxvat", CategoryID: "acc", VariantIDs: []string{"acc-v"}},
		},
		Variants: []Variant{
			{ID: "eco-v", ProductID: "eco", Price: d("10"), PriceUnit: PerSqm, IsActive: true},
			{ID: "std-v", ProductID: "std", Price: d("20"), PriceUnit: PerSqm, IsActive: true},
			{ID: "tul-v", ProductID: "tul", Price: d("8"), PriceUnit: PerSqm, IsActive: true},
			{ID: "pre-v", ProductID: "pre", Price: d("30"), PriceUnit: PerSqm, IsActive: true},
			{ID: "black-v", ProductID: "black", Price: d("25"), PriceUnit: PerSqm, IsActive: true},
			{ID: "acc-v", ProductID: "acc-p", Price: d("3"), PriceUnit: PerPiece, IsActive: true},
		},
		CalcRules: []CalcRule{{Key: "coef_economy", Value: d("2")}, {Key: "coef_standard", Value: d("2.5")}, {Key: "coef_premium", Value: d("3")}},
		Fasons:    []Fason{{ID: "f1", Name: "Classic", Rooms: []string{"living"}, Parts: []FasonPart{{Role: RolePortyera, WidthShare: d("1"), HeightShare: d("1"), DefaultCoef: d("2")}}, Accessories: []FasonAccessory{{Category: "podxvat", DefaultQty: d("2")}}}},
	})
	return c
}

func TestPlanOptions_BuildsThreeDeterministicPlans(t *testing.T) {
	v := variantTestCatalog().View()
	st := SessionState{Room: "living", CorniceLengthM: decimal.RequireFromString("4"), HeightM: decimal.RequireFromString("2.6")}
	q := SearchQuery{}
	got := PlanOptions(v, st, q)
	if got[0].Tier != OptionEconomy || got[1].Tier != OptionStandard || got[2].Tier != OptionPremium {
		t.Fatalf("tierlar: %+v", got)
	}
	if len(got[0].Parts) != 1 || len(got[1].Parts) != 2 || len(got[2].Parts) < 2 {
		t.Fatalf("qatlamlar: economy=%d standard=%d premium=%d", len(got[0].Parts), len(got[1].Parts), len(got[2].Parts))
	}
	if !got[1].Coef.Equal(decimal.RequireFromString("2.5")) || !got[2].Coef.Equal(decimal.RequireFromString("3")) {
		t.Fatalf("coef: %s %s", got[1].Coef, got[2].Coef)
	}
	if !reflect.DeepEqual(got, PlanOptions(v, st, q)) {
		t.Fatal("bir xil snapshot uchun reja deterministik bo'lishi kerak")
	}
}

func TestPlanOptions_MissingRuleAndMeasurement(t *testing.T) {
	v := variantTestCatalog().View()
	st := SessionState{CorniceLengthM: decimal.RequireFromString("4"), HeightM: decimal.RequireFromString("2.6")}
	withoutRoom := PlanOptions(v, st, SearchQuery{})
	if withoutRoom[0].Why == "" {
		t.Fatal("mos segment yoki koeffitsient bo'lmasa sabab qaytishi kerak")
	}
	noSize := PlanOptions(v, SessionState{}, SearchQuery{})
	for _, p := range noSize {
		if p.Parts != nil || p.Why != "o'lcham yo'q" {
			t.Fatalf("o'lchamsiz reja: %+v", p)
		}
	}
}

// 2026-09-20 live: premium segmentda eng arzon "mato" lambrekan edi va shaffoflik
// faqat ball bo'lgani uchun portyera/tul/astar uchalasi "Lambrekan Klassik" bo'lib
// chiqdi. Qatlam qat'iy filtr, yetishmagan qatlam pastroq segmentdan olinadi.
func TestPlanOptions_PremiumLayersNeverUseLambrekanAsFabric(t *testing.T) {
	d := decimal.RequireFromString
	c := NewCatalog(RealClock{}, time.Hour)
	c.ApplySnapshot(Snapshot{
		Version:    1,
		Categories: []Category{{ID: "lam", Name: "Lambrekan", Slug: "lambrekan"}, {ID: "por", Name: "Portyera", Slug: "portyera"}, {ID: "tl", Name: "Tul", Slug: "tul"}, {ID: "as", Name: "Astar", Slug: "astar"}},
		Products: []Product{
			{ID: "lk", Name: "Lambrekan Klassik", Segment: SegmentPremium, Transparency: "semi", CategoryID: "lam", VariantIDs: []string{"lk-v"}},
			{ID: "velvet", Name: "Velvet Chek", Segment: SegmentPremium, Transparency: "blackout", CategoryID: "por", VariantIDs: []string{"velvet-v"}},
			{ID: "eco", Name: "Oddiy", Segment: SegmentEconomy, Transparency: "semi", CategoryID: "por", VariantIDs: []string{"eco-v"}},
			{ID: "std", Name: "Linen", Segment: SegmentStandard, Transparency: "semi", CategoryID: "por", VariantIDs: []string{"std-v"}},
			{ID: "tul", Name: "Organza", Segment: SegmentStandard, Transparency: "transparent", CategoryID: "tl", VariantIDs: []string{"tul-v"}},
			{ID: "astar", Name: "Podklad", Segment: SegmentStandard, Transparency: "semi", CategoryID: "as", VariantIDs: []string{"astar-v"}},
		},
		Variants: []Variant{
			{ID: "lk-v", ProductID: "lk", Price: d("120"), PriceUnit: PerSqm, IsActive: true},
			{ID: "velvet-v", ProductID: "velvet", Price: d("165"), PriceUnit: PerSqm, IsActive: true},
			{ID: "eco-v", ProductID: "eco", Price: d("18"), PriceUnit: PerSqm, IsActive: true},
			{ID: "std-v", ProductID: "std", Price: d("70"), PriceUnit: PerSqm, IsActive: true},
			{ID: "tul-v", ProductID: "tul", Price: d("30"), PriceUnit: PerSqm, IsActive: true},
			{ID: "astar-v", ProductID: "astar", Price: d("25"), PriceUnit: PerSqm, IsActive: true},
		},
		CalcRules: []CalcRule{{Key: "coef_default", Value: d("2")}},
	})
	st := SessionState{CorniceLengthM: d("2.5"), HeightM: d("1.5")}
	got := PlanOptions(c.View(), st, SearchQuery{})
	premium := got[2]
	if premium.Parts == nil {
		t.Fatalf("premium paket tuzilmadi: %s", premium.Why)
	}
	byRole := map[PartRole]string{}
	for _, p := range premium.Parts {
		byRole[p.Role] = p.VariantID
	}
	if byRole[RolePortyera] != "velvet-v" || byRole[RoleTul] != "tul-v" || byRole[RoleAstar] != "astar-v" {
		t.Fatalf("premium qatlamlar noto'g'ri: %v", byRole)
	}
	if got[1].Parts == nil || got[1].Parts[1].VariantID != "tul-v" {
		t.Fatalf("standard tul: %+v", got[1].Parts)
	}
}

// TestPlanOptions_RespectsTransparencyAndScore — blackout so'ralsa asosiy mato blackout
// bo'ladi (xona balli yuqori arzon semi mato bo'lsa ham); segmentda blackout yo'q bo'lsa
// paket bo'sh qolmaydi — ballga qaytiladi (2026-09-27 live: blackout o'rniga semi).
func TestPlanOptions_RespectsTransparencyAndScore(t *testing.T) {
	d := decimal.RequireFromString
	c := NewCatalog(RealClock{}, time.Hour)
	c.ApplySnapshot(Snapshot{
		Version: 1,
		Products: []Product{
			{ID: "eco", Name: "Eco", Segment: SegmentEconomy, Transparency: "semi", RecommendedRooms: []string{"living"}, VariantIDs: []string{"eco-v"}},
			{ID: "std-semi", Name: "Std semi", Segment: SegmentStandard, Transparency: "semi", RecommendedRooms: []string{"living"}, VariantIDs: []string{"std-semi-v"}},
			{ID: "std-black", Name: "Std black", Segment: SegmentStandard, Transparency: "blackout", VariantIDs: []string{"std-black-v"}},
			{ID: "std-black2", Name: "Std black 2", Segment: SegmentStandard, Transparency: "blackout", RecommendedRooms: []string{"living"}, VariantIDs: []string{"std-black2-v"}},
			{ID: "tul", Name: "Tul", Segment: SegmentStandard, Transparency: "transparent", VariantIDs: []string{"tul-v"}},
		},
		Variants: []Variant{
			{ID: "eco-v", ProductID: "eco", Price: d("10"), PriceUnit: PerSqm, IsActive: true},
			{ID: "std-semi-v", ProductID: "std-semi", Price: d("15"), PriceUnit: PerSqm, IsActive: true},
			{ID: "std-black-v", ProductID: "std-black", Price: d("20"), PriceUnit: PerSqm, IsActive: true},
			{ID: "std-black2-v", ProductID: "std-black2", Price: d("40"), PriceUnit: PerSqm, IsActive: true},
			{ID: "tul-v", ProductID: "tul", Price: d("8"), PriceUnit: PerSqm, IsActive: true},
		},
		CalcRules: []CalcRule{{Key: "coef_default", Value: d("2")}},
	})
	st := SessionState{Room: "living", CorniceLengthM: d("3"), HeightM: d("2.7")}
	got := PlanOptions(c.View(), st, SearchQuery{Room: "living", Transparency: "blackout"})

	if len(got[0].Parts) != 1 || got[0].Parts[0].VariantID != "eco-v" {
		t.Fatalf("economy: segmentda blackout yo'q — eco-v kutilgan, olindi %+v", got[0].Parts)
	}
	if got[0].Why != "asosiy portyera (bu segmentda blackout yo'q — mato semi)" {
		t.Fatalf("economy why: %q", got[0].Why)
	}
	if got[1].Why != "portyera va tul" {
		t.Fatalf("standard why (blackout topilgan — izoh bo'lmasligi kerak): %q", got[1].Why)
	}
	// std-black2 (blackout + xona) ball bo'yicha std-black'dan yuqori — narxidan qat'i nazar.
	if len(got[1].Parts) != 2 || got[1].Parts[0].VariantID != "std-black2-v" || got[1].Parts[1].VariantID != "tul-v" {
		t.Fatalf("standard: std-black2-v + tul-v kutilgan, olindi %+v", got[1].Parts)
	}
}
