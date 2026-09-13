// Package golden — etalon dialog ssenariylari (tests/golden/scenarios/*.yaml)
// uchun fikstura va sohta Calculator. Mock LLM (mockllm.go) skript bo'yicha
// tool chaqiradi, harness (golden_test.go) esa tool ketma-ketligi, stage,
// products va hallucination'ni tekshiradi (rules §14.4, tasks 1.41/1.42).
package golden

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/muhammadvadud/alora-agent/internal/curtains"
)

// Variant ID'lar — YAML ssenariylarda variant_id sifatida ishlatiladi. ID'lar
// ustunlik bilan belgilangan: ssenariy matni qisqa va o'qilishi oson bo'lsin.
const (
	VHiTechGrafit = "v-hitech-grafit"
	VHiTechBej    = "v-hitech-bej"
	VClassicIvory = "v-classic-ivory"
	VVelvetBordo  = "v-velvet-bordo"
	VAccGul       = "v-acc-gul"
	VAccPodxvat   = "v-acc-podxvat"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// testSnapshot — barcha ssenariylar uchun bitta barqaror katalog. Narxlar
// Rules bo'yicha Django'dan keladi; bu yerda ular test fiksturasi, hol.
func testSnapshot() curtains.Snapshot {
	return curtains.Snapshot{
		Version: 1,
		Categories: []curtains.Category{
			{ID: "c-fabrics", Name: "Matolar", Slug: "fabrics"},
			{ID: "c-accessories", Name: "Bezaklar", Slug: "accessories"},
		},
		Products: []curtains.Product{
			{ID: "p-velvet-milano", CategoryID: "c-fabrics", Name: "Velvet Milano", Style: "hi_tech",
				Pattern: "plain", RecommendedRooms: []string{"living"}, Segment: curtains.SegmentStandard,
				DefaultWidthM: d("2.8"), VariantIDs: []string{VHiTechGrafit, VHiTechBej}},
			{ID: "p-classic-ivoire", CategoryID: "c-fabrics", Name: "Classic Ivoire", Style: "provence",
				Pattern: "jacquard", RecommendedRooms: []string{"living", "bedroom"}, Segment: curtains.SegmentStandard,
				DefaultWidthM: d("2.8"), VariantIDs: []string{VClassicIvory}},
			{ID: "p-velvet-bordo", CategoryID: "c-fabrics", Name: "Velvet Bordo", Style: "classic",
				Pattern: "plain", RecommendedRooms: []string{"wedding"}, Segment: curtains.SegmentPremium,
				DefaultWidthM: d("2.8"), VariantIDs: []string{VVelvetBordo}},
			{ID: "p-dekorativ-gul", CategoryID: "c-accessories", Name: "Dekorativ gul", Segment: curtains.SegmentStandard,
				VariantIDs: []string{VAccGul}},
			{ID: "p-podxvat-shnur", CategoryID: "c-accessories", Name: "Podxvat shnur", Segment: curtains.SegmentStandard,
				VariantIDs: []string{VAccPodxvat}},
		},
		Variants: []curtains.Variant{
			{ID: VHiTechGrafit, ProductID: "p-velvet-milano", SKU: "VM-GRAFIT", ColorName: "grafit",
				ColorHex: "#2E2E2E", WidthM: d("2.8"), Price: d("180000"), PriceUnit: curtains.PerSqm, IsActive: true},
			{ID: VHiTechBej, ProductID: "p-velvet-milano", SKU: "VM-BEJ", ColorName: "bej",
				ColorHex: "#E8DCC8", WidthM: d("2.8"), Price: d("120000"), PriceUnit: curtains.PerSqm, IsActive: true},
			{ID: VClassicIvory, ProductID: "p-classic-ivoire", SKU: "CI-IVORY", ColorName: "kremshi",
				ColorHex: "#EFE7DA", WidthM: d("2.8"), Price: d("90000"), PriceUnit: curtains.PerSqm, IsActive: true},
			{ID: VVelvetBordo, ProductID: "p-velvet-bordo", SKU: "VB-BORDO", ColorName: "bordo",
				ColorHex: "#6E1E2E", WidthM: d("2.8"), Price: d("150000"), PriceUnit: curtains.PerSqm, IsActive: true},
			{ID: VAccGul, ProductID: "p-dekorativ-gul", SKU: "GUL-OLTIN", ColorName: "oltin",
				Price: d("85000"), PriceUnit: curtains.PerPiece, IsActive: true},
			{ID: VAccPodxvat, ProductID: "p-podxvat-shnur", SKU: "PSH-SHNUR", ColorName: "bej",
				Price: d("45000"), PriceUnit: curtains.PerMeter, IsActive: true},
		},
		CalcRules: []curtains.CalcRule{{Key: "coef_default", Value: d("2.5")}},
		Services: []curtains.Service{
			{Key: "sewing", Name: "Tikuv", Price: d("12500"), Unit: curtains.PerSqm, IsActive: true},
			{Key: "tape", Name: "Tasma", Price: d("8000"), Unit: curtains.PerMeter, IsActive: true},
			{Key: "hook", Name: "Ilgich", Price: d("3000"), Unit: curtains.PerPiece, IsActive: true},
			{Key: "install", Name: "Montaj", Price: d("50000"), Unit: curtains.PerPiece, IsActive: true},
			{Key: "cornice", Name: "Karniz", Price: d("120000"), Unit: curtains.PerMeter, IsActive: true},
		},
		Policy:   &curtains.DiscountPolicy{ManualMaxPercentEmployee: d("5"), ManualMaxPercentManager: d("15")},
		Defaults: &curtains.OrderDefaults{PrepaymentPercent: d("50"), DeliveryAmount: d("30000"), DeadlineDays: 14},
	}
}

// fakeCalc — Django CalcPreview (ADR-009) o'rnidagi deterministik hisob.
// Haqiqiy formula Go'da YO'Q; ssenariy faqat agent oqimi va tool ketma-ketligini
// tekshiradi, shuning uchun arifmetika soda va kutilgan bo'lishi kerak:
//   - mato: kenglik × balandlik × koeffitsient = area; area × narx = line
//   - tikuv: umumiy area m² bo'yicha; tasma: karniz metriga; ilgich: bo'lakka
//   - yetkazish: OrderDefaults.DeliveryAmount (katalogda)
type fakeCalc struct {
	view *curtains.View
}

func (f *fakeCalc) Calculate(_ context.Context, in curtains.CalcInput) (curtains.CalcResult, error) {
	var (
		subtotal decimal.Decimal
		sewArea  decimal.Decimal
		tapeQty  decimal.Decimal
		partCnt  int64
	)
	measurements := make([]curtains.MeasurementResult, 0, len(in.Measurements))
	for i := range in.Measurements {
		m := in.Measurements[i]
		mr := curtains.MeasurementResult{Length: m.Length, Height: m.Height}
		for j := range m.Parts {
			p := m.Parts[j]
			v, ok := f.view.Variant(p.VariantID)
			if !ok {
				continue
			}
			w, h, coef := m.Length, m.Height, f.coefDefault()
			if p.WidthM != nil {
				w = *p.WidthM
			}
			if p.HeightM != nil {
				h = *p.HeightM
			}
			if p.Coef != nil {
				coef = *p.Coef
			}
			area := w.Mul(h).Mul(coef)
			line := area.Mul(v.Price)
			subtotal = subtotal.Add(line)
			sewArea = sewArea.Add(area)
			partCnt++
			mr.Parts = append(mr.Parts, curtains.PartResult{
				VariantID: p.VariantID, SKU: v.SKU, Role: p.Role,
				WidthM: w, HeightM: h, Coef: coef,
				VisibleAreaM2: area, RawAreaM2: area, AreaM2: area,
				PriceUnit: v.PriceUnit, UnitPrice: v.Price, LineTotal: line,
			})
		}
		for j := range m.Accessories {
			a := m.Accessories[j]
			v, ok := f.view.Variant(a.VariantID)
			if !ok {
				continue
			}
			line := a.Qty.Mul(v.Price)
			subtotal = subtotal.Add(line)
			mr.Accessories = append(mr.Accessories, curtains.AccessoryResult{
				VariantID: a.VariantID, SKU: v.SKU, Qty: a.Qty,
				PriceUnit: v.PriceUnit, UnitPrice: v.Price, LineTotal: line,
			})
			mr.AccessoriesTotal = mr.AccessoriesTotal.Add(line)
		}
		measurements = append(measurements, mr)
		tapeQty = tapeQty.Add(m.Length)
	}

	res := curtains.CalcResult{Measurements: measurements, Warnings: []string{}}
	for _, key := range in.Services {
		sv, ok := f.view.Service(key)
		if !ok {
			continue
		}
		qty := f.serviceQty(key, sewArea, tapeQty, partCnt)
		res.Services = append(res.Services, curtains.ServiceLine{
			Key: key, Name: sv.Name, Qty: qty, PriceUnit: sv.Unit,
			UnitPrice: sv.Price, LineTotal: qty.Mul(sv.Price),
		})
	}

	res.Subtotal = subtotal
	res.ManualDiscountPercent = in.DiscountPercent
	res.DiscountPercent = in.DiscountPercent
	res.DiscountAmount = subtotal.Mul(in.DiscountPercent).Div(decimal.NewFromInt(100)).Round(2)

	if in.Delivery {
		if defs, ok := f.view.OrderDefaults(); ok {
			res.DeliveryAmount = defs.DeliveryAmount
			res.Services = append(res.Services, curtains.ServiceLine{
				Key: "delivery", Name: "Yetkazib berish", Qty: decimal.NewFromInt(1),
				PriceUnit: curtains.PerPiece, UnitPrice: res.DeliveryAmount,
				LineTotal: res.DeliveryAmount, IsDelivery: true,
			})
		}
	}

	// Total = subtotal − chegirma + xizmatlar; yetkazish DELIVERY xizmat qatori
	// orqali qo'shiladi, DeliveryAmount alohida qo'shilmaydi (er-xotin sanash xato!).
	res.Total = subtotal.Sub(res.DiscountAmount)
	for i := range res.Services {
		res.Total = res.Total.Add(res.Services[i].LineTotal)
	}
	return res, nil
}

func (f *fakeCalc) coefDefault() decimal.Decimal {
	if c, ok := f.view.CalcRule("coef_default"); ok {
		return c
	}
	return decimal.RequireFromString("2.5")
}

func (f *fakeCalc) serviceQty(key string, sewArea, tapeQty decimal.Decimal, partCnt int64) decimal.Decimal {
	switch key {
	case "sewing":
		return sewArea
	case "tape":
		return tapeQty
	case "hook":
		return decimal.NewFromInt(partCnt)
	default:
		return decimal.NewFromInt(1)
	}
}
