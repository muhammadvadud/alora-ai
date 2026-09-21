package tools

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/shopspring/decimal"

	"github.com/muhammadvadud/alora-ai/internal/agent"
	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

// ProductSearch — mijoz talabiga mos mato/aksessuarni LOKAL katalogdan topadi (rules §4.3).
func ProductSearch(d Deps) tool.BaseTool {
	h := &productSearchHandler{d: d}
	return must(utils.InferTool("product_search",
		"Katalogdan mato/aksessuar topadi. Ko'p qatlam kerak bo'lsa layers=[...] bilan BIR marta chaqir (by_layer). "+
			"Nom aytilsa query. Bo'sh natijada filtrlarni yumshatib 1 marta qayta qidir; aks holda qayta chaqirma. variant_id → curtain_calculate.",
		h.do))
}

type productSearchHandler struct{ d Deps }

func (h *productSearchHandler) do(ctx context.Context, a productSearchArgs) (productSearchResult, error) {
	v, errMsg, stale := readyView(ctx, h.d)
	if errMsg != "" {
		return productSearchResult{Error: errMsg}, nil
	}
	q := curtains.SearchQuery{
		Query: a.Query, Room: a.Room, Style: a.Style, Pattern: a.Pattern, Transparency: a.Transparency,
		Layer: a.Layer, Color: a.Color, Kind: a.Kind, Segment: curtains.Segment(a.Segment),
		Limit: a.Limit,
	}
	if a.MaxPrice > 0 {
		q.MaxPrice = decimal.NewFromFloat(a.MaxPrice)
	}
	var cards []curtains.ProductCard
	res := productSearchResult{Stale: stale}
	if len(a.Layers) > 0 {
		// Ko'p qatlamli so'rov: har qatlam uchun 3 ta eng mos variant — bitta
		// chaqiruvda (aks holda model 5–7 marta qidirib, hisobga yetmaydi).
		res.ByLayer = make(map[string][]productCard, len(a.Layers))
		for _, layer := range a.Layers {
			lq := q
			lq.Layer, lq.Limit = layer, 3
			lc := curtains.Search(v, lq)
			res.ByLayer[layer] = toCards(lc)
			cards = append(cards, lc...)
		}
	} else {
		cards = curtains.Search(v, q)
		res.Products = toCards(cards)
	}
	res.Count = len(cards)
	if stale {
		res.Warnings = append(res.Warnings, warnStale)
	}
	refs := make([]agent.ProductRef, 0, len(cards))
	for _, c := range cards {
		// Kartochka ixcham: faqat LLM tanlash uchun kerakli maydonlar (≤ 2 KB).
		// PriceDisplay — ayrim ssenariylar uchun formatlangan narx aynan natijada
		// bo'lishi shart (hallucination tekshiruvi: "85 000 so'm/m²").
		price, _ := c.Price.Float64()
		refs = append(refs, agent.ProductRef{
			VariantID: c.VariantID, ProductName: c.Name, ColorName: c.ColorName, ColorHex: c.ColorHex,
			Price: price, ImageURL: c.ImageURL,
		})
	}
	// Ilova kartochka chizsin (KindProducts) — LLM'ga emas, transportga.
	agent.AttachProducts(ctx, refs)

	st, sid := loadState(ctx, h.d)
	st.Apply("product_search", argsMap(a), now(h.d))
	saveState(ctx, h.d, sid, st)
	return res, nil
}

// toCards — qidiruv kartochkalarini LLM uchun ixcham ko'rinishga o'tkazadi (≤ 2 KB).
// PriceDisplay aynan natijada bo'lishi shart (hallucination tekshiruvi: "85 000 so'm/m²").
func toCards(cs []curtains.ProductCard) []productCard {
	out := make([]productCard, 0, len(cs))
	for _, c := range cs {
		why := ""
		if len(c.Why) > 0 {
			why = c.Why[0] // bitta qisqa sabab, sabablar ro'yxati emas
		}
		out = append(out, productCard{
			VariantID: c.VariantID, Name: c.Name, ColorName: c.ColorName,
			Price: c.Price.String(), PriceDisplay: displayMoney(c.Price), PriceUnit: string(c.PriceUnit),
			WidthM: c.WidthM.String(), Transparency: c.Transparency,
			Segment: string(c.Segment), Why: why,
		})
	}
	return out
}
