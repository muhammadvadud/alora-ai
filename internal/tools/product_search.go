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
		"Mijoz talabiga mos mato yoki bezak/furnituralarni do'kon katalogidan topadi. "+
			"Natija bo'sh bo'lsa filtrlarni yumshatib 1 marta qayta qidir, aks holda qayta chaqirma; "+
			"natijadagi variant_id ni curtain_calculate ga uzat.",
		h.do))
}

type productSearchHandler struct{ d Deps }

func (h *productSearchHandler) do(ctx context.Context, a productSearchArgs) (productSearchResult, error) {
	v, errMsg, stale := readyView(h.d)
	if errMsg != "" {
		return productSearchResult{Error: errMsg}, nil
	}
	q := curtains.SearchQuery{
		Room: a.Room, Style: a.Style, Pattern: a.Pattern, Transparency: a.Transparency,
		Layer: a.Layer, Color: a.Color, Kind: a.Kind, Segment: curtains.Segment(a.Segment),
		Limit: a.Limit,
	}
	if a.MaxPrice > 0 {
		q.MaxPrice = decimal.NewFromFloat(a.MaxPrice)
	}
	cards := curtains.Search(v, q)

	res := productSearchResult{Count: len(cards), Stale: stale}
	if stale {
		res.Warnings = append(res.Warnings, warnStale)
	}
	refs := make([]agent.ProductRef, 0, len(cards))
	for _, c := range cards {
		res.Products = append(res.Products, productCard{
			ProductID: c.ProductID, VariantID: c.VariantID, Name: c.Name,
			ColorName: c.ColorName, ColorHex: c.ColorHex,
			Price: c.Price.String(), PriceDisplay: displayMoney(c.Price), PriceUnit: string(c.PriceUnit),
			WidthM: c.WidthM.String(), FabricType: c.FabricType, Transparency: c.Transparency,
			Segment: string(c.Segment), ImageURL: c.ImageURL, Why: c.Why,
		})
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
