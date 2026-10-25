package curtains

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// Explain — hisob natijasi bo'yicha sotuvchiga qisqa izoh/tejash tavsiyalari.
// SOF matn: hech qanday summa hisoblanmaydi (rules §0.2) — foizlar va
// koeffitsientlar natijadan aynan olinadi; Django `warnings` takrorlanmaydi.
// Koeffitsient tavsiyasi faqat "qayta hisoblang" deydi, narx aytmaydi.
func Explain(res CalcResult) []string {
	const maxLines = 6
	half := decimal.NewFromFloat(0.5)
	two := decimal.NewFromInt(2)
	var out []string
	add := func(s string) {
		if len(out) < maxLines {
			out = append(out, s)
		}
	}
	for _, m := range res.Measurements {
		for _, p := range m.Parts {
			if p.WastePercent.IsPositive() {
				add(fmt.Sprintf("%s (%s): chok/bukma zapasi %s%%; rulon bichish chiqindisi emas", p.Role, p.SKU, p.WastePercent.StringFixed(1)))
			}
		}
	}
	for _, m := range res.Measurements {
		for _, p := range m.Parts {
			if p.Coef.GreaterThanOrEqual(two) {
				add(fmt.Sprintf("%s: burma koeffitsienti %s — %s ga tushirsangiz mato kamayadi (qayta hisoblash: curtain_calculate)", p.Role, p.Coef.String(), p.Coef.Sub(half).String()))
				break // bitta tavsiya yetarli
			}
		}
	}
	if res.DiscountPercent.IsPositive() {
		add(fmt.Sprintf("chegirma %s%% qo'llandi (tier %s%%; qo'lda %s%%)", res.DiscountPercent.String(), res.TierDiscountPercent.String(), res.ManualDiscountPercent.String()))
	}
	return out
}
