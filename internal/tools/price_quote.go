package tools

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// PriceQuote — oxirgi hisob (LastCalc) ustiga xizmatlar, chegirma, avans (rules §5.7, tasks 1.15).
// Narx Django'da; bu yerda faqat rol chegarasi (oldindan aniq xato) va avans/qoldiq.
func PriceQuote(d Deps) tool.BaseTool {
	h := &priceQuoteHandler{d: d}
	return must(utils.InferTool("price_quote",
		"curtain_calculate natijasiga tikuv, montaj, yetkazish va chegirmani qo'shib yakuniy narx, "+
			"avans va qoldiqni beradi. Xizmat tarkibi o'zgarsa services to'liq ro'yxatini yubor; [] xizmatlarni olib tashlaydi. curtain_calculate dan keyin chaqir. "+
			"discount: tier_percent=mijoz miqdori (bosqichli), manual_percent=qo'lda berilgan, applied_percent=jami; "+
			"rule='qo'shiladi' — ikkalasi qo'shiladi, 'kattasi' — kattasi olinadi.",
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
	if msg := selectOption(&st, a.OptionTier); msg != "" {
		return priceQuoteResult{Error: msg}, nil
	}
	if st.LastCalc == nil || len(st.LastCalc.Measurements) == 0 {
		return priceQuoteResult{Error: "avval curtain_calculate chaqir — hisoblanadigan parda yo'q"}, nil
	}

	in, warnings, errMsg := prepareQuoteInput(ctx, v, st, a)
	if errMsg != "" {
		return priceQuoteResult{Error: errMsg}, nil
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
		Totals:   t,
		Lines:    lines,
		Services: services,
		Discount: discountBlock(ctx, v, res, in.Role),
		Display:  disp,
		Warnings: append(warnings, res.Warnings...),
		Stale:    stale,
	}
	// Mobil "Qanday hisoblandi?" — formula qatorlari. price_quote natijasi
	// lines bilan 2 KB ga yaqin (rules §5), shuning uchun faqat kichik hisobda.
	if len(lines) <= 4 {
		out.Explanation = buildExplanation(v, res)
	}
	if stale {
		out.Warnings = append(out.Warnings, warnStale)
	}
	st.LastQuote = &res
	st.QuoteNeedsReview = false
	saveState(ctx, h.d, sid, st)
	return out, nil
}
