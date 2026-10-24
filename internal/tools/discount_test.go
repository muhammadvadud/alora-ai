package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
	"github.com/muhammadvadud/alora-ai/internal/session"
)

// B5 (ADR-020): chegirma TARKIBI, faqat yalpi foiz emas.
//
// 2026-09-28 live: mijoz "12% qayerdan?" deb so'raganda model bosqichli va
// qo'lda chegirmani chalkashtirdi. Endi natija ikkalasini alohida ko'rsatadi.

func tierCatalog(stackWithTier bool) *curtains.Catalog {
	c := curtains.NewCatalog(curtains.RealClock{}, time.Hour)
	c.ApplySnapshot(curtains.Snapshot{
		Version: 1,
		Products: []curtains.Product{{
			ID: "p-1", Name: "Test", CategoryID: "c-fabrics", Style: "modern", Pattern: "plain",
			RecommendedRooms: []string{"living"}, Segment: curtains.SegmentStandard,
			DefaultWidthM: d("2.8"), VariantIDs: []string{vPortyera},
		}},
		Variants: []curtains.Variant{{ID: vPortyera, ProductID: "p-1", SKU: "VM-GRAFIT",
			ColorName: "grafit", WidthM: d("2.8"), Price: d("100000"), PriceUnit: curtains.PerSqm, IsActive: true}},
		CalcRules: []curtains.CalcRule{{Key: "coef_default", Value: d("2.5")}},
		Policy: &curtains.DiscountPolicy{
			ManualMaxPercentEmployee: d("5"), ManualMaxPercentManager: d("15"),
			StackWithTier: stackWithTier,
		},
		Tiers: []curtains.DiscountTier{{ID: "t1", MinAmount: d("1000000"), Percent: d("7"), IsActive: true}},
	})
	return c
}

// B5: qo'lda chegirma natijada alohida ko'rinadi, chegirma chegarasi rol bo'yicha.
func TestDiscount_ManualAndTierAreSeparate(t *testing.T) {
	dp, calc, _ := deps(t)
	dp.Catalog = tierCatalog(false)
	dp.Calc = calc
	ctx, _ := ctxFor(curtains.RoleEmployee)
	if _, err := (&curtainCalcHandler{d: dp}).do(ctx, curtainCalcArgs{CorniceLengthM: 4, HeightM: 2.6,
		Parts: []calcPart{{VariantID: vPortyera, Role: "portyera"}}}); err != nil {
		t.Fatal(err)
	}
	res, _ := (&priceQuoteHandler{d: dp}).do(ctx, priceQuoteArgs{DiscountPct: 5})
	if res.Discount == nil {
		t.Fatal("chegirma bloki yo'q")
	}
	if res.Discount.ManualPercent != "5" {
		t.Fatalf("qo'lda chegirma: %q", res.Discount.ManualPercent)
	}
	if res.Discount.AppliedPercent == "" {
		t.Fatalf("qo'llangan foiz bo'sh: %+v", res.Discount)
	}
	if res.Discount.ManualMaxPercent != "5" {
		t.Fatalf("employee chegarasi 5%% bo'lishi kerak: %q", res.Discount.ManualMaxPercent)
	}
}

// B5: stack_with_tier=false → qoida matni "KATTASI" beriladi.
func TestDiscount_RuleWhenNotStacked(t *testing.T) {
	dp, calc, _ := deps(t)
	dp.Catalog = tierCatalog(false)
	dp.Calc = calc
	ctx, _ := ctxFor(curtains.RoleEmployee)
	_, _ = (&curtainCalcHandler{d: dp}).do(ctx, curtainCalcArgs{CorniceLengthM: 4, HeightM: 2.6,
		Parts: []calcPart{{VariantID: vPortyera, Role: "portyera"}}})
	res, _ := (&priceQuoteHandler{d: dp}).do(ctx, priceQuoteArgs{DiscountPct: 5})
	if res.Discount == nil || res.Discount.Rule != "kattasi" {
		t.Fatalf("stack_with_tier=false qoida: %+v", res.Discount)
	}
}

// B5: stack_with_tier=true → qoida matni "QO'SHILADI" beriladi.
func TestDiscount_RuleWhenStacked(t *testing.T) {
	dp, calc, _ := deps(t)
	dp.Catalog = tierCatalog(true)
	dp.Calc = calc
	ctx, _ := ctxFor(curtains.RoleEmployee)
	_, _ = (&curtainCalcHandler{d: dp}).do(ctx, curtainCalcArgs{CorniceLengthM: 4, HeightM: 2.6,
		Parts: []calcPart{{VariantID: vPortyera, Role: "portyera"}}})
	res, _ := (&priceQuoteHandler{d: dp}).do(ctx, priceQuoteArgs{DiscountPct: 5})
	if res.Discount == nil || res.Discount.Rule != "qo'shiladi" {
		t.Fatalf("stack_with_tier=true qoida: %+v", res.Discount)
	}
}

// B5: mijoz rejimida chegirma chegarasi HECH QACHON chiqmaydi (rules §1.6).
func TestDiscount_NoManualMaxInCustomerMode(t *testing.T) {
	dp, calc, _ := deps(t)
	dp.Catalog = tierCatalog(false)
	dp.Calc = calc
	ctx := curtains.WithUser(session.WithID(context.Background(), "s1"),
		curtains.UserInfo{Role: curtains.RoleEmployee, TenantID: "tenant-1", CustomerMode: true})
	_, _ = (&curtainCalcHandler{d: dp}).do(ctx, curtainCalcArgs{CorniceLengthM: 4, HeightM: 2.6,
		Parts: []calcPart{{VariantID: vPortyera, Role: "portyera"}}})
	res, _ := (&priceQuoteHandler{d: dp}).do(ctx, priceQuoteArgs{DiscountPct: 5})
	if res.Discount == nil {
		t.Fatal("chegirma bloki yo'q")
	}
	if res.Discount.ManualMaxPercent != "" {
		t.Fatalf("mijoz rejimida chegirma chegarasi sir bo'lishi kerak: %q", res.Discount.ManualMaxPercent)
	}
	out, _ := json.Marshal(res)
	if strings.Contains(string(out), "manual_max_percent") {
		t.Fatalf("mijoz rejimida manual_max_percent chiqdi: %s", out)
	}
}

// B5: manager roli uchun chegirma chegarasi 15%.
func TestDiscount_ManualMaxByRole(t *testing.T) {
	dp, calc, _ := deps(t)
	dp.Catalog = tierCatalog(false)
	dp.Calc = calc
	ctx, _ := ctxFor(curtains.RoleManager)
	_, _ = (&curtainCalcHandler{d: dp}).do(ctx, curtainCalcArgs{CorniceLengthM: 4, HeightM: 2.6,
		Parts: []calcPart{{VariantID: vPortyera, Role: "portyera"}}})
	res, _ := (&priceQuoteHandler{d: dp}).do(ctx, priceQuoteArgs{DiscountPct: 5})
	if res.Discount == nil || res.Discount.ManualMaxPercent != "15" {
		t.Fatalf("manager chegarasi 15%% bo'lishi kerak: %+v", res.Discount)
	}
}

// B5: ixcham curtain_calculate natijasi ham chegirma tarkibini oladi.
// curtain_quote'ning o'zida discount_pct yo'q — chegirmani Django hisoblaydi
// (bosqichli), shuning uchun soxta kalkulyator tarkibni qaytaradi.
func TestCurtainCalculate_IncludesDiscount(t *testing.T) {
	dp, _, _ := deps(t)
	dp.Calc = &tieredCalc{}
	ctx, _ := ctxFor(curtains.RoleEmployee)
	res, _ := (&curtainCalcHandler{d: dp}).do(ctx, curtainCalcArgs{
		CorniceLengthM: 4, HeightM: 2.6, Parts: []calcPart{{VariantID: vPortyera, Role: "portyera"}},
	})
	if res.Discount == nil {
		t.Fatalf("curtain_calculate natijasida chegirma bloki yo'q: %+v", res)
	}
	if res.Discount.TierPercent != "7" || res.Discount.AppliedPercent != "7" {
		t.Fatalf("bosqichli chegirma ko'rsatilmadi: %+v", res.Discount)
	}
	if res.Discount.ManualPercent != "" {
		t.Fatalf("qo'lda chegirma bo'lmasa maydon chiqmasligi kerak: %+v", res.Discount)
	}
}

// tieredCalc — Django kabi bosqichli chegirmani o'zi qo'llaydigan soxta kalkulyator.
type tieredCalc struct{}

func (tieredCalc) Calculate(_ context.Context, in curtains.CalcInput) (curtains.CalcResult, error) {
	m := in.Measurements[0]
	res := curtains.CalcResult{
		Subtotal: d("5382000"), Total: d("5005260"),
		TierDiscountPercent: d("7"), DiscountPercent: d("7"), DiscountAmount: d("376740"),
		Measurements: []curtains.MeasurementResult{{
			Length: m.Length, Height: m.Height,
			Parts: []curtains.PartResult{{VariantID: vPortyera, SKU: "VM-GRAFIT", Role: "portyera",
				WidthM: m.Length, HeightM: m.Height, Coef: d("2.5"), PriceUnit: curtains.PerSqm,
				UnitPrice: d("5382000"), LineTotal: d("5382000")}},
		}},
		Warnings: []string{},
	}
	return res, nil
}
