package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

// OrderDraft — buyurtma qoralamasini qurish va Django'ga saqlash (rules §5.8).
// AI buyurtmani o'zi tasdiqlamaydi: yakuniy tasdiq sotuvchida (REST tugma bilan).
func OrderDraft(d Deps) tool.BaseTool {
	h := &orderDraftHandler{d: d}
	return must(utils.InferTool("order_draft",
		"Buyurtma qoralamasi: action=quote — joriy taklif; draft — yetishmayotgan mijoz "+
			"ma'lumotlarini ko'rsatadi (sotuvchi kiritadi); confirm — Django'da draft buyurtma "+
			"yaratadi. AI o'zi tasdiqlamaydi — yakuniy tasdiq sotuvchida.",
		h.do))
}

type orderDraftHandler struct{ d Deps }

func (h *orderDraftHandler) do(ctx context.Context, a orderDraftArgs) (orderDraftResult, error) {
	st, sid := loadState(ctx, h.d)
	quote := st.LastQuote
	if quote == nil {
		quote = st.LastCalc // price_quote chaqirilmagan bo'lsa ham oxirgi hisobdan quriladi
	}
	if quote == nil {
		return orderDraftResult{Error: "avval price_quote chaqir — buyurtma uchun taklif yo'q"}, nil
	}
	switch a.Action {
	case "quote":
		return orderDraftResult{Quote: toAny(quote), Display: displayTotals(*quote)}, nil
	case "draft":
		missing := missingOrderFields(a)
		if len(missing) > 0 {
			return orderDraftResult{
				Missing: missing,
				Hint:    "sotuvchi mijoz ma'lumotlarini kiritsin",
				Quote:   toAny(quote),
			}, nil
		}
		return orderDraftResult{
			Ready:   true,
			Summary: orderSummary(a, quote),
			Quote:   toAny(quote),
		}, nil
	case "confirm":
		missing := missingOrderFields(a)
		if len(missing) > 0 {
			return orderDraftResult{
				Missing: missing,
				Hint:    "sotuvchi mijoz ma'lumotlarini kiritsin",
				Quote:   toAny(quote),
			}, nil
		}
		if h.d.Orders == nil {
			return orderDraftResult{Error: "buyurtma xizmati ulanmagan"}, nil
		}
		// qayta so'rov (Django o'sha o'lchamni oladi) + taklifning xizmat/chegirma holati
		in := rebuildInput(quote, userRole(ctx))
		for _, s := range quote.Services {
			if s.IsDelivery {
				in.Delivery = true
			} else {
				in.Services = append(in.Services, s.Key)
			}
		}
		in.DiscountPercent = quote.ManualDiscountPercent
		itemsJSON, err := json.Marshal(in) // CalcInput.MarshalJSON — Django items_json sxemasi
		if err != nil {
			return orderDraftResult{Error: "buyurtma tarkibini shakllantirib bo'lmadi"}, nil
		}
		res, err := h.d.Orders.SaveOrderDraft(ctx, curtains.OrderDraft{
			CustomerName:  a.CustomerName,
			CustomerPhone: a.CustomerPhone,
			Address:       a.Address,
			ItemsJSON:     string(itemsJSON),
			Note:          a.Note,
		})
		if err != nil {
			return orderDraftResult{Error: orderErrorText(err)}, nil
		}
		st.Apply("order_draft", map[string]any{"action": a.Action}, now(h.d))
		saveState(ctx, h.d, sid, st)
		return orderDraftResult{
			OrderID:      res.OrderID,
			OrderNumber:  res.OrderNumber,
			CustomerRef:  customerRef(a.CustomerPhone),
			TotalDisplay: displayMoney(quote.Total),
		}, nil
	default:
		return orderDraftResult{Error: "action=quote|draft|confirm bo'lishi kerak"}, nil
	}
}

// missingOrderFields — confirm uchun shart mijoz maydonlari (rules §5.8: sotuvchi kiritadi).
func missingOrderFields(a orderDraftArgs) []string {
	var missing []string
	if strings.TrimSpace(a.CustomerName) == "" {
		missing = append(missing, "customer_name")
	}
	if strings.TrimSpace(a.CustomerPhone) == "" {
		missing = append(missing, "customer_phone")
	}
	if strings.TrimSpace(a.Address) == "" {
		missing = append(missing, "address")
	}
	return missing
}

// orderSummary — draft tayyor bo'lganda AI ko'radigan xulosa. Ism/telefon EMAS —
// faqat customer_ref va taklif sonlari (rules §1.6 PII modelga kirmaydi).
func orderSummary(a orderDraftArgs, quote *curtains.CalcResult) map[string]any {
	s := map[string]any{
		"customer_ref":   customerRef(a.CustomerPhone),
		"total":          displayMoney(quote.Total),
		"deadline_days":  a.DeadlineDays,
		"prepayment_pct": a.PrepaymentPct,
	}
	return s
}

// customerRef — "mijoz #1234": telefon oxirgi 4 raqami. To'liq telefon/ism hech
// qaerga chiqmaydi (rules §16.3 maskalash yagona joyda bo'lmasa ham bu yerda PII yo'q).
func customerRef(phone string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)
	if len(digits) < 4 {
		return "mijoz #****"
	}
	return "mijoz #" + digits[len(digits)-4:]
}

// orderErrorText — Django/klient xatosi → sotuvchiga tushunarli matn; ichki tafsilot
// sizib chiqmaydi (rules §9.4 confirm xato → {"error"}).
func orderErrorText(err error) string {
	var ve *curtains.ValidationError
	if errors.As(err, &ve) {
		return ve.Msg
	}
	return "buyurtma xizmati vaqtincha ishlamayapti"
}
