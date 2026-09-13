package tools

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/muhammadvadud/alora-ai/internal/agent"
	"github.com/muhammadvadud/alora-ai/internal/curtains"
	"github.com/muhammadvadud/alora-ai/internal/session"
)

const (
	vPortyera = "11111111-1111-4111-8111-111111111111"
	vCheap    = "12121212-1212-4121-8121-121212121212"
	vGul      = "44444444-4444-4444-8444-444444444444"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

// testCatalog — 2 ta mato (bir mahsulot, ikki narx) + 1 ta bezak; pricing defaultlari.
func testCatalog() *curtains.Catalog {
	c := curtains.NewCatalog(curtains.RealClock{}, time.Hour)
	c.ApplySnapshot(curtains.Snapshot{
		Version: 1,
		Products: []curtains.Product{
			{ID: "p1", CategoryID: "c1", Name: "Velvet Milano", Style: "hi_tech", Pattern: "plain",
				RecommendedRooms: []string{"living"}, Segment: curtains.SegmentStandard, VariantIDs: []string{vPortyera, vCheap}},
			{ID: "p2", CategoryID: "c2", Name: "Dekorativ gul", Segment: curtains.SegmentStandard, VariantIDs: []string{vGul}},
		},
		Variants: []curtains.Variant{
			{ID: vPortyera, ProductID: "p1", SKU: "VM-GRAFIT", ColorName: "grafit", WidthM: d("2.8"), Price: d("180000"), PriceUnit: curtains.PerSqm, IsActive: true},
			{ID: vCheap, ProductID: "p1", SKU: "VM-BEJ", ColorName: "bej", WidthM: d("2.8"), Price: d("120000"), PriceUnit: curtains.PerSqm, IsActive: true},
			{ID: vGul, ProductID: "p2", SKU: "GUL-OLTIN", ColorName: "oltin", Price: d("85000"), PriceUnit: curtains.PerPiece, IsActive: true},
		},
		Services: []curtains.Service{{Key: "sewing", Name: "Tikuv", Price: d("12500"), Unit: curtains.PerSqm, IsActive: true}},
		Policy:   &curtains.DiscountPolicy{ManualMaxPercentEmployee: d("5"), ManualMaxPercentManager: d("15")},
		Defaults: &curtains.OrderDefaults{PrepaymentPercent: d("50"), DeliveryAmount: d("30000"), DeadlineDays: 14},
	})
	return c
}

// fakeCalc — Django o'rnida: kirishni eslab qoladi, tayyor natija qaytaradi.
type fakeCalc struct {
	last curtains.CalcInput
	err  error
}

func (f *fakeCalc) Calculate(_ context.Context, in curtains.CalcInput) (curtains.CalcResult, error) {
	f.last = in
	if f.err != nil {
		return curtains.CalcResult{}, f.err
	}
	m := in.Measurements[0]
	res := curtains.CalcResult{Subtotal: d("5382000"), Total: d("5382000"), DiscountPercent: in.DiscountPercent, Warnings: []string{}}
	mr := curtains.MeasurementResult{Length: m.Length, Height: m.Height}
	for _, p := range m.Parts {
		mr.Parts = append(mr.Parts, curtains.PartResult{VariantID: p.VariantID, SKU: "VM-GRAFIT", Role: p.Role,
			WidthM: m.Length, HeightM: m.Height, Coef: d("2.5"), LineTotal: d("5382000")})
	}
	res.Measurements = []curtains.MeasurementResult{mr}
	return res, nil
}

type memState struct {
	mu sync.Mutex
	m  map[string]curtains.SessionState
}

func (s *memState) State(_ context.Context, id string) (curtains.SessionState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.m[id]; ok {
		return st, nil
	}
	return curtains.NewSessionState(), nil
}

func (s *memState) SetState(_ context.Context, id string, st curtains.SessionState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string]curtains.SessionState{}
	}
	s.m[id] = st
	return nil
}

func deps(t *testing.T) (Deps, *fakeCalc, *memState) {
	t.Helper()
	calc := &fakeCalc{}
	st := &memState{}
	return Deps{Catalog: testCatalog(), Calc: calc, State: st, Clock: curtains.RealClock{}}, calc, st
}

func ctxFor(role curtains.UserRole) (context.Context, *agent.ProductCollector) {
	ctx := session.WithID(context.Background(), "s1")
	ctx = curtains.WithUser(ctx, curtains.UserInfo{Role: role})
	return agent.WithProductCollector(ctx)
}

func TestAlora_ConstructorsAndNames(t *testing.T) {
	dp, _, _ := deps(t)
	want := map[string]bool{"product_search": true, "curtain_calculate": true, "price_quote": true, "build_options": true, "order_draft": true, "set_context": true}
	for _, tl := range Alora(dp) {
		info, err := tl.Info(context.Background())
		if err != nil || !want[info.Name] {
			t.Fatalf("tool nomi kutilmagan: %v %v", info, err)
		}
		delete(want, info.Name)
	}
	if len(want) != 0 {
		t.Fatalf("yetishmayotgan tool'lar: %v", want)
	}
}

func TestProductSearch_ReturnsCardsAndAttachesProducts(t *testing.T) {
	dp, _, st := deps(t)
	ctx, col := ctxFor(curtains.RoleEmployee)
	res, err := (&productSearchHandler{d: dp}).do(ctx, productSearchArgs{Room: "living", Style: "hi_tech", Color: "grafit", Kind: "fabric"})
	if err != nil || res.Error != "" {
		t.Fatalf("xato: %v %s", err, res.Error)
	}
	if res.Count != 2 || res.Products[0].VariantID != vPortyera {
		t.Fatalf("grafit birinchi bo'lishi kerak: %+v", res.Products)
	}
	if res.Products[0].PriceDisplay != "180 000 so'm" || len(res.Products[0].Why) == 0 {
		t.Fatalf("display/why: %+v", res.Products[0])
	}
	if refs := col.Drain(); len(refs) != 2 || refs[0].ProductName != "Velvet Milano" {
		t.Fatalf("AttachProducts: %+v", refs)
	}
	if s, _ := st.State(ctx, "s1"); s.Room != "living" || s.Stage != curtains.StageRecommend {
		t.Fatalf("state yangilanmadi: %+v", s)
	}
}

func TestProductSearch_CatalogNotReady(t *testing.T) {
	dp := Deps{Catalog: curtains.NewCatalog(curtains.RealClock{}, time.Hour)}
	res, _ := (&productSearchHandler{d: dp}).do(context.Background(), productSearchArgs{})
	if res.Error != errCatalogNotReady {
		t.Fatalf("katalog bo'sh xatosi kutilgan: %q", res.Error)
	}
}

func TestCurtainCalculate_Success(t *testing.T) {
	dp, calc, st := deps(t)
	ctx, _ := ctxFor(curtains.RoleEmployee)
	res, _ := (&curtainCalcHandler{d: dp}).do(ctx, curtainCalcArgs{
		CorniceLengthM: 4.2, HeightM: 2.6, RoomName: "Mehmonxona",
		Parts:       []calcPart{{VariantID: vPortyera, Role: "portyera", Coefficient: 2.5}},
		Accessories: []calcAccessory{{VariantID: vGul, Qty: 2}},
	})
	if res.Error != "" {
		t.Fatalf("xato: %s", res.Error)
	}
	if calc.last.Role != curtains.RoleEmployee || calc.last.Measurements[0].Parts[0].WidthM != nil {
		t.Fatalf("Django'ga so'rov noto'g'ri: %+v", calc.last)
	}
	if res.Display.(map[string]string)["total"] != "5 382 000 so'm" {
		t.Fatalf("display: %v", res.Display)
	}
	if !strings.Contains(res.Explanation, "chok kerak emas") || !strings.Contains(res.Explanation, "VM-BEJ") {
		t.Fatalf("explanation: %q", res.Explanation)
	}
	if s, _ := st.State(ctx, "s1"); s.LastCalc == nil || s.Stage != curtains.StageCalc || !s.CorniceLengthM.Equal(d("4.2")) {
		t.Fatalf("state: %+v", s)
	}
}

func TestCurtainCalculate_ValidationAndUnavailable(t *testing.T) {
	dp, calc, _ := deps(t)
	ctx, _ := ctxFor(curtains.RoleEmployee)
	h := &curtainCalcHandler{d: dp}

	parts := make([]calcPart, 11)
	for i := range parts {
		parts[i] = calcPart{VariantID: vPortyera, Role: "portyera"}
	}
	if res, _ := h.do(ctx, curtainCalcArgs{CorniceLengthM: 4, HeightM: 2.6, Parts: parts}); res.Error == "" {
		t.Fatal("11 bo'lak uchun validatsiya xatosi kutilgan")
	}
	if res, _ := h.do(ctx, curtainCalcArgs{CorniceLengthM: 50, HeightM: 2.6, Parts: parts[:1]}); !strings.Contains(res.Error, "50") {
		t.Fatalf("L=50 xatosi: %q", res.Error)
	}
	if res, _ := h.do(ctx, curtainCalcArgs{CorniceLengthM: 4, HeightM: 2.6, Parts: []calcPart{{VariantID: "yo'q", Role: "portyera"}}}); res.Error == "" {
		t.Fatal("noma'lum variant xatosi kutilgan")
	}
	calc.err = errors.New("dial tcp: connection refused")
	if res, _ := h.do(ctx, curtainCalcArgs{CorniceLengthM: 4, HeightM: 2.6, Parts: parts[:1]}); res.Error != errCalcUnavailable {
		t.Fatalf("Django yotganda: %q", res.Error)
	}
	calc.err = &curtains.ValidationError{Field: "variant_id", Msg: "Variant topilmadi"}
	if res, _ := h.do(ctx, curtainCalcArgs{CorniceLengthM: 4, HeightM: 2.6, Parts: parts[:1]}); res.Error != "Variant topilmadi" {
		t.Fatalf("Django ValidationError matni: %q", res.Error)
	}
}

func TestPriceQuote_RequiresCalcAndRespectsRoleLimit(t *testing.T) {
	dp, calc, _ := deps(t)
	ctx, _ := ctxFor(curtains.RoleEmployee)
	q := &priceQuoteHandler{d: dp}

	if res, _ := q.do(ctx, priceQuoteArgs{}); !strings.Contains(res.Error, "curtain_calculate") {
		t.Fatalf("hisobsiz quote: %q", res.Error)
	}
	_, _ = (&curtainCalcHandler{d: dp}).do(ctx, curtainCalcArgs{CorniceLengthM: 4.2, HeightM: 2.6,
		Parts: []calcPart{{VariantID: vPortyera, Role: "portyera"}}})

	if res, _ := q.do(ctx, priceQuoteArgs{DiscountPct: 12}); res.Error != "chegirma chegarasi 5% — 12% berib bo'lmaydi" {
		t.Fatalf("chegirma chegarasi: %q", res.Error)
	}
	res, _ := q.do(ctx, priceQuoteArgs{DiscountPct: 5, NeedInstall: true, NeedCornice: true, NeedDelivery: true})
	if res.Error != "" {
		t.Fatalf("xato: %s", res.Error)
	}
	if got := strings.Join(calc.last.Services, ","); got != "sewing,tape,hook,install" || !calc.last.Delivery {
		t.Fatalf("xizmatlar: %s delivery=%v", got, calc.last.Delivery)
	}
	if res.Prepayment != "2 691 000 so'm" || res.Balance != "2 691 000 so'm" {
		t.Fatalf("avans/qoldiq: %s / %s", res.Prepayment, res.Balance)
	}
	if len(res.Warnings) == 0 || !strings.Contains(res.Warnings[0], "karniz") {
		t.Fatalf("karniz ogohlantirishi kutilgan: %v", res.Warnings)
	}
	// manager uchun 12% o'tadi
	mctx, _ := ctxFor(curtains.RoleManager)
	_, _ = (&curtainCalcHandler{d: dp}).do(mctx, curtainCalcArgs{CorniceLengthM: 4.2, HeightM: 2.6,
		Parts: []calcPart{{VariantID: vPortyera, Role: "portyera"}}})
	if res, _ := q.do(mctx, priceQuoteArgs{DiscountPct: 12}); res.Error != "" {
		t.Fatalf("manager 12%%: %q", res.Error)
	}
}

func TestSetContext_NextQuestion(t *testing.T) {
	dp, _, st := deps(t)
	ctx, _ := ctxFor(curtains.RoleEmployee)
	res, _ := (&setContextHandler{d: dp}).do(ctx, setContextArgs{Room: "living"})
	if res.Error != "" || res.NextQuestion == "" || strings.Contains(res.NextQuestion, "xona") {
		t.Fatalf("room dan keyin yorug'lik so'ralishi kerak: %+v", res)
	}
	if s, _ := st.State(ctx, "s1"); s.Room != "living" {
		t.Fatalf("state: %+v", s)
	}
}

func TestDisplayMoney(t *testing.T) {
	for in, want := range map[string]string{"0": "0 so'm", "999": "999 so'm", "5382000": "5 382 000 so'm", "1234567.49": "1 234 567 so'm"} {
		if got := displayMoney(d(in)); got != want {
			t.Errorf("%s: %q != %q", in, got, want)
		}
	}
}
