package tools

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

// fasonLookupLimit natija sonini kontekst byudjetida ushlab turadi (rules §5.5).
const fasonLookupLimit = 5

// FasonLookup — tayyor fason shablonini beradi. Model curtain_calculate'ga parts
// yasashda shu blokdan foydalanadi; share/koef o'zi o'ylab topmaydi (rules §0.2).
func FasonLookup(d Deps) tool.BaseTool {
	h := &fasonLookupHandler{d: d}
	return must(utils.InferTool("fason_lookup",
		"Tayyor fason shablonini beradi: bo'laklar (rol, ulush, koeffitsient) va HAR ROL UCHUN katalogdan tanlangan variant_id. "+
			"O'lcham state'da bo'lsa natijadagi tayyor width_m/height_m va variant_id ni curtain_calculate.parts ga uzat; ulushlarni o'zing ko'paytirma, qatlam-qatlam qidirma. "+
			"Faqat murakkab fason, lambrekan yoki ko'p qatlamli parda so'ralganda yoki premium taklifda chaqir.",
		h.do))
}

type fasonLookupHandler struct{ d Deps }

func (h *fasonLookupHandler) do(ctx context.Context, a fasonLookupArgs) (fasonLookupResult, error) {
	v, errMsg, stale := readyCatalogView(ctx, h.d)
	if errMsg != "" {
		return fasonLookupResult{Error: errMsg}, nil
	}
	state, _ := loadState(ctx, h.d)
	room, style := first(a.Room, state.Room), first(a.Style, state.Style)
	fasons := curtains.LookupFason(v, room, style)
	if len(fasons) > fasonLookupLimit {
		fasons = fasons[:fasonLookupLimit]
	}

	res := fasonLookupResult{Count: len(fasons), Stale: stale}
	if stale {
		res.Warnings = append(res.Warnings, warnStale)
	}
	for _, f := range fasons {
		res.Fasons = append(res.Fasons, fasonCardFrom(v, f, room, style, state))
	}
	return res, nil
}

// fasonCardFrom — domen Fason'ni LLM kartochkasiga aylantiradi; sonlar matn
// (decimal string) — LLM o'zi hisoblamaydi, faqat o'qib uzatadi.
func fasonCardFrom(v *curtains.View, f curtains.Fason, room, style string, state curtains.SessionState) fasonCard {
	c := fasonCard{
		ID: f.ID, Name: f.Name, Style: f.Style,
		Rooms: append([]string(nil), f.Rooms...), PartCount: f.PartCount,
	}
	resolved := curtains.FasonToParts(f, state.CorniceLengthM, state.HeightM, nil)
	for index, p := range f.Parts {
		card := fasonPartCard{
			Role: string(p.Role), Name: p.Name,
			WidthShare: p.WidthShare.String(), HeightShare: p.HeightShare.String(),
			DefaultCoef: p.DefaultCoef.String(),
		}
		if state.CorniceLengthM.IsPositive() && state.HeightM.IsPositive() {
			card.WidthM = resolved[index].WidthM.String()
			card.HeightM = resolved[index].HeightM.String()
		}
		// Har rol uchun eng mos variant — qatlam filtri bilan (LayerOf), deterministik
		// (Search tartibi). Topilmasa bo'sh qoladi — model product_search qiladi.
		query := curtains.SearchQuery{Room: room, Style: style, Layer: string(p.Role), Kind: "fabric", Limit: 1}
		if p.Role == curtains.RolePortyera && state.Transparency != "" {
			query.Transparency, query.StrictTransparency = state.Transparency, true
		}
		if found := curtains.Search(v, query); len(found) > 0 {
			card.VariantID, card.VariantName, card.PriceDisplay = found[0].VariantID, found[0].Name, displayMoney(found[0].Price)
		}
		c.Parts = append(c.Parts, card)
	}
	for _, a := range f.Accessories {
		c.Accessories = append(c.Accessories, fasonAccessoryCard{
			Category: a.Category, DefaultQty: a.DefaultQty.String(), QtyFormula: a.QtyFormula,
		})
	}
	return c
}
