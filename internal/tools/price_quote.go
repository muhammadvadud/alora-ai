package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

// PriceQuote — oxirgi hisob (LastCalc) ustiga xizmatlar, chegirma, avans (rules §5.7, tasks 1.15).
// Narx Django'da; bu yerda faqat rol chegarasi (oldindan aniq xato) va avans/qoldiq.
func PriceQuote(d Deps) tool.BaseTool {
	h := &priceQuoteHandler{d: d}
	return must(utils.InferTool("price_quote",
		"curtain_calculate natijasiga tikuv, montaj, yetkazish va chegirmani qo'shib yakuniy narx, "+
			"avans va qoldiqni beradi. Faqat chegirma yoki yakuniy taklif so'ralganda, curtain_calculate dan keyin chaqir.",
		h.do))
}

type priceQuoteHandler struct{ d Deps }

func (h *priceQuoteHandler) do(ctx context.Context, a priceQuoteArgs) (priceQuoteResult, error) {
	v, errMsg, stale := readyView(ctx, h.d)
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
	in.Delivery = a.NeedDelivery
	in.DiscountPercent = discount

	// Xizmatlar faqat katalogdan (ServicePrice): model bergan yoki standart kalit
	// katalogda bo'lmasa u hisobga yuborilmaydi — ogohlantirish bilan. Aks holda
	// Django "'x' xizmati topilmadi" deb butun hisobni rad etadi.
	requested := services(a)
	if a.NeedCornice {
		requested = append(requested, "cornice")
	}
	var warnings []string
	in.Services = []string{}
	var missing []string
	for _, key := range requested {
		if _, ok := v.Service(key); ok {
			in.Services = append(in.Services, key)
		} else {
			missing = append(missing, key)
		}
	}
	warnings = append(warnings, missingServiceWarnings(missing)...)

	// Avans foizi payload bilan Django'ga ketadi — summani Django hisoblaydi (rules §0.2).
	if a.PrepaymentPct > 0 {
		in.PrepaymentPercent = dec(a.PrepaymentPct)
	}
	res, err := h.d.Calc.Calculate(ctx, in)
	if err != nil {
		return priceQuoteResult{Error: calcErrorText(err)}, nil
	}

	t, lines, services := compactCalc(v, res)
	if res.PrepaymentAmount != nil {
		t.Prepayment = res.PrepaymentAmount.String()
	}
	if res.BalanceAmount != nil {
		t.Balance = res.BalanceAmount.String()
	}
	disp := displayTotals(res)
	// Avans/qoldiq ham FORMATLANGAN holda chiqadi — LLM "2 535 000 so'm" matnini
	// o'zi tuzmaydi, display'dan aynan oladi (rules §0.2, hallucination tekshiruvi).
	if res.PrepaymentAmount != nil && res.BalanceAmount != nil {
		disp["prepayment"] = displayMoney(*res.PrepaymentAmount)
		disp["balance"] = displayMoney(*res.BalanceAmount)
	}

	out := priceQuoteResult{
		Totals:      t,
		Lines:       lines,
		Services:    services,
		Display:     disp,
		Warnings:    append(warnings, res.Warnings...),
		Explanation: curtains.Explain(res),
		Stale:       stale,
	}
	// Mobil "Qanday hisoblandi?" — formula qatorlari. price_quote natijasi
	// lines bilan 2 KB ga yaqin (rules §5), shuning uchun faqat kichik hisobda.
	if len(lines) <= 4 {
		out.Explanation = append(buildExplanation(v, res), out.Explanation...)
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
	s := normalizeServiceKeys(a.Services)
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

// normalizeServiceKeys — LLM ba'zan bir nechta kalitni bitta satrga yozadi
// ("sewing tape hook" yoki "sewing, tape"): bo'sh joy/vergul bo'yicha bo'linadi,
// trim+lowercase, dublikat olib tashlanadi. Aks holda Django "'sewing tape hook'
// xizmati topilmadi" deb butun hisobni rad etadi.
func normalizeServiceKeys(raw []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range raw {
		for _, key := range strings.FieldsFunc(item, func(r rune) bool {
			return r == ' ' || r == ',' || r == ';' || r == '\t' || r == '\n'
		}) {
			key = strings.ToLower(strings.TrimSpace(key))
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}
