package tools

import (
	"context"
	"errors"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

// CurtainCalculate — kompozitsion material hisobi. Formula Django'da (ADR-009):
// bu yerda faqat validatsiya, so'rov va ixcham natija. Natijadagi total ni
// aynan ayting. Narxni FAQAT shu tool beradi.
func CurtainCalculate(d Deps) tool.BaseTool {
	h := &curtainCalcHandler{d: d}
	return must(utils.InferTool("curtain_calculate",
		"Parda uchun mato (m2), bezaklar va narxini hisoblaydi — o'lchamlar ma'lum bo'lgach, hisob so'ralganda chaqir. "+
			"Natijadagi total ni aynan ayting. Narxni FAQAT shu tool beradi.",
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

	t, lines, services := compactCalc(v, res)
	out := curtainCalcResult{
		Totals:   t,
		Lines:    lines,
		Services: services,
		Display:  displayTotals(res),
		Warnings: append([]string{}, res.Warnings...),
		Stale:    stale,
	}
	if stale {
		out.Warnings = append(out.Warnings, warnStale)
	}

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
