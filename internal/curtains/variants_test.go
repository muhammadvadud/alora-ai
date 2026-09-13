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
