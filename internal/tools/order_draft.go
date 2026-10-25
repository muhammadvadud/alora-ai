package tools

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

// OrderDraft — joriy taklifni ko'rsatadi va buyurtma qoralamasini tayyorlaydi.
// AI buyurtmani tasdiqlamaydi va PII yig'maydi: mijoz ismi/telefoni/manzili
// ilova formasida sotuvchi tomonidan kiritiladi (ADR-020, rules §1.6).
func OrderDraft(d Deps) tool.BaseTool {
	h := &orderDraftHandler{d: d}
	return must(utils.InferTool("order_draft",
		"Paket tanlansa action=draft va option_tier bilan DARHOL chaqir: saqlangan paketni aynan oladi, curtain_calculate/price_quote bilan qayta qurma. "+
			"action=quote — joriy taklifni ko'rsatadi; action=draft — taklifni buyurtma qoralamasiga shakllantiradi "+
			"va customer_form_required=true qaytaradi: mijoz ismi, telefoni va manzilini ILOVA FORMASI orqali sotuvchi kiritadi. "+
			"Qoralamadan oldin required_services orqali mijoz so'ragan xizmatlarni tekshir. Mijoz ma'lumotlarini hech qachon so'rama va tool'ga yuborma. Yakuniy tasdiq sotuvchida.",
		h.do))
}

type orderDraftHandler struct{ d Deps }

func (h *orderDraftHandler) do(ctx context.Context, a orderDraftArgs) (orderDraftResult, error) {
	st, sid := loadState(ctx, h.d)
	if st.QuoteNeedsReview {
		return orderDraftResult{Error: "talab o'zgargan: eski taklifdan qoralama tuzib bo'lmaydi; avval yangilangan talab bilan qayta hisobla"}, nil
	}
	if a.OptionTier != "" {
		if msg := selectOption(&st, a.OptionTier); msg != "" {
			return orderDraftResult{Error: msg}, nil
		}
		// Tanlov saqlanadi: keyingi price_quote/order_draft ham shu paket ustida ishlaydi.
		saveState(ctx, h.d, sid, st)
	}
	quote := st.LastQuote
	if quote == nil {
		quote = st.LastCalc // price_quote chaqirilmagan bo'lsa ham oxirgi hisobdan quriladi
	}
	if quote == nil {
		return orderDraftResult{Error: "avval price_quote chaqir — buyurtma uchun taklif yo'q"}, nil
	}
	if missing := missingQuotedServices(quote, normalizeServiceKeys(a.RequiredServices)); len(missing) > 0 {
		return orderDraftResult{Error: "so'ralgan xizmatlar hisobga kiritilmagan: " + strings.Join(missing, ", ") + "; avval price_quote orqali to'liq xizmatlar ro'yxati bilan qayta hisobla"}, nil
	}
	switch a.Action {
	case "quote":
		return orderDraftResult{
			Quote:   toAny(quote),
			Display: displayTotals(*quote),
		}, nil
	case "draft":
		st.Apply("order_draft", argsMap(a), now(h.d))
		saveState(ctx, h.d, sid, st)
		// Mijoz ma'lumotlari AI orqali YIG'ILMAYDI — forma shtrixi shu yerda
		// chiqadi. Bu yerda Django'ga yozilmaydi: buyurtma sotuvchi ilovada
		// formani to'ldirib tasdiqlaganda yaratiladi (ADR-020).
		return orderDraftResult{
			OptionTier:           string(st.SelectedOption),
			Quote:                toAny(quote),
			Display:              displayTotals(*quote),
			TotalDisplay:         displayMoney(quote.Total),
			CustomerFormRequired: true,
			Summary:              orderSummary(a, quote),
			Hint:                 "mijoz ismi, telefoni va manzilini ilova formasida kiritish uchun xabar ber — so'rab olma",
		}, nil
	default:
		return orderDraftResult{Error: "action=quote yoki draft bo'lishi kerak (confirm yo'q — tasdiq sotuvchida)"}, nil
	}
}

// orderSummary — draft tayyor bo'lganda AI ko'radigan xulosa. Ism/telefon/manzil
// YO'Q (ADR-020) — faqat taklif sonlari va muddat.
func orderSummary(a orderDraftArgs, quote *curtains.CalcResult) map[string]any {
	return map[string]any{
		"total":          displayMoney(quote.Total),
		"deadline_days":  a.DeadlineDays,
		"prepayment_pct": a.PrepaymentPct,
	}
}

// missingQuotedServices — summa emas, haqiqiy hisob qatorlari tekshiriladi.
func missingQuotedServices(quote *curtains.CalcResult, required []string) []string {
	present := make(map[string]bool, len(quote.Services))
	for _, service := range quote.Services {
		present[service.Key] = true
	}
	var missing []string
	for _, key := range required {
		if !present[key] {
			missing = append(missing, key)
		}
	}
	return missing
}
