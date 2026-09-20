package tools

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
	"github.com/muhammadvadud/alora-ai/internal/session"
)

const (
	errCatalogNotReady = "katalog yuklanmadi, narx aytib bo'lmaydi"
	errCalcUnavailable = "hisob xizmati vaqtincha ishlamayapti"
	errUnknownRole     = "foydalanuvchi roli noma'lum — chegirma berib bo'lmaydi"
	warnStale          = "narxlar yangilanmagan bo'lishi mumkin (katalog eskirgan)"
)

// displayMoney — "5382000" → "5 382 000 so'm". LLM sonni o'zi formatlamasin.
func displayMoney(d decimal.Decimal) string {
	s := d.Round(0).StringFixed(0)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String() + " so'm"
	}
	return b.String() + " so'm"
}

// displayTotals — Django natijasidan o'qiladigan summalar. Sonlar hisobdan, matn bu yerda.
func displayTotals(r curtains.CalcResult) map[string]string {
	return map[string]string{
		"subtotal": displayMoney(r.Subtotal),
		"discount": displayMoney(r.DiscountAmount) + " (" + r.DiscountPercent.StringFixed(0) + "%)",
		"delivery": displayMoney(r.DeliveryAmount),
		"total":    displayMoney(r.Total),
	}
}

// toAny — struct → JSON → any. Django natijasi LLM'ga aynan kelganicha ketadi (rules §4.2.4).
func toAny(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}

// compactCalc — LLM'ga qaytariladigan IXCHAM hisob (rules §5: natija ≤ 2 KB).
// To'liq CalcResult bu yerda chiqarilmaydi, lekin SessionState'da saqlanadi
// (LastCalc/LastQuote) — hisob hech qaerda yo'qolmaydi. Yig'indilar faqat
// Django qaytargandagina chiqadi (partLabel, Go hisoblamaydi — rules §0.3).
func compactCalc(v *curtains.View, r curtains.CalcResult) (*calcTotals, []calcLine, []calcServiceLine) {
	t := &calcTotals{
		PartsTotal:       decStr(r.PartsTotal),
		AccessoriesTotal: decStr(r.AccessoriesTotal),
		ServicesTotal:    decStr(r.ServicesTotal),
		DeliveryAmount:   r.DeliveryAmount.String(),
		DiscountPercent:  r.DiscountPercent.String(),
		DiscountAmount:   r.DiscountAmount.String(),
		Subtotal:         r.Subtotal.String(),
		Total:            r.Total.String(),
	}
	lines := make([]calcLine, 0)
	for i := range r.Measurements {
		for _, p := range r.Measurements[i].Parts {
			lines = append(lines, calcLine{
				Role:      string(p.Role),
				Name:      partLabel(v, p.VariantID, p.SKU),
				AreaM2:    p.AreaM2.String(),
				Coef:      p.Coef.String(),
				LineTotal: p.LineTotal.String(),
			})
		}
	}
	services := make([]calcServiceLine, 0, len(r.Services))
	for _, s := range r.Services {
		services = append(services, calcServiceLine{Key: s.Key, LineTotal: s.LineTotal.String()})
	}
	return t, lines, services
}

// decStr — kontrakt 1.4.1 oraliq yig'indisi: Django ularni bermasa nil —
// LLM natijasida bo'sh qoladi (Go hisoblamaydi, rules §0.3).
func decStr(d *decimal.Decimal) string {
	if d == nil {
		return ""
	}
	return d.String()
}

// partLabel — bo'lakning o'qiladigan nomi: mahsulot nomi (katalogdan), bo'lmasa
// SKU, u ham bo'lmasa variant ID. LLM variantni shu nom bilan eslab qoladi.
func partLabel(v *curtains.View, variantID, sku string) string {
	if v != nil {
		if x, ok := v.Variant(variantID); ok {
			if p, ok := v.Product(x.ProductID); ok && p.Name != "" {
				return p.Name
			}
		}
	}
	if sku != "" {
		return sku
	}
	return variantID
}

func dec(f float64) decimal.Decimal { return decimal.NewFromFloat(f) }

func decPtr(f float64) *decimal.Decimal {
	if f == 0 {
		return nil
	}
	d := decimal.NewFromFloat(f)
	return &d
}

// readyView — katalog tayyor bo'lmasa xato matni; stale bo'lsa ogohlantirish.
func readyView(d Deps) (*curtains.View, string, bool) {
	if d.Catalog == nil {
		return nil, errCatalogNotReady, false
	}
	v := d.Catalog.View()
	if !v.Ready() {
		return nil, errCatalogNotReady, false
	}
	return v, "", v.Stale
}

// loadState / saveState — sessiya holati (State nil bo'lsa — dev/CLI: holat saqlanmaydi).
func loadState(ctx context.Context, d Deps) (curtains.SessionState, string) {
	sid := session.IDFromContext(ctx)
	if d.State == nil || sid == "" {
		return curtains.NewSessionState(), sid
	}
	st, err := d.State.State(ctx, sid)
	if err != nil {
		return curtains.NewSessionState(), sid
	}
	return st, sid
}

func saveState(ctx context.Context, d Deps, sid string, st curtains.SessionState) {
	if d.State == nil || sid == "" {
		return
	}
	_ = d.State.SetState(ctx, sid, st) // xato chat'ni to'xtatmaydi; keyingi tool qayta yozadi
}

func now(d Deps) time.Time {
	if d.Clock == nil {
		return time.Now()
	}
	return d.Clock.Now()
}

// argsMap — SessionState.Apply uchun DTO'ni map'ga aylantiradi.
func argsMap(v any) map[string]any {
	m, _ := toAny(v).(map[string]any)
	return m
}

// userRole — hisob tool'lariga (curtain_calculate, build_options, order_draft)
// yuboriladigan rol: noma'lum/bo'sh rol → employee (eng qat'iy chegara), chunki
// CalcInput.Role Django'ga boradi va u ham tekshiradi. Chegirma (price_quote)
// esa userRole'ga tayanmaydi — Valid() bilan fail-closed rad qiladi (rules §5.6).
func userRole(ctx context.Context) curtains.UserRole {
	u, ok := curtains.UserFromContext(ctx)
	if !ok {
		return curtains.RoleEmployee
	}
	if role, ok := curtains.ParseRole(string(u.Role)); ok {
		return role
	}
	// Noma'lum rol ("admin", "MANAGER"...) — hisob eng cheklangan chegirma bilan
	// borsin; manager limitiga aylantirilmaydi.
	return curtains.RoleEmployee
}

// warnCorniceMissing — "cornice" xizmati katalogda yo'q: karniz summaga kirmaydi.
const warnCorniceMissing = "karniz narxi katalogda sozlanmagan — karniz summaga kirmadi"
