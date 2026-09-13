package backend

import (
	"errors"
	"testing"

	"github.com/muhammadvadud/alora-agent/gen/alorav1"
	"github.com/muhammadvadud/alora-agent/internal/curtains"
)

func TestMapping_DoubleAndDecimalRemainExact(t *testing.T) {
	v, err := variantFromPB(&alorav1.CatalogVariant{WidthM: 180000.0, Price: 12500.00, OldPrice: 0})
	if err != nil {
		t.Fatal(err)
	}
	if got := v.WidthM.String(); got != "180000" {
		t.Fatalf("width_m = %q", got)
	}
	if got := v.Price.String(); got != "12500" {
		t.Fatalf("price = %q", got)
	}
	s, err := serviceFromPB(&alorav1.CatalogServicePrice{Price: "12500.00"})
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Price.StringFixed(2); got != "12500.00" {
		t.Fatalf("service price = %q", got)
	}
}

func TestMapping_ProductTagsAndSnapshotPages(t *testing.T) {
	pages := []*alorav1.CatalogSnapshot{
		{Version: 7, Products: []*alorav1.CatalogProduct{{Id: "p1", TagsJson: `{"color":"bej"}`, Variants: []*alorav1.CatalogVariant{{Id: "v1", ProductId: "p1", Price: 1}}}}, ServicePrices: []*alorav1.CatalogServicePrice{{Key: "sewing", Price: "12500.00"}}, Prompt: &alorav1.CatalogPrompt{Version: 1, UpdatedAt: "2026-09-13T10:00:00Z"}, NextPageToken: "2"},
		{Version: 7, Products: []*alorav1.CatalogProduct{{Id: "p2"}}, ServicePrices: []*alorav1.CatalogServicePrice{{Key: "ignored", Price: "1"}}},
	}
	s, err := snapshotFromPages(pages)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Products) != 2 || len(s.Variants) != 1 {
		t.Fatalf("snapshot sizes: products=%d variants=%d", len(s.Products), len(s.Variants))
	}
	if s.Products[0].Tags["color"] != "bej" {
		t.Fatalf("tags = %#v", s.Products[0].Tags)
	}
	if len(s.Services) != 1 || s.Services[0].Key != "sewing" {
		t.Fatalf("services = %#v", s.Services)
	}
}

func TestMapping_DeltaVariantDeleteAndUnknownEntity(t *testing.T) {
	delta, err := deltaFromPB(&alorav1.CatalogDelta{Version: 2, Entity: "variant", EntityId: "v1", Action: "delete", PayloadJson: `{"product_id":"p1"}`})
	if err != nil {
		t.Fatal(err)
	}
	ref, ok := delta.Payload.(curtains.VariantDeleteRef)
	if !ok || ref.ProductID != "p1" {
		t.Fatalf("delete ref = %#v", delta.Payload)
	}
	_, err = deltaFromPB(&alorav1.CatalogDelta{Entity: "future_entity", Action: "create"})
	if !errors.Is(err, curtains.ErrUnknownEntity) {
		t.Fatalf("unknown entity error = %v", err)
	}
}
