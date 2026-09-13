package curtains

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func TestExplain(t *testing.T) {
	res := CalcResult{
		Measurements: []MeasurementResult{{Parts: []PartResult{
			{Role: "portyera", SKU: "VB-CHK", Coef: d("2.5"), WastePercent: d("18.5")},
			{Role: "tul", SKU: "TL-1", Coef: d("1.5"), WastePercent: d("0")},
		}}},
		DiscountPercent: d("7"), TierDiscountPercent: d("2"), ManualDiscountPercent: d("5"),
	}
	got := Explain(res)
	joined := strings.Join(got, "\n")
	for _, want := range []string{"chiqindi 18.5%", "2.5 — 2 ga tushirsangiz", "chegirma 7% qo'llandi (tier 2% + qo'lda 5%)"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("kutildi %q, got:\n%s", want, joined)
		}
	}
	if strings.Contains(joined, "tul: burma") {
		t.Fatal("coef 1.5 uchun tavsiya bo'lmasligi kerak")
	}
	if len(Explain(CalcResult{})) != 0 {
		t.Fatal("bo'sh natija → bo'sh izoh")
	}
	for _, line := range got {
		if len([]rune(line)) > 120 {
			t.Fatalf("qator 120 dan uzun: %q", line)
		}
	}
}

// TestLayerOf — qatlam: teg ustun, keyin kategoriya/fabric_type/nom, default portyera.
func TestLayerOf(t *testing.T) {
	v := &View{categories: map[string]Category{"c-tul": {ID: "c-tul", Name: "Tul"}}}
	cases := []struct {
		p    Product
		want string
	}{
		{Product{Name: "Carmel Kitchen", Tags: map[string]any{"rim": true}}, "rim"},
		{Product{Name: "Oq Oddiy", CategoryID: "c-tul"}, "tul"},
		{Product{Name: "Soft", FabricType: "lining"}, "astar"},
		{Product{Name: "Kant Atlas"}, "kant"},
		{Product{Name: "Lambrekan Klassik"}, "lambrekan"},
		{Product{Name: "Velvet Milano"}, "portyera"},
	}
	for _, c := range cases {
		if got := LayerOf(v, c.p); got != c.want {
			t.Fatalf("%s: got %s want %s", c.p.Name, got, c.want)
		}
	}
}

// TestSearch_Query — nom/SKU bo'yicha qat'iy qidiruv: "Velvet Chek" faqat shu mahsulotni beradi.
func TestSearch_Query(t *testing.T) {
	v := buildView(Snapshot{
		Products: []Product{
			{ID: "p1", Name: "Velvet Chek", Segment: SegmentPremium, VariantIDs: []string{"v1"}},
			{ID: "p2", Name: "Velvet Milano", Segment: SegmentPremium, VariantIDs: []string{"v2"}},
			{ID: "p3", Name: "Tul Oq", Segment: SegmentEconomy, VariantIDs: []string{"v3"}},
		},
		Variants: []Variant{
			{ID: "v1", ProductID: "p1", SKU: "VB-CHK", ColorName: "kulrang", Price: d("165000"), PriceUnit: "per_sqm", IsActive: true},
			{ID: "v2", ProductID: "p2", SKU: "VB-MIL", ColorName: "bordo", Price: d("180000"), PriceUnit: "per_sqm", IsActive: true},
			{ID: "v3", ProductID: "p3", SKU: "TL-OQ", ColorName: "oq", Price: d("18000"), PriceUnit: "per_sqm", IsActive: true},
		},
	}, time.Now(), 0, false)
	got := Search(v, SearchQuery{Query: "velvet chek"})
	if len(got) != 1 || got[0].Name != "Velvet Chek" {
		t.Fatalf("query: %+v", got)
	}
	if n := len(Search(v, SearchQuery{Query: "velvet"})); n != 2 {
		t.Fatalf("velvet: %d", n)
	}
	if n := len(Search(v, SearchQuery{Query: "VB-MIL"})); n != 1 {
		t.Fatalf("sku: %d", n)
	}
	if n := len(Search(v, SearchQuery{Query: "atlas roma"})); n != 0 {
		t.Fatalf("yo'q mahsulot: %d", n)
	}
}
