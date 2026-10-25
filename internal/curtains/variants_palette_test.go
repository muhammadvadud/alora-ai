package curtains

import (
	"testing"
	"time"
)

func paletteView() *View {
	return buildView(Snapshot{
		Products: []Product{
			{ID: "port", Name: "Portyera", Segment: SegmentStandard, Transparency: "blackout"},
			{ID: "tul", Name: "Tul", Segment: SegmentStandard, Transparency: "transparent"},
		},
		Variants: []Variant{
			{ID: "port-grey", ProductID: "port", ColorName: "Grafit", Price: d("100"), PriceUnit: PerSqm, IsActive: true},
			{ID: "tul-1", ProductID: "tul", ColorName: "Pushti", Price: d("5"), PriceUnit: PerSqm, IsActive: true},
			{ID: "tul-2", ProductID: "tul", ColorName: "Pushti", Price: d("6"), PriceUnit: PerSqm, IsActive: true},
			{ID: "tul-3", ProductID: "tul", ColorName: "Pushti", Price: d("7"), PriceUnit: PerSqm, IsActive: true},
			{ID: "tul-4", ProductID: "tul", ColorName: "Белый", Price: d("10"), PriceUnit: PerSqm, IsActive: true},
		},
		CalcRules: []CalcRule{{Key: "coef_default", Value: d("2")}},
	}, time.Now(), 0, false)
}

func TestPlanOptions_IndependentTulColor(t *testing.T) {
	state := SessionState{CorniceLengthM: d("3"), HeightM: d("2.6")}
	plans := PlanOptions(paletteView(), state, SearchQuery{Color: "grafit", TulColor: "oq", Transparency: "blackout"})
	if len(plans[1].Parts) != 2 || plans[1].Parts[0].VariantID != "port-grey" || plans[1].Parts[1].VariantID != "tul-4" {
		t.Fatalf("portyera va tul ranglari mustaqil bo'lishi kerak: %+v", plans[1])
	}
}

func TestPlanOptions_MissingTulColorNeverSubstitutesPink(t *testing.T) {
	state := SessionState{CorniceLengthM: d("3"), HeightM: d("2.6")}
	plans := PlanOptions(paletteView(), state, SearchQuery{TulColor: "yashil"})
	if len(plans[1].Parts) != 0 {
		t.Fatalf("yo'q rang o'rniga boshqa rang yashirilmasligi kerak: %+v", plans[1])
	}
}

func TestSearch_StrictColorFiltersBeforePerProductLimit(t *testing.T) {
	cards := Search(paletteView(), SearchQuery{Layer: "tul", Color: "oq", StrictColor: true})
	if len(cards) != 1 || cards[0].VariantID != "tul-4" {
		t.Fatalf("uch mos bo'lmagan variant to'rtinchi mos rangni yashirmasin: %+v", cards)
	}
}
