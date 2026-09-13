//go:build live

// Live testlar — haqiqiy Django gRPC (run_grpc, :9091) bilan. CI'da ishlamaydi.
// Ishga tushirish: ALORA_LIVE_BACKEND=localhost:9091 AGENT_SERVICE_TOKEN=... go test -tags live ./tests/live/
package live

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/muhammadvadud/alora-agent/internal/backend"
	"github.com/muhammadvadud/alora-agent/internal/curtains"
	"github.com/muhammadvadud/alora-agent/internal/curtains/curtainstest"
)

func dial(t *testing.T) (context.Context, *backend.PricingClient, *curtains.Catalog) {
	t.Helper()
	addr := os.Getenv("ALORA_LIVE_BACKEND")
	if addr == "" {
		t.Skip("ALORA_LIVE_BACKEND yo'q")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	conn, err := backend.Dial(ctx, addr, os.Getenv("AGENT_SERVICE_TOKEN"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	catalog := curtains.NewCatalog(curtains.RealClock{}, 10*time.Minute)
	if err := backend.NewCatalogSyncer(conn, catalog, backend.SyncOptions{PageSize: 100}).Snapshot(ctx); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return ctx, backend.NewPricingClient(conn, 3*time.Second, ""), catalog
}

// TestLive_Snapshot — Django katalogi Go View'ga to'liq keladi (1.44).
func TestLive_Snapshot(t *testing.T) {
	_, _, catalog := dial(t)
	v := catalog.View()
	if !v.Ready() || len(v.Products()) == 0 {
		t.Fatalf("katalog bo'sh: ready=%v products=%d", v.Ready(), len(v.Products()))
	}
	if _, ok := v.Service("sewing"); !ok {
		t.Fatal("service_prices kelmadi (kontrakt 1.1)")
	}
	if _, ok := v.DiscountPolicy(); !ok {
		t.Fatal("discount_policy kelmadi")
	}
	if _, ok := v.Variant("11111111-1111-4111-8111-111111111111"); !ok {
		t.Fatal("etalon variant katalogda yo'q — backend'da seed qiling")
	}
	t.Logf("version=%d products=%d", v.Version, len(v.Products()))
}

// TestLive_SnapshotContract11 — kontrakt 1.1 dagi qo'shimcha katalog bo'limlari
// mapping qatlamida xatosiz kelayotganini tekshiradi; ro'yxatlar bo'sh bo'lishi mumkin.
func TestLive_SnapshotContract11(t *testing.T) {
	_, _, catalog := dial(t)
	v := catalog.View()
	_ = v.DiscountTiers()
	_ = v.Fasons()
	_ = v.Knowledge()
	if _, ok := v.OrderDefaults(); !ok {
		t.Log("Django order_defaults qaytarmadi")
	}
	if _, ok := v.Prompt(); !ok {
		t.Log("Django joriy PromptVersion qaytarmadi")
	}
	if _, ok := v.Service("sewing"); !ok {
		t.Log("Django service_prices ichida sewing yo'q")
	}
}

// TestLive_ReferenceCases — 20 etalon haqiqiy Django CalcPreview orqali (1.10).
func TestLive_ReferenceCases(t *testing.T) {
	ctx, pricing, _ := dial(t)
	fx, _, err := curtainstest.Load(curtainstest.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range fx.Cases {
		t.Run(c.Name, func(t *testing.T) {
			got, err := pricing.Calculate(ctx, c.Input.ToCalcInput(curtains.RoleManager))
			if err != nil {
				t.Fatalf("CalcPreview: %v", err)
			}
			var want struct {
				Total    string   `json:"total"`
				Subtotal string   `json:"subtotal"`
				Warnings []string `json:"warnings"`
			}
			_ = json.Unmarshal(c.Expected, &want)
			if !got.Total.Equal(decimal.RequireFromString(want.Total)) || !got.Subtotal.Equal(decimal.RequireFromString(want.Subtotal)) {
				t.Fatalf("total want %s got %s; subtotal want %s got %s", want.Total, got.Total, want.Subtotal, got.Subtotal)
			}
			if len(got.Warnings) != len(want.Warnings) {
				t.Fatalf("warnings want %v got %v", want.Warnings, got.Warnings)
			}
		})
	}
}
