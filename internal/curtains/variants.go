package curtains

import (
	"sort"

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
			cards := cheapestFabric(v, q, SegmentEconomy, "")
			if len(cards) == 0 {
				plans[i].Why = "economy segmentida mos mato yo'q"
				continue
			}
			plans[i].Parts = []PartSpec{basicPart(cards[0], RolePortyera, st, coef)}
		case OptionStandard:
			port := cheapestFabric(v, q, SegmentStandard, "")
			tul := cheapestFabric(v, q, SegmentStandard, "transparent")
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
		plans[i].Why = whyFor(plans[i].Tier)
	}
	return plans
}

func whyFor(t OptionTier) string {
	switch t {
	case OptionEconomy:
		return "asosiy portyera"
	case OptionStandard:
		return "portyera va tul"
	default:
		return "astar, lambrekan va montaj"
	}
}

func basicPart(card ProductCard, role PartRole, st SessionState, coef decimal.Decimal) PartSpec {
	return PartSpec{Role: role, VariantID: card.VariantID, WidthM: st.CorniceLengthM, HeightM: st.HeightM, Coef: coef}
}

func cheapestFabric(v *View, q SearchQuery, segment Segment, transparency string) []ProductCard {
	q.Kind, q.Segment, q.Transparency, q.Limit = "fabric", segment, transparency, 10
	cards := Search(v, q)
	// Economy katalogi eski bo'lishi mumkin; faqat shu paketda segmentsiz fallback
	// ishlatiladi, aks holda standard/premium mahsulotlari noto'g'ri paketga o'tadi.
	if len(cards) == 0 && segment == SegmentEconomy {
		q.Segment = ""
		cards = Search(v, q)
	}
	sort.SliceStable(cards, func(i, j int) bool {
		if !cards[i].Price.Equal(cards[j].Price) {
			return cards[i].Price.LessThan(cards[j].Price)
		}
		return cards[i].VariantID < cards[j].VariantID
	})
	return cards
}

func premiumParts(v *View, st SessionState, q SearchQuery, coef decimal.Decimal) ([]PartSpec, []AccessoryInput) {
	fasons := LookupFason(v, st.Room, st.Style)
	port := cheapestFabric(v, q, SegmentPremium, "")
	tul := cheapestFabric(v, q, SegmentPremium, "transparent")
	astar := cheapestFabric(v, q, SegmentPremium, "blackout")
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
