package curtains

import (
	"sort"

	"github.com/shopspring/decimal"
)

// LookupFason — berilgan xona/uslub bo'yicha mos fason shablonlarini qaytaradi.
// Xona: shablon shu xonani Rooms ro'yxatida ko'rsatgan yoki umumiysi (Rooms bo'sh);
// uslub: shablon uslubi teng yoki shablonda uslub ko'rsatilmagan. Natija deterministik:
// ikkala mezonga ANIQ mos shablon birinchi, keyin nom bo'yicha (rules §1.2).
func LookupFason(v *View, room, style string) []Fason {
	fasons := v.Fasons()
	out := make([]Fason, 0, len(fasons))
	for _, f := range fasons {
		if fasonMatches(f, room, style) {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		ei, ej := fasonExactness(out[i], room, style), fasonExactness(out[j], room, style)
		if ei != ej {
			return ei > ej
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// fasonMatches — filtr: bo'sh berilgan mezon shart emas; Rooms ko'rsatilgan shablon
// uchun xona Roomsda bo'lishi, ko'rsatilmagan (umumiy) uchun — har qanday xona.
func fasonMatches(f Fason, room, style string) bool {
	if room != "" && len(f.Rooms) > 0 && !contains(f.Rooms, room) {
		return false
	}
	if style != "" && f.Style != "" && f.Style != style {
		return false
	}
	return true
}

// fasonExactness — muvofiqlik darajasi (0–2): berilgan mezonlardan nechtasiga
// shablon o'z maydonlari bilan to'g'ridan-to'g'ri mos kelishi. Umumiy shablon (Rooms
// yoki Style bo'sh) aniqroq variantdan pastda chiqadi — fason_lookup tartibi shu.
func fasonExactness(f Fason, room, style string) int {
	n := 0
	if room != "" && len(f.Rooms) > 0 && contains(f.Rooms, room) {
		n++
	}
	if style != "" && f.Style == style {
		n++
	}
	return n
}

// FasonToParts — fason shablonidagi bo'laklarni hisobga tayyor spetsifikatsiyaga
// aylantiradi: kenglik karniz ulushidan, balandlik umumiy balandlik ulushidan (3 kasr).
// VariantID qatlam roli bo'yicha to'ldiriladi va bo'sh qolishi mumkin — AI
// curtain_calculate oldidan product_search natijasidan tanlaydi (rules §0.2).
func FasonToParts(f Fason, corniceM, heightM decimal.Decimal, layerVariants map[PartRole]string) []PartSpec {
	out := make([]PartSpec, 0, len(f.Parts))
	for _, p := range f.Parts {
		out = append(out, PartSpec{
			Role:      p.Role,
			VariantID: layerVariants[p.Role],
			WidthM:    corniceM.Mul(p.WidthShare).Round(3),
			HeightM:   heightM.Mul(p.HeightShare).Round(3),
			Coef:      p.DefaultCoef,
		})
	}
	return out
}

// FasonAccessoryQty — aksessuar miqdorini qty_formula bo'yicha hisoblaydi:
// bo'sh formula → DefaultQty (dona), per_side → har ikki tomondan bittadan,
// per_meter → metrga DefaultQty (3 kasr). Son deterministik — AI o'zi o'ylab topmaydi.
func FasonAccessoryQty(a FasonAccessory, corniceM decimal.Decimal) decimal.Decimal {
	switch a.QtyFormula {
	case "per_side":
		return decimal.NewFromInt(2)
	case "per_meter":
		return corniceM.Mul(a.DefaultQty).Round(3)
	default:
		return a.DefaultQty
	}
}
