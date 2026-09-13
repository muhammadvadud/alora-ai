package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/shopspring/decimal"

	"github.com/muhammadvadud/alora-agent/internal/curtains"
)

// CurtainCalculate — kompozitsion material hisobi. Formula Django'da (ADR-009):
// bu yerda faqat validatsiya, so'rov va tavsiya matni.
func CurtainCalculate(d Deps) tool.BaseTool {
	h := &curtainCalcHandler{d: d}
	return must(utils.InferTool("curtain_calculate",
		"Parda uchun mato (m2), bezaklar va ularning narxini hisoblaydi — bir pardada 1 tadan 10 tagacha bo'lak. "+
			"Karniz uzunligi, balandlik va har bo'lak uchun variant_id kerak. Narxni FAQAT shu tool beradi; keyin price_quote chaqir.",
		h.do))
}

type curtainCalcHandler struct{ d Deps }

func (h *curtainCalcHandler) do(ctx context.Context, a curtainCalcArgs) (curtainCalcResult, error) {
	v, errMsg, stale := readyView(h.d)
	if errMsg != "" {
		return curtainCalcResult{Error: errMsg}, nil
	}
	if h.d.Calc == nil {
		return curtainCalcResult{Error: errCalcUnavailable}, nil
	}

	in := calcInputFrom(a, userRole(ctx))
	if err := curtains.Validate(in, v.Variant); err != nil {
		return curtainCalcResult{Error: err.Error()}, nil
	}

	res, err := h.d.Calc.Calculate(ctx, in)
	if err != nil {
		return curtainCalcResult{Error: calcErrorText(err)}, nil
	}

	out := curtainCalcResult{
		RoomName: a.RoomName,
		Calc:     toAny(res),
		Display:  displayTotals(res),
		Warnings: append([]string{}, res.Warnings...),
		Stale:    stale,
	}
	if stale {
		out.Warnings = append(out.Warnings, warnStale)
	}
	out.Explanation = explain(v, res, in)

	st, sid := loadState(ctx, h.d)
	st.Apply("curtain_calculate", argsMap(a), now(h.d))
	st.LastCalc = &res
	saveState(ctx, h.d, sid, st)
	return out, nil
}

// calcInputFrom — DTO (float64) → domen (decimal). Bo'sh maydonlar nil — Django o'lchovdan oladi.
func calcInputFrom(a curtainCalcArgs, role curtains.UserRole) curtains.CalcInput {
	m := curtains.MeasurementInput{Length: dec(a.CorniceLengthM), Height: dec(a.HeightM)}
	for _, p := range a.Parts {
		m.Parts = append(m.Parts, curtains.PartInput{
			VariantID: p.VariantID, Role: curtains.PartRole(p.Role),
			WidthM: decPtr(p.WidthM), HeightM: decPtr(p.HeightM), Coef: decPtr(p.Coefficient),
		})
	}
	for _, x := range a.Accessories {
		m.Accessories = append(m.Accessories, curtains.AccessoryInput{VariantID: x.VariantID, Qty: dec(x.Qty)})
	}
	return curtains.CalcInput{Measurements: []curtains.MeasurementInput{m}, Services: []string{}, Role: role}
}

// calcErrorText — Django/klient xatosi → sotuvchiga tushunarli matn; ichki tafsilot sizib chiqmaydi.
func calcErrorText(err error) string {
	var ve *curtains.ValidationError
	if errors.As(err, &ve) {
		return ve.Msg
	}
	return errCalcUnavailable
}

// explain — hisob paytida ham tavsiya (TZ §3.2). Sonlar Django natijasidan, matn shu yerda.
// Chiqindi ogohlantirishi Django `warnings` da; bu yerda mato eni/balandlik izohi va
// arzonroq analog (katalogdan, ≤ 80% narx/m²) — hisob emas, taqqoslash (rules §4.2.5).
func explain(v *curtains.View, res curtains.CalcResult, in curtains.CalcInput) string {
	if len(res.Measurements) == 0 {
		return ""
	}
	var parts []string
	m := res.Measurements[0]
	for _, p := range m.Parts {
		variant, ok := v.Variant(p.VariantID)
		if !ok {
			continue
		}
		if variant.WidthM.GreaterThan(decimal.Zero) && variant.WidthM.GreaterThanOrEqual(p.HeightM) {
			parts = append(parts, fmt.Sprintf("%s: mato eni %s m, balandlik %s m — chok kerak emas",
				p.SKU, variant.WidthM.String(), p.HeightM.String()))
		}
		if alt, ok := cheaperAnalog(v, variant); ok {
			parts = append(parts, fmt.Sprintf("%s o'rniga %s (%s/m², %s%% arzon) ham mos kelishi mumkin",
				p.SKU, alt.SKU, displayMoney(alt.Price), savingPct(variant.Price, alt.Price)))
		}
	}
	return strings.Join(parts, ". ")
}

// cheaperAnalog — o'sha mahsulotning arzonroq varianti yoki bir kategoriyadagi ≤ 80% narxli mato.
func cheaperAnalog(v *curtains.View, cur curtains.Variant) (curtains.Variant, bool) {
	limit := cur.Price.Mul(decimal.NewFromFloat(0.8))
	best, found := curtains.Variant{}, false
	consider := func(c curtains.Variant) {
		if c.ID == cur.ID || !c.IsActive || c.PriceUnit != cur.PriceUnit || c.Price.GreaterThan(limit) {
			return
		}
		if !found || c.Price.LessThan(best.Price) {
			best, found = c, true
		}
	}
	for _, c := range v.VariantsOf(cur.ProductID) {
		consider(c)
	}
	if found {
		return best, true
	}
	p, ok := v.Product(cur.ProductID)
	if !ok {
		return curtains.Variant{}, false
	}
	for _, other := range v.Products() {
		if other.CategoryID != p.CategoryID || other.ID == p.ID {
			continue
		}
		for _, c := range v.VariantsOf(other.ID) {
			consider(c)
		}
	}
	return best, found
}

func savingPct(from, to decimal.Decimal) string {
	if from.IsZero() {
		return "0"
	}
	return from.Sub(to).Div(from).Mul(decimal.NewFromInt(100)).Round(0).StringFixed(0)
}
