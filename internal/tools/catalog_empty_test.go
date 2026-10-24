package tools

import (
	"strings"
	"testing"
	"time"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

// ---- B1: ketma-ket bo'sh qidiruv sikli ----

// B1 (ADR-020): 1-bo'sh natija — filtr yumshatishga ishora (stop yo'q).
// 2- ketma-ket bo'sh — stop:true, model qayta qidirmaydi.
func TestProductSearch_EmptySearchStopsAfterTwo(t *testing.T) {
	dp, _, st := deps(t)
	ctx, _ := ctxFor(curtains.RoleEmployee)
	h := &productSearchHandler{d: dp}

	// hech qanday mahsulot topilmaydigan so'rov
	q := productSearchArgs{Query: "yo'q-bunday-mato-12345", Kind: "fabric"}

	res, _ := h.do(ctx, q)
	if res.Error != "" || res.Count != 0 {
		t.Fatalf("birinchi qidiruv bo'sh bo'lishi kerak: %+v", res)
	}
	if res.Stop || res.Hint != "" {
		t.Fatalf("birinchi bo'sh natijada stop bo'lmasligi kerak: stop=%v hint=%q", res.Stop, res.Hint)
	}

	res, _ = h.do(ctx, q)
	if !res.Stop {
		t.Fatalf("ikkinchi ketma-ket bo'sh natijada stop=true kutilgan: %+v", res)
	}
	if !strings.Contains(res.Hint, "qayta qidirma") {
		t.Fatalf("stop hinti: %q", res.Hint)
	}
	if s, _ := st.State(ctx, "s1"); s.EmptySearches != 2 {
		t.Fatalf("sessiyada sanovchi saqlanishi kerak: %d", s.EmptySearches)
	}
}

// B1: natija topilganda sanovchi nolga qaytadi — keyingi bo'sh qidiruv "birinchi" hisoblanadi.
func TestProductSearch_NonEmptyResetsCounter(t *testing.T) {
	dp, _, st := deps(t)
	ctx, _ := ctxFor(curtains.RoleEmployee)
	h := &productSearchHandler{d: dp}

	miss := productSearchArgs{Query: "yo'q-bunday-mato-12345", Kind: "fabric"}
	if res, _ := h.do(ctx, miss); res.Stop {
		t.Fatalf("birinchi bo'sh natijada stop: %+v", res)
	}

	// topiladigan so'rov — sanovchi nolga tushishi kerak
	hit := productSearchArgs{Query: "Velvet", Kind: "fabric"}
	res, _ := h.do(ctx, hit)
	if res.Error != "" || res.Count == 0 {
		t.Fatalf("topilishi kerak edi: %+v", res)
	}
	if s, _ := st.State(ctx, "s1"); s.EmptySearches != 0 {
		t.Fatalf("sanovchi nolga qaytmadi: %d", s.EmptySearches)
	}

	// endi yana ikki bo'sh — ikkinchisi to'xtatadi (sanovchi 1 dan boshlanadi)
	if res, _ := h.do(ctx, miss); res.Stop {
		t.Fatalf("sanovchi nolga qaytigan bo'lishi kerak edi: %+v", res)
	}
	if res, _ := h.do(ctx, miss); !res.Stop {
		t.Fatalf("ikkinchi bo'sh natija to'xtatishi kerak: %+v", res)
	}
}

// B1: layers rejimida ham natija sananadi — by_layer to'ldirilgan bo'lsa stop chiqmaydi.
func TestProductSearch_LayersResultsCountAsFound(t *testing.T) {
	dp, _, st := deps(t)
	ctx, _ := ctxFor(curtains.RoleEmployee)
	h := &productSearchHandler{d: dp}

	res, _ := h.do(ctx, productSearchArgs{Query: "Velvet", Layers: []string{"portyera", "tul"}})
	if res.Error != "" || res.Count == 0 || len(res.ByLayer) != 2 {
		t.Fatalf("layers natijasi: count=%d layers=%d", res.Count, len(res.ByLayer))
	}
	if res.Stop {
		t.Fatalf("muvaffaqiyatli layers qidiruvida stop chiqmasligi kerak: %+v", res)
	}
	if s, _ := st.State(ctx, "s1"); s.EmptySearches != 0 {
		t.Fatalf("muvaffaqiyatli qidiruv sanovchini oshirmadi: %d", s.EmptySearches)
	}
}

// ---- B2: bo'sh tenant katalogi ----

// emptyTenantCatalog — katalog SYNKRON, lekin bu tenant uchun ma'lumot yo'q.
func emptyTenantCatalog() *curtains.Catalog {
	c := curtains.NewCatalog(curtains.RealClock{}, time.Hour)
	c.ApplySnapshot(curtains.Snapshot{
		Version: 7,
		// Boshqa tenantga tegishli mahsulotlar — bo'sh tenant uchun HISOBGA OLINMAYDI
		Products: []curtains.Product{{ID: "p-1", Name: "X", TenantID: "tenant-2", VariantIDs: []string{"v-1"}}},
		Variants: []curtains.Variant{{ID: "v-1", ProductID: "p-1", TenantID: "tenant-2", Price: d("1000"), IsActive: true}},
		// Global sozlamalar — bo'sh tenantni "to'ldirilgan" deb ko'rsatmasligi kerak
		Services:  []curtains.Service{{Key: "sewing", Name: "Tikuv", Price: d("1000"), IsActive: true}},
		CalcRules: []curtains.CalcRule{{Key: "coef_default", Value: d("2.5")}},
		Policy:    &curtains.DiscountPolicy{ManualMaxPercentEmployee: d("5")},
	})
	return c
}

func emptyTenantDeps() Deps {
	return Deps{Catalog: emptyTenantCatalog(), Calc: &fakeCalc{}, State: &memState{}, Clock: curtains.RealClock{}}
}

// B2 (ADR-020): bo'sh filialda uchala katalog tool'i ham bitta aniq xato beradi.
func TestEmptyTenantCatalog_AllLookupToolsExplain(t *testing.T) {
	ctx, _ := ctxFor(curtains.RoleEmployee)
	dp := emptyTenantDeps()

	cases := []struct {
		name string
		call func() string
	}{
		{"product_search", func() string {
			r, _ := (&productSearchHandler{d: dp}).do(ctx, productSearchArgs{Query: "Velvet"})
			return r.Error
		}},
		{"build_options", func() string {
			r, _ := (&buildOptionsHandler{d: dp}).do(ctx, buildOptionsArgs{})
			return r.Error
		}},
		{"fason_lookup", func() string {
			r, _ := (&fasonLookupHandler{d: dp}).do(ctx, fasonLookupArgs{Room: "living"})
			return r.Error
		}},
	}
	for _, c := range cases {
		if errMsg := c.call(); errMsg != errCatalogEmpty {
			t.Fatalf("%s: %q kutilgan edi", c.name, errMsg)
		}
	}
}

// B2: boshqa tenantda ma'lumot borligi bu tenantni bo'sh qilmaydi.
func TestEmptyTenantCatalog_OtherTenantIgnored(t *testing.T) {
	c := emptyTenantCatalog()
	if !c.View().ForTenant("tenant-1").IsEmpty() {
		t.Fatal("tenant-1 uchun katalog bo'sh bo'lishi kerak")
	}
	if c.View().ForTenant("tenant-2").IsEmpty() {
		t.Fatal("tenant-2 da mahsulot bor — bo'sh bo'lmasligi kerak")
	}
	if c.View().IsEmpty() {
		t.Fatal("filtersiz ko'rinishda mahsulot bor — bo'sh bo'lmasligi kerak")
	}
}

// B2: build_options — barcha paketda Parts yo'q bo'lsa, "hisoblash mumkin emas"
// emas, PAKET SABABLARI (why) jamlanadi.
func TestBuildOptions_AggregatesWhyNotUnavailable(t *testing.T) {
	dp, _, _ := deps(t)
	ctx, _ := ctxFor(curtains.RoleEmployee)

	// O'lcham bor, lekin hech qanday matoga mos kelmaydigan rang — uchala paket
	// ham turli sabab bilan qoladi.
	res, _ := (&buildOptionsHandler{d: dp}).do(ctx, buildOptionsArgs{
		CorniceLengthM: 4, HeightM: 2.6, Color: "yo'q-bunday-rang-98765",
	})
	if res.Error == "" {
		t.Fatalf("xato kutilgan edi: %+v", res)
	}
	if res.Error == errCalcUnavailable {
		t.Fatalf("paket sabablari keltirilishi kerak, umumiy xato emas: %q", res.Error)
	}
	if !strings.Contains(res.Error, "; ") {
		t.Fatalf("uchala sabab ';' bilan birlashtirilishi kerak: %q", res.Error)
	}
	for _, tier := range []string{"economy", "standard", "premium"} {
		if !strings.Contains(res.Error, tier) {
			t.Fatalf("%s sababi yo'q: %q", tier, res.Error)
		}
	}
	// Sabablar har bir paketning o'z Why si — bo'sh emas.
	for _, o := range res.Options {
		if o.Error == "" {
			t.Fatalf("%s paketida sabab yo'q: %+v", o.Tier, o)
		}
	}
}
