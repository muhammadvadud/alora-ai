package curtains

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shopspring/decimal"
)

// OptionTier — mijozga ko'rsatiladigan uchta tayyor paket darajasi.
type OptionTier string

const (
	OptionEconomy  OptionTier = "economy"
	OptionStandard OptionTier = "standard"
	OptionPremium  OptionTier = "premium"
)

// OptionPlan hisoblash xizmatiga yuboriladigan mahsulot va xizmatlar rejasidir.
// Bu yerda narx formulasi yo'q: yakuniy qiymatni Django CalcPreview beradi.
type OptionPlan struct {
	Tier        OptionTier
	Coef        decimal.Decimal
	Parts       []PartSpec
	Accessories []AccessoryInput
	Services    []string
	NeedCornice bool
	NeedInstall bool
	Why         string
}

// PlanOptions uchta paketni katalog snapshotidan deterministik tuzadi.
func PlanOptions(v *View, st SessionState, q SearchQuery) [3]OptionPlan {
	plans := [3]OptionPlan{{Tier: OptionEconomy}, {Tier: OptionStandard}, {Tier: OptionPremium}}
	if v == nil {
		for i := range plans {
			plans[i].Why = "katalog yuklanmadi"
		}
		return plans
	}
	if st.CorniceLengthM.IsZero() || st.HeightM.IsZero() {
		for i := range plans {
			plans[i].Why = "o'lcham yo'q"
		}
		return plans
	}
	for i := range plans {
		key := "coef_" + string(plans[i].Tier)
		coef, ok := v.CalcRule(key)
		if !ok {
			coef, ok = v.CalcRule("coef_default")
		}
		if !ok {
			plans[i].Why = "koeffitsient qoidasi yo'q"
			continue
		}
		plans[i].Coef = coef
		plans[i].Services = []string{"sewing", "tape", "hook"}
		switch plans[i].Tier {
		case OptionEconomy:
			cards := cheapestFabric(v, q, SegmentEconomy)
			if len(cards) == 0 {
				plans[i].Why = "economy segmentida mos mato yo'q"
				continue
			}
			plans[i].Parts = []PartSpec{basicPart(cards[0], RolePortyera, st, coef)}
		case OptionStandard:
			port := cheapestFabric(v, q, SegmentStandard)
			tul := cheapestLayer(v, q, "tul", SegmentStandard, SegmentEconomy, SegmentPremium)
			if len(port) == 0 || len(tul) == 0 {
				plans[i].Why = "standard segmentida mos qatlam topilmadi"
				continue
			}
			plans[i].Parts = []PartSpec{
				basicPart(port[0], RolePortyera, st, coef),
				basicPart(tul[0], RoleTul, st, coef),
			}
		case OptionPremium:
			plans[i].Services = append(plans[i].Services, "install")
			plans[i].NeedCornice, plans[i].NeedInstall = true, true
			plans[i].Parts, plans[i].Accessories = premiumParts(v, st, q, coef)
			if plans[i].Parts == nil {
				plans[i].Why = "premium segmentida mos qatlam topilmadi"
				continue
			}
		}
		plans[i].Why = whyFor(plans[i].Tier) + transparencyNote(v, plans[i].Parts, q.Transparency) + roomNote(v, plans[i].Parts, q.Room)
		// Rulon/rim paketga tushsa (kategoriya nomi/xalqaro SKU mos kelmagan holda) —
		// narxini sotuvchi qo'lda belgilaydi, AI hisoblamaydi (ADR-014).
		if !PartsCalcSupported(v, plans[i].Parts) {
			plans[i].Parts, plans[i].Accessories, plans[i].Why = nil, nil, MsgNotCalcSupported
		}
	}
	return plans
}

// transparencyNote — so'ralgan shaffoflik segmentda topilmay, paket ballga qaytganini
// aytadi. Aks holda model paketni so'ralgan shaffoflikda deb o'ylab, mijozga semi
// matoni "blackout" deb taqdim etdi (2026-09-27 live). Faqat nomuvofiqlikda — token tejaladi.
func transparencyNote(v *View, parts []PartSpec, want string) string {
	if want == "" {
		return ""
	}
	for _, p := range parts {
		if p.Role != RolePortyera {
			continue
		}
		x, ok := v.variants[p.VariantID]
		if !ok {
			return ""
		}
		got := v.products[x.ProductID].Transparency
		if strings.EqualFold(got, want) {
			return ""
		}
		return fmt.Sprintf(" (bu segmentda %s yo'q — mato %s)", want, got)
	}
	return ""
}

// roomNote — transparencyNote uslubida, lekin xona uchun: so'ralgan xonaga tavsiya
// etilmagan mato paketga tushsa, mijozga izohsiz "mos mato" deyish noto'g'ri edi
// (2026-09-28 live: wedding so'rog'ida kids xonasiga matosi keldi). Faqat nomuvofiqlikda.
func roomNote(v *View, parts []PartSpec, want string) string {
	if want == "" {
		return ""
	}
	for _, p := range parts {
		if p.Role != RolePortyera {
			continue
		}
		x, ok := v.variants[p.VariantID]
		if !ok {
			return ""
		}
		rooms := v.products[x.ProductID].RecommendedRooms
		if containsFold(rooms, want) {
			return ""
		}
		return fmt.Sprintf(" (bu segmentda %s uchun mato yo'q — mato %s uchun)", want, displayRooms(rooms))
	}
	return ""
}

// displayRooms xonalar ro'yxatini qisqacha ko'rsatadi (bo'sh bo'lsa "boshqa xonalar").
func displayRooms(rooms []string) string {
	if len(rooms) == 0 {
		return "boshqa xonalar"
	}
	if len(rooms) > 3 {
		rooms = rooms[:3]
	}
	return strings.Join(rooms, ", ")
}

func whyFor(t OptionTier) string {
	switch t {
	case OptionEconomy:
		return "asosiy portyera"
	case OptionStandard:
		return "portyera va tul"
	default:
		return "fason bo'yicha to'liq tarkib; tafsilotlar hisobda"
	}
}

func basicPart(card ProductCard, role PartRole, st SessionState, coef decimal.Decimal) PartSpec {
	return PartSpec{Role: role, VariantID: card.VariantID, WidthM: st.CorniceLengthM, HeightM: st.HeightM, Coef: coef}
}

// cheapestLayer — berilgan qatlam (portyera/tul/astar) uchun segmentdagi eng
// arzon matoni qaytaradi. Qatlam Search'da qat'iy filtr (LayerOf), shaffoflik esa
// faqat ball — shuning uchun ilgari "transparent" so'rab lambrekan qaytardi
// (2026-09-20 live: premium paketda portyera/tul/astar uchalasi "Lambrekan Klassik").
// Segmentda qatlam bo'lmasa `fallback` segmentlari navbat bilan sinaladi —
// premium tul katalogda bo'lmasligi mumkin, lekin tul tul bo'lib qolaveradi.
func cheapestLayer(v *View, q SearchQuery, layer string, segments ...Segment) []ProductCard {
	return cheapestWhere(v, q, layer, strictFilter{}, segments...)
}

// strictFilter — SearchQuery ichidagi qat'iy filtrlar kombinatsiyasi. Ball emas,
// "mos bo'lishi shart" (ADR-012 shaffoflik, ADR-015 xona).
type strictFilter struct {
	room         bool
	transparency bool
}

// cheapestWhere — cheapestLayer, ixtiyoriy qat'iy filtr bilan (bo'sh — filtr yo'q).
// Filtr skor guruhidan OLDIN qo'llanadi: aks holda xona balli yuqori semi mato guruhni
// egallab, blackout variant umuman ko'rilmay qoladi.
func cheapestWhere(v *View, q SearchQuery, layer string, f strictFilter, segments ...Segment) []ProductCard {
	q.Kind, q.Layer, q.Limit = "fabric", layer, 10
	q.StrictRoom, q.StrictTransparency = f.room, f.transparency
	var cards []ProductCard
	for _, seg := range segments {
		q.Segment = seg
		if cards = DropUnsupportedCards(v, bestMatches(Search(v, q))); len(cards) > 0 {
			break
		}
	}
	sort.SliceStable(cards, func(i, j int) bool {
		if !cards[i].Price.Equal(cards[j].Price) {
			return cards[i].Price.LessThan(cards[j].Price)
		}
		return cards[i].VariantID < cards[j].VariantID
	})
	return cards
}

// bestMatches — Search tartibidagi (skor kamayish) natijadan faqat eng yuqori skorli
// guruhni qoldiradi: paket "search → calc" (rules §4.4.1), ya'ni avval talabga moslik
// (xona/uslub/shaffoflik), keyin shu guruh ichida eng arzoni. Ilgari skor tashlanib
// faqat narx bo'yicha saralanardi — blackout so'ralganda semi mato tushdi (2026-09-27 live).
func bestMatches(cards []ProductCard) []ProductCard {
	if len(cards) == 0 {
		return cards
	}
	n := 1
	for n < len(cards) && cards[n].Score == cards[0].Score {
		n++
	}
	return cards[:n]
}

// cheapestFabric — asosiy (portyera) mato. Economy katalogi eski bo'lishi mumkin;
// faqat shu paketda segmentsiz fallback ishlatiladi, aks holda standard/premium
// mahsulotlari noto'g'ri paketga o'tadi.
//
// Talab funksional bo'lgani uchun avval QAT'IY sinanadi, keyin ballga qaytiladi:
//  1. xona + shaffoflik — ikkalasi ham mos bo'lgani (matn talabi);
//  2. shaffoflik — xonadan voz kechildi, funksional talab saqlanadi (ADR-012);
//  3. xona — shaffoflikdan voz kechildi, xona talabi saqlanadi (ADR-015);
//  4. ball — hech biri topilmasa.
//
// Qat'iy sinov faqat paketning O'Z segmentida — segmentsiz fallback bilan economy'ga
// standard blackout tushib, paketlar bir-biriga aralashardi.
func cheapestFabric(v *View, q SearchQuery, segment Segment) []ProductCard {
	for _, f := range fabricFilters(q) {
		if cards := cheapestWhere(v, q, "portyera", f, segment); len(cards) > 0 {
			return cards
		}
	}
	segments := []Segment{segment}
	if segment == SegmentEconomy {
		segments = append(segments, "")
	}
	return cheapestLayer(v, q, "portyera", segments...)
}

// fabricFilters — asosiy mato uchun qat'iy filtr bosqichlari tartibi. Bo'sh maydon
// (shaffoflik/xona so'ralmagan) bosqichni butunlay tushiradi.
func fabricFilters(q SearchQuery) []strictFilter {
	var out []strictFilter
	if q.Transparency != "" && q.Room != "" {
		out = append(out, strictFilter{room: true, transparency: true})
	}
	if q.Transparency != "" {
		out = append(out, strictFilter{transparency: true})
	}
	if q.Room != "" {
		out = append(out, strictFilter{room: true})
	}
	return out
}

func premiumParts(v *View, st SessionState, q SearchQuery, coef decimal.Decimal) ([]PartSpec, []AccessoryInput) {
	fasons := LookupFason(v, q.Room, q.Style)
	port := cheapestFabric(v, q, SegmentPremium)
	// Yordamchi qatlamlar premium segmentda bo'lmasa pastroq segmentdan olinadi.
	tul := cheapestLayer(v, q, "tul", SegmentPremium, SegmentStandard, SegmentEconomy)
	astar := cheapestLayer(v, q, "astar", SegmentPremium, SegmentStandard, SegmentEconomy)
	if len(port) == 0 || len(tul) == 0 || len(astar) == 0 {
		return nil, nil
	}
	var parts []PartSpec
	if len(fasons) > 0 {
		f := fasons[0]
		roles := make(map[PartRole]string)
		for _, p := range f.Parts {
			card := port[0]
			switch p.Role {
			case RoleTul:
				card = tul[0]
			case RoleAstar:
				card = astar[0]
			case RoleLambrekan, RoleKant, RoleDekor:
				matches := cheapestLayer(v, q, string(p.Role), SegmentPremium, SegmentStandard, SegmentEconomy)
				if len(matches) == 0 {
					return nil, nil
				}
				card = matches[0]
			}
			roles[p.Role] = card.VariantID
		}
		parts = FasonToParts(f, st.CorniceLengthM, st.HeightM, roles)
		for i := range parts {
			// Fason defaulti shaklning tavsiyasi; paket koeffitsienti esa
			// Django CalcPreview uchun shu tierning yagona qoidasi bo'ladi.
			parts[i].Coef = coef
		}
	} else {
		parts = []PartSpec{
			basicPart(port[0], RolePortyera, st, coef),
			basicPart(tul[0], RoleTul, st, coef),
			{Role: RoleLambrekan, VariantID: port[0].VariantID, WidthM: st.CorniceLengthM,
				HeightM: st.HeightM.Mul(decimal.RequireFromString("0.15")), Coef: coef},
		}
	}
	// Astar fason tarkibida bo'lmasa ham premium paketning majburiy qatlamidir.
	hasAstar := false
	for _, p := range parts {
		hasAstar = hasAstar || p.Role == RoleAstar
	}
	if !hasAstar {
		parts = append(parts, basicPart(astar[0], RoleAstar, st, coef))
	}
	var accessories []AccessoryInput
	if len(fasons) > 0 {
		for _, a := range fasons[0].Accessories {
			cards := cheapestAccessory(v, a.Category, q)
			if len(cards) == 0 {
				continue
			}
			accessories = append(accessories, AccessoryInput{VariantID: cards[0].VariantID, Qty: FasonAccessoryQty(a, st.CorniceLengthM)})
		}
	}
	return parts, accessories
}

func cheapestAccessory(v *View, category string, q SearchQuery) []ProductCard {
	q.Kind, q.Segment, q.Limit = "accessory", "", 10
	cards := Search(v, q)
	out := cards[:0]
	for _, card := range cards {
		p, ok := v.Product(card.ProductID)
		if !ok || !accessoryCategory(v, p.CategoryID, category) {
			continue
		}
		out = append(out, card)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].Price.Equal(out[j].Price) {
			return out[i].Price.LessThan(out[j].Price)
		}
		return out[i].VariantID < out[j].VariantID
	})
	return out
}

func accessoryCategory(v *View, id, wanted string) bool {
	seen := map[string]bool{}
	for id != "" && !seen[id] {
		seen[id] = true
		cat, ok := v.Category(id)
		if !ok {
			return false
		}
		if cat.Slug == wanted || cat.Slug == "accessories" {
			return true
		}
		id = cat.ParentID
	}
	return wanted == "" && len(seen) > 0
}
