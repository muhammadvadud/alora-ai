package tools

import (
	"context"
	"errors"
	"fmt"

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
		"Mato va so'ralgan services narxini Django hisoblaydi; [] faqat mato. Xizmatni natijada bor bo'lsagina kiritilgan de. "+
			"Bir oyna: cornice_length_m/height_m/parts; ko'p oyna: windows[]. variant_id faqat katalogdagi to'liq ID. "+
			"Rim/rulon hisoblanmaydi. Jami va explanation'ni ishlat.",
		h.do))
}

type curtainCalcHandler struct{ d Deps }

func (h *curtainCalcHandler) do(ctx context.Context, a curtainCalcArgs) (curtainCalcResult, error) {
	v, errMsg, stale := readyView(ctx, h.d)
	if errMsg != "" {
		return curtainCalcResult{Error: errMsg}, nil
	}
	if h.d.Calc == nil {
		return curtainCalcResult{Error: errCalcUnavailable}, nil
	}

	measurements, windows, errMsg := calcMeasurements(a)
	if errMsg != "" {
		return curtainCalcResult{Error: errMsg}, nil
	}
	// Rulon/rim — tayyor, burmasiz mahsulot: kalkulyator ularni to'g'ri narxlamaydi
	// (ADR-014). Model roli "portyera" qilib aylab o'tsa ham rad etiladi — tekshiruv
	// rolda emas, mahsulotning katalog qatlamida.
	for _, m := range measurements {
		for _, p := range m.Parts {
			if _, ok := curtains.CalcSupportLayer(v, p.VariantID); !ok {
				return curtainCalcResult{Error: curtains.MsgNotCalcSupported}, nil
			}
		}
	}

	in := curtains.CalcInput{Measurements: measurements, Services: normalizeServiceKeys(a.Services), Role: userRole(ctx)}
	if err := curtains.ValidateWithLimits(in, v.Variant, v.Limits()); err != nil {
		return curtainCalcResult{Error: err.Error()}, nil
	}

	res, err := h.d.Calc.Calculate(ctx, in)
	if err != nil {
		return curtainCalcResult{Error: calcErrorText(err)}, nil
	}

	t, lines, services := compactCalc(v, res)
	out := curtainCalcResult{
		Totals:      t,
		Windows:     windowLines(windows, res),
		Lines:       lines,
		Services:    services,
		Discount:    discountBlock(ctx, v, res, in.Role),
		Display:     displayTotals(res),
		Warnings:    append([]string{}, res.Warnings...),
		Explanation: buildExplanation(v, res),
		Stale:       stale,
	}
	if a.SuggestSavings {
		out.Explanation = append(out.Explanation, curtains.Explain(res)...)
	}
	if stale {
		out.Warnings = append(out.Warnings, warnStale)
	}

	st, sid := loadState(ctx, h.d)
	st.Apply("curtain_calculate", argsMap(a), now(h.d))
	st.LastCalc = &res
	st.QuoteNeedsReview = false
	st.LastQuote = nil
	st.SelectedOption = ""
	st.LastOptions = nil
	saveState(ctx, h.d, sid, st)
	return out, nil
}

// calcMeasurements — DTO (float64) → domen o'lchovlari (decimal) + oyna spesi.
//
// windows[] bo'lsa har bir oyna alohida o'lchov bo'ladi; qty bo'yicha o'lchov
// TAKRORLANADI (Django'ga har biri alohida yozuv) — Go summani ko'paytirmaydi,
// chunki formula faqat Django'da (rules §0.2). Eski top-level maydonlar saqlanadi,
// ikkalasi birga kelsa xato: model qaysi o'lchov kerakligini chalkashtirmasin.
func calcMeasurements(a curtainCalcArgs) (measurements []curtains.MeasurementInput, windows []curtains.WindowSpec, errMsg string) {
	if len(a.Windows) == 0 {
		if a.CorniceLengthM <= 0 || a.HeightM <= 0 {
			return nil, nil, "karniz uzunligi va balandlikni yozing (cornice_length_m, height_m) yoki bir nechta oyna uchun windows[] yuboring"
		}
		return []curtains.MeasurementInput{measurementFrom(a.CorniceLengthM, a.HeightM, a.Parts, a.Accessories)},
			[]curtains.WindowSpec{{LengthM: dec(a.CorniceLengthM), HeightM: dec(a.HeightM), Parts: len(a.Parts), Qty: 1}},
			""
	}
	if a.CorniceLengthM > 0 || a.HeightM > 0 || len(a.Parts) > 0 || len(a.Accessories) > 0 {
		return nil, nil, "windows[] bilan birga cornice_length_m/height_m/parts/accessories'ni yuborma — har bir oyna o'z o'lchamini windows ichida oladi"
	}
	for _, w := range a.Windows {
		qty := w.Qty
		if qty < 1 {
			qty = 1
		}
		if qty > curtains.MaxMeasurements {
			return nil, nil, fmt.Sprintf("bir chaqiruvda %d tadan ko'p o'lchov bo'lmaydi", curtains.MaxMeasurements)
		}
		if w.CorniceLengthM <= 0 || w.HeightM <= 0 {
			return nil, nil, "har bir oyna uchun cornice_length_m va height_m majburiy"
		}
		if len(w.Parts) < 1 {
			return nil, nil, "har bir oyna uchun parts majburiy"
		}
		spec := curtains.WindowSpec{
			RoomName: w.RoomName,
			LengthM:  dec(w.CorniceLengthM),
			HeightM:  dec(w.HeightM),
			Qty:      qty,
			Parts:    len(w.Parts),
		}
		for i := 0; i < qty; i++ {
			measurements = append(measurements, measurementFrom(w.CorniceLengthM, w.HeightM, w.Parts, w.Accessories))
		}
		windows = append(windows, spec)
	}
	if len(measurements) > curtains.MaxMeasurements {
		return nil, nil, fmt.Sprintf("bir chaqiruvda %d tadan ko'p o'lchov bo'lmaydi", curtains.MaxMeasurements)
	}
	return measurements, windows, ""
}

// measurementFrom — bitta oyna: o'lcham + bo'laklar + bezaklar. Bo'sh maydonlar nil —
// Django o'lchovdan oladi (rules §4.4.2).
func measurementFrom(lengthM, heightM float64, parts []calcPart, accessories []calcAccessory) curtains.MeasurementInput {
	m := curtains.MeasurementInput{Length: dec(lengthM), Height: dec(heightM)}
	for _, p := range parts {
		m.Parts = append(m.Parts, curtains.PartInput{
			VariantID: p.VariantID, Role: curtains.PartRole(p.Role),
			WidthM: decPtr(p.WidthM), HeightM: decPtr(p.HeightM), Coef: decPtr(p.Coefficient),
		})
	}
	for _, x := range accessories {
		m.Accessories = append(m.Accessories, curtains.AccessoryInput{VariantID: x.VariantID, Qty: dec(x.Qty)})
	}
	return m
}

// calcErrorText — Django/klient xatosi → sotuvchiga tushunarli matn; ichki tafsilot sizib chiqmaydi.
func calcErrorText(err error) string {
	var ve *curtains.ValidationError
	if errors.As(err, &ve) {
		return ve.Msg
	}
	return errCalcUnavailable
}
