package tools

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/shopspring/decimal"

	"github.com/muhammadvadud/alora-agent/internal/curtains"
)

// PriceQuote — oxirgi hisob (LastCalc) ustiga xizmatlar, chegirma, avans (rules §5.7, tasks 1.15).
// Narx Django'da; bu yerda faqat rol chegarasi (oldindan aniq xato) va avans/qoldiq.
func PriceQuote(d Deps) tool.BaseTool {
	h := &priceQuoteHandler{d: d}
	return must(utils.InferTool("price_quote",
		"curtain_calculate natijasiga tikuv, tasma, ilgich, montaj, yetkazish va chegirmani qo'shib "+
			"yakuniy narx, avans va qoldiqni beradi. Avval curtain_calculate chaqirilgan bo'lishi shart.",
		h.do))
}

type priceQuoteHandler struct{ d Deps }

func (h *priceQuoteHandler) do(ctx context.Context, a priceQuoteArgs) (priceQuoteResult, error) {
	v, errMsg, stale := readyView(h.d)
	if errMsg != "" {
		return priceQuoteResult{Error: errMsg}, nil
	}
	if h.d.Calc == nil {
		return priceQuoteResult{Error: errCalcUnavailable}, nil
	}
	st, sid := loadState(ctx, h.d)
	if st.LastCalc == nil || len(st.LastCalc.Measurements) == 0 {
		return priceQuoteResult{Error: "avval curtain_calculate chaqir — hisoblanadigan parda yo'q"}, nil
	}

	// Fail-closed: rol noma'lum yoki bo'sh bo'lsa chegirma yo'q — employee kabi
	// qabul qilinsa ham baribir, lekin taxmin qilish o'rniga aniq aytiladi.
	// (userRole emas — u noma'lumni employee'ga tushiradi, bu yerda Valid kerak.)
	u, ok := curtains.UserFromContext(ctx)
	if !ok || !u.Role.Valid() {
		return priceQuoteResult{Error: errUnknownRole}, nil
	}
	role := u.Role
	discount := dec(a.DiscountPct)
	if policy, ok := v.DiscountPolicy(); ok && a.DiscountPct > 0 {
		max := policy.ManualMaxPercentManager
		if role == curtains.RoleEmployee {
			max = policy.ManualMaxPercentEmployee
		}
		if discount.GreaterThan(max) {
			// Django jim kesadi — AI esa sotuvchiga chegarani aniq aytishi kerak.
			return priceQuoteResult{Error: fmt.Sprintf("chegirma chegarasi %s%% — %s%% berib bo'lmaydi",
				max.StringFixed(0), discount.StringFixed(0))}, nil
		}
	}

	in := rebuildInput(st.LastCalc, role)
	in.Services = services(a)
	in.Delivery = a.NeedDelivery
	in.DiscountPercent = discount

	var warnings []string
	if a.NeedCornice {
		if _, ok := v.Service("cornice"); ok {
			in.Services = append(in.Services, "cornice")
		} else {
			warnings = append(warnings, "karniz narxi katalogda sozlanmagan — karniz summaga kirmadi")
		}
	}

	res, err := h.d.Calc.Calculate(ctx, in)
	if err != nil {
		return priceQuoteResult{Error: calcErrorText(err)}, nil
	}

	prepayPct := dec(a.PrepaymentPct)
	if a.PrepaymentPct <= 0 {
		if defs, ok := v.OrderDefaults(); ok {
			prepayPct = defs.PrepaymentPercent
		}
	}
	prepayment := res.Total.Mul(prepayPct).Div(decimal.NewFromInt(100)).Round(2)

	out := priceQuoteResult{
		Quote:      toAny(res),
		Display:    displayTotals(res),
		Prepayment: displayMoney(prepayment),
		Balance:    displayMoney(res.Total.Sub(prepayment)),
		Warnings:   append(warnings, res.Warnings...),
		Stale:      stale,
	}
	if stale {
		out.Warnings = append(out.Warnings, warnStale)
	}
	st.LastQuote = &res
	saveState(ctx, h.d, sid, st)
	return out, nil
}

// services — bo'sh bo'lsa standart to'plam; montaj bayroq bilan.
func services(a priceQuoteArgs) []string {
	s := append([]string{}, a.Services...)
	if len(s) == 0 {
		s = []string{"sewing", "tape", "hook"}
	}
	if a.NeedInstall {
		s = append(s, "install")
	}
	return s
}

// rebuildInput — oxirgi hisob natijasidan qayta so'rov (Django o'sha sonlarni oladi).
func rebuildInput(last *curtains.CalcResult, role curtains.UserRole) curtains.CalcInput {
	in := curtains.CalcInput{Role: role, Services: []string{}}
	for _, m := range last.Measurements {
		mi := curtains.MeasurementInput{Length: m.Length, Height: m.Height}
		for _, p := range m.Parts {
			w, hgt, c := p.WidthM, p.HeightM, p.Coef
			mi.Parts = append(mi.Parts, curtains.PartInput{VariantID: p.VariantID, Role: p.Role, WidthM: &w, HeightM: &hgt, Coef: &c})
		}
		for _, x := range m.Accessories {
			mi.Accessories = append(mi.Accessories, curtains.AccessoryInput{VariantID: x.VariantID, Qty: x.Qty})
		}
		in.Measurements = append(in.Measurements, mi)
	}
	return in
}
