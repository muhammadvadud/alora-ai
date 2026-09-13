package curtains

import (
	"strings"
	"testing"

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
