package tools

import (
	"context"
	"encoding/json"
	"fmt"
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

	// errCatalogEmpty — filial katalogi umuman bo'sh (barcha mahsulotlar import
	// qilinmagan). Bu xizmat uzilishi EMAS: model "hisob xizmati ishlamayapti"
	// demasin, balki sotuvchiga katalogni to'ldirish kerakligini aytsin (ADR-020).
	errCatalogEmpty = "bu filial katalogi bo'sh — menejer katalogni to'ldirishi kerak"

	// hintEmptySearch — 2-marta ketma-ket bo'sh qidiruvdan keyin modelga ko'rsatma
	// (ADR-020): qayta qidirmaslik, mijozga to'g'ri aytish.
	hintEmptySearch = "katalogda topilmadi — qayta qidirma; foydalanuvchiga ayt va mavjud muqobilni taklif qil"
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
				VariantID: p.VariantID,
				Name:      partLabel(v, p.VariantID, p.SKU),
				WidthM:    p.WidthM.String(),
				HeightM:   p.HeightM.String(),
				Coef:      p.Coef.String(),
				AreaM2:    compactDecimal(p.AreaM2),
				PriceUnit: string(p.PriceUnit),
				UnitPrice: compactDecimal(p.UnitPrice),
				LineTotal: p.LineTotal.String(),
			})
		}
	}
	// Ko'p o'chovda qatorlar ko'payadi — hisob chegarasi 2 KB (rules §5.5), shuning
	// uchun bir o'chovda 8, bir nechta o'chovda 4 qator; oyna bo'yicha yig'indi
	// esa windowLines'da qoladi.
	maxLines := 8
	if len(r.Measurements) > 1 {
		maxLines = 4
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	services := make([]calcServiceLine, 0, len(r.Services))
	for _, s := range r.Services {
		name := ""
		if service, ok := v.Service(s.Key); ok {
			name = service.Name
		}
		services = append(services, calcServiceLine{
			Key:       s.Key,
			Name:      name,
			LineTotal: s.LineTotal.String(),
		})
	}
	return t, lines, services
}

func compactDecimal(value decimal.Decimal) string {
	if value.IsZero() {
		return ""
	}
	return value.String()
}

// windowLines — oyna (karniz kesimi) bo'yicha ixcham natija qatorlari: xona, o'lcham,
// nechta deraza va jami. Ko'p o'chovda `lines` qisqaradi (yuqoriga qarang), shuning
// uchun oyna bo'yicha yig'indi shu yerda kerak.
//
// Jami — Django qaytargan qatorlar (line_total) va bezaklar (accessories_total)
// YIG'INDISI: Go hech narsa hisoblamaydi, faqat Django sonlarini jamlaydi va
// ko'rsatadi (rules §0.3). Specs yo'q/short bo'lsa — har o'lchov alohida qator.
func windowLines(specs []curtains.WindowSpec, r curtains.CalcResult) []calcWindowLine {
	if len(r.Measurements) == 0 {
		return nil
	}
	if len(specs) == 0 {
		specs = []curtains.WindowSpec{{}}
	}
	out := make([]calcWindowLine, 0, len(specs))
	idx := 0
	for _, w := range specs {
		qty := w.Qty
		if qty < 1 {
			qty = 1
		}
		if idx+qty > len(r.Measurements) {
			break
		}
		total, parts := decimal.Zero, w.Parts
		for _, m := range r.Measurements[idx : idx+qty] {
			for _, p := range m.Parts {
				total = total.Add(p.LineTotal)
			}
			for _, x := range m.Accessories {
				total = total.Add(x.LineTotal)
			}
			if parts == 0 {
				parts = len(m.Parts)
			}
		}
		out = append(out, calcWindowLine{
			RoomName: w.RoomName,
			LengthM:  decWindow(w.LengthM),
			HeightM:  decWindow(w.HeightM),
			Qty:      qty,
			Parts:    parts,
			Total:    total.String(),
		})
		idx += qty
	}
	// Specs qisqalgan bo'lsa, qolgan o'lchovlar ko'rinmasdan qolmasin.
	for ; idx < len(r.Measurements); idx++ {
		m := r.Measurements[idx]
		total := decimal.Zero
		for _, p := range m.Parts {
			total = total.Add(p.LineTotal)
		}
		for _, x := range m.Accessories {
			total = total.Add(x.LineTotal)
		}
		out = append(out, calcWindowLine{
			LengthM: decWindow(m.Length), HeightM: decWindow(m.Height),
			Parts: len(m.Parts), Total: total.String(),
		})
	}
	return out
}

// decWindow — oyna o'lchamini ixcham ko'rinishda chiqaradi (nol = ko'rsatilmaydi).
func decWindow(d decimal.Decimal) string {
	return compactDecimal(d)
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

// discountBlock — natijaga chegirma TARKIBINI qo'shadi (ADR-020): qaysi foiz
// bosqichli, qaysi qo'lda va qanday qoidalar bo'yicha. Sonlar Django'dan
// (rules §0.2) — Go hech narsa hisoblamaydi, faqat maydonlarni ko'rsatadi.
//
// Chegirma yo'q bo'lsa blok chiqmaydi (token tejaladi). Mijoz rejimida
// manual_max_percent HECH QACHON chiqmaydi (rules §1.6: chegirma chegarasi
// mijozga aytilmaydi).
func discountBlock(ctx context.Context, v *curtains.View, res curtains.CalcResult, role curtains.UserRole) *calcDiscount {
	if !res.DiscountPercent.IsPositive() {
		return nil
	}
	out := &calcDiscount{
		TierPercent:    compactDecimal(res.TierDiscountPercent),
		ManualPercent:  compactDecimal(res.ManualDiscountPercent),
		AppliedPercent: res.DiscountPercent.String(),
	}
	policy, hasPolicy := v.DiscountPolicy()
	if !hasPolicy {
		return out
	}
	// Qoida Django belgilaydi (stack_with_tier) — Go uni faqat o'zbekchaga
	// tarjima qiladi, o'zgartirmaydi. Matn atama qisqa: 2 KB natija limiti
	// (rules §16.1) — ma'nosi tool tavsifida yozilgan.
	if policy.StackWithTier {
		out.Rule = "qo'shiladi"
	} else {
		out.Rule = "kattasi"
	}
	if u, ok := curtains.UserFromContext(ctx); ok && u.CustomerMode {
		return out // chegirma chegarasi mijozga aytilmaydi
	}
	if max := manualMaxFor(policy, role); max.IsPositive() {
		out.ManualMaxPercent = max.StringFixed(0)
	}
	return out
}

// manualMaxFor — joriy rol uchun qo'lda chegirma chegarasi. Noma'lum rol —
// employee (eng qat'iy): limit oshmasligi kerak (rules §5.6).
func manualMaxFor(policy curtains.DiscountPolicy, role curtains.UserRole) decimal.Decimal {
	if role == curtains.RoleManager || role == curtains.RoleAdmin {
		return policy.ManualMaxPercentManager
	}
	return policy.ManualMaxPercentEmployee
}

// readyView — katalog tayyor bo'lmasa xato matni; stale bo'lsa ogohlantirish.
func readyView(ctx context.Context, d Deps) (*curtains.View, string, bool) {
	if d.Catalog == nil {
		return nil, errCatalogNotReady, false
	}
	v := d.Catalog.View()
	if !v.Ready() {
		return nil, errCatalogNotReady, false
	}
	if user, ok := curtains.UserFromContext(ctx); ok && user.TenantID != "" {
		v = v.ForTenant(user.TenantID)
	}
	return v, "", v.Stale
}

// readyCatalogView — readyView + bo'sh katalog tekshiruvi. Katalog yuklangan, lekin
// unda BIRTA ham mahsulot/fason/xizmat yo'q (filial ma'lumot kiritmagan): bu xizmat
// uzilishi EMAS, modelga boshqa xato aytib bermasligi kerak (ADR-020).
func readyCatalogView(ctx context.Context, d Deps) (*curtains.View, string, bool) {
	v, errMsg, stale := readyView(ctx, d)
	if errMsg != "" {
		return nil, errMsg, stale
	}
	if v.IsEmpty() {
		return nil, errCatalogEmpty, stale
	}
	return v, "", stale
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

// buildExplanation — "Qanday hisoblandi?" uchun formula qatorlari (mobil ko'rsatadi).
// Barcha sonlar Django CalcResult'dan (rules §0.3), Go hech narsa hisoblamaydi.
// Bo'laklar 2 tagacha (LLM natijasi ≤ 2 KB — rules §5), yakun qatorlari har doim.
func buildExplanation(v *curtains.View, res curtains.CalcResult) []string {
	const maxParts = 2
	var out []string
	partsTotal, partsShown := 0, 0
	for _, m := range res.Measurements {
		partsTotal += len(m.Parts)
	}
	for _, m := range res.Measurements {
		for _, p := range m.Parts {
			if partsShown >= maxParts {
				continue
			}
			partsShown++
			unit := unitLabel(p.PriceUnit)
			var b strings.Builder
			fmt.Fprintf(&b, "%s %s: ", p.Role, partLabel(v, p.VariantID, p.SKU))
			if p.WidthM.IsPositive() && p.Coef.IsPositive() {
				fmt.Fprintf(&b, "eni %s m × K %s; ", p.WidthM.String(), p.Coef.String())
			}
			if p.HeightM.IsPositive() {
				fmt.Fprintf(&b, "balandlik %s m; ", p.HeightM.String())
			}
			if p.WastePercent.IsPositive() {
				fmt.Fprintf(&b, "xom maydon %s %s → %s %s (chiqindi %s%%); ",
					p.RawAreaM2.String(), unit, p.AreaM2.String(), unit, p.WastePercent.StringFixed(1))
			}
			if p.AreaM2.IsPositive() && p.UnitPrice.IsPositive() {
				fmt.Fprintf(&b, "%s %s × %s = %s", p.AreaM2.String(), unit, displayMoney(p.UnitPrice), displayMoney(p.LineTotal))
			} else {
				fmt.Fprintf(&b, "jami %s", displayMoney(p.LineTotal))
			}
			out = append(out, b.String())
		}
		for _, a := range m.Accessories {
			out = append(out, fmt.Sprintf("Bezak %s: %s %s × %s = %s",
				partLabel(v, a.VariantID, a.SKU), a.Qty.String(), unitLabel(a.PriceUnit),
				displayMoney(a.UnitPrice), displayMoney(a.LineTotal)))
		}
	}
	if partsTotal > partsShown {
		out = append(out, fmt.Sprintf("... va yana %d ta bo'lak", partsTotal-partsShown))
	}
	for _, s := range res.Services {
		if s.IsDelivery {
			continue // quyida DeliveryAmount bilan bitta qator
		}
		name := s.Name
		if name == "" {
			name = s.Key
			if service, ok := v.Service(s.Key); ok {
				name = service.Name
			}
		}
		out = append(out, fmt.Sprintf("%s: %s %s × %s = %s",
			name, s.Qty.String(), unitLabel(s.PriceUnit), displayMoney(s.UnitPrice), displayMoney(s.LineTotal)))
	}
	if res.DeliveryAmount.IsPositive() {
		out = append(out, "Yetkazish: "+displayMoney(res.DeliveryAmount))
	}
	if res.DiscountAmount.IsPositive() {
		out = append(out, fmt.Sprintf("Chegirma %s%%: -%s", res.DiscountPercent.StringFixed(0), displayMoney(res.DiscountAmount)))
	}
	out = append(out, "Jami: "+displayMoney(res.Total))
	return out
}

// unitLabel — narx birligining o'zbekcha qisqa nomi.
func unitLabel(u curtains.PriceUnit) string {
	switch u {
	case curtains.PerPiece:
		return "dona"
	case curtains.PerMeter:
		return "m"
	case curtains.PerSqm:
		return "m²"
	default:
		return string(u)
	}
}
