package tools

import (
	"context"
	"testing"
	"time"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

func TestProductSearch_ReturnsOnlyVerifiedFactsAndLocalAliases(t *testing.T) {
	catalog := curtains.NewCatalog(curtains.RealClock{}, time.Hour)
	catalog.ApplySnapshot(curtains.Snapshot{Version: 1,
		Products: []curtains.Product{{ID: "fabric", Name: "Sample Velvet", Transparency: "semi", FabricType: "velvet", Care: "Yorliqda: 30°C, oqartiruvchisiz", Tags: map[string]any{"sales_aliases": []any{"baxmal", "бархат"}}, VariantIDs: []string{"fabric-v"}}},
		Variants: []curtains.Variant{{ID: "fabric-v", ProductID: "fabric", PriceUnit: curtains.PerSqm, Price: d("45000"), IsActive: true}},
	})
	handler := &productSearchHandler{d: Deps{Catalog: catalog}}
	for _, query := range []string{"baxmal", "бархат"} {
		result, err := handler.do(context.Background(), productSearchArgs{Query: query})
		if err != nil || result.Error != "" || len(result.Products) != 1 {
			t.Fatalf("query=%s: %v %+v", query, err, result)
		}
		card := result.Products[0]
		if card.Care != "Yorliqda: 30°C, oqartiruvchisiz" || card.Composition != "" || card.Transparency != "semi" {
			t.Fatalf("xususiyatlar katalogdan kelishi kerak: %+v", card)
		}
	}
	result, err := handler.do(context.Background(), productSearchArgs{Transparency: "blackout", Kind: "fabric"})
	if err != nil || result.Count != 0 {
		t.Fatalf("blackout o'rniga semi o'tmasligi kerak: %v %+v", err, result)
	}
}

func TestProductSearch_ListTagAliasesFromCatalog(t *testing.T) {
	catalog := curtains.NewCatalog(curtains.RealClock{}, time.Hour)
	catalog.ApplySnapshot(curtains.Snapshot{Version: 1,
		Products: []curtains.Product{{ID: "fabric", Name: "Sample Velvet", Tags: map[string]any{"baxmal": true, "бархат": true}, VariantIDs: []string{"fabric-v"}}},
		Variants: []curtains.Variant{{ID: "fabric-v", ProductID: "fabric", PriceUnit: curtains.PerSqm, Price: d("45000"), IsActive: true}},
	})
	for _, query := range []string{"baxmal", "бархат"} {
		result, err := (&productSearchHandler{d: Deps{Catalog: catalog}}).do(context.Background(), productSearchArgs{Query: query})
		if err != nil || len(result.Products) != 1 {
			t.Fatalf("ro'yxat aliasi %s: %v %+v", query, err, result)
		}
	}
}
