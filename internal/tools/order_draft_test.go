package tools

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

// quoteFixture — price_quote natijasiga teng taklif (LastQuote); items_json sxemasi
// testconfirmda CalcResult shaklidan tiklanadi.
func quoteFixture() *curtains.CalcResult {
	return &curtains.CalcResult{
		Measurements: []curtains.MeasurementResult{{
			Length: d("3.000"), Height: d("2.500"),
			Parts: []curtains.PartResult{{
				VariantID: vPortyera, SKU: "VM-GRAFIT", Role: curtains.RolePortyera,
				WidthM: d("3.000"), HeightM: d("2.500"), Coef: d("2.5"),
			}},
			Accessories: []curtains.AccessoryResult{{VariantID: vGul, Qty: d("2")}},
		}},
		Services: []curtains.ServiceLine{
			{Key: "sewing", Name: "Tikuv"},
			{Key: "tape", Name: "Tasma"},
			{Key: "delivery", Name: "Yetkazish", IsDelivery: true},
		},
		Subtotal:              d("5382000"),
		ManualDiscountPercent: d("5"),
		DiscountPercent:       d("5"),
		Total:                 d("5712000"),
		Warnings:              []string{},
	}
}

// setStateWithQuote — sessiya holatiga oxirgi taklifni joylaydi, kontekst qaytaradi.
func setStateWithQuote(st *memState) context.Context {
	ctx, _ := ctxFor(curtains.RoleEmployee)
	s := curtains.NewSessionState()
	s.LastQuote = quoteFixture()
	_ = st.SetState(ctx, "s1", s)
	return ctx
}

// fakeOrders — tools.Orders o'rnida: yuborilgan draftni ushlab, tayyor natija beradi.
type fakeOrders struct {
	mu     sync.Mutex
	called int
	got    curtains.OrderDraft
	res    curtains.OrderDraftResult
	err    error
}

func (f *fakeOrders) SaveOrderDraft(_ context.Context, d curtains.OrderDraft) (curtains.OrderDraftResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.called++
	f.got = d
	if f.err != nil {
		return curtains.OrderDraftResult{}, f.err
	}
	return f.res, nil
}

func TestOrderDraft_Quote(t *testing.T) {
	dp, _, st := deps(t)
	ctx := setStateWithQuote(st)
	h := &orderDraftHandler{d: dp}

	res, _ := h.do(ctx, orderDraftArgs{Action: "quote"})
	if res.Error != "" || res.Quote == nil {
		t.Fatalf("quote: %+v", res)
	}
	if _, ok := res.Display.(map[string]string); !ok {
		t.Fatalf("display kutilgan: %+v", res.Display)
	}
}

func TestOrderDraft_QuoteFallsBackToLastCalc(t *testing.T) {
	dp, _, st := deps(t)
	ctx, _ := ctxFor(curtains.RoleEmployee)
	s := curtains.NewSessionState()
	s.LastCalc = quoteFixture()
	_ = st.SetState(ctx, "s1", s)

	res, _ := (&orderDraftHandler{d: dp}).do(ctx, orderDraftArgs{Action: "quote"})
	if res.Error != "" || res.Quote == nil {
		t.Fatalf("LastCalc'dan quote: %+v", res)
	}
}

func TestOrderDraft_NoQuote(t *testing.T) {
	dp, _, _ := deps(t)
	ctx, _ := ctxFor(curtains.RoleEmployee)
	res, _ := (&orderDraftHandler{d: dp}).do(ctx, orderDraftArgs{Action: "quote"})
	if !strings.Contains(res.Error, "price_quote") {
		t.Fatalf("taklifsiz xato: %q", res.Error)
	}
}

func TestOrderDraft_DraftMissing(t *testing.T) {
	dp, _, st := deps(t)
	ctx := setStateWithQuote(st)
	h := &orderDraftHandler{d: dp}

	res, _ := h.do(ctx, orderDraftArgs{Action: "draft"})
	want := []string{"customer_name", "customer_phone", "address"}
	if len(res.Missing) != 3 || res.Missing[0] != want[0] || res.Missing[1] != want[1] || res.Missing[2] != want[2] {
		t.Fatalf("missing: %v", res.Missing)
	}
	if res.Ready || res.Hint == "" {
		t.Fatalf("ready/hint: %+v", res)
	}
	// qisman to'ldirilgan bo'lsa ham tartib saqlanadi
	res, _ = h.do(ctx, orderDraftArgs{Action: "draft", CustomerName: "Ali"})
	if len(res.Missing) != 2 || res.Missing[0] != "customer_phone" {
		t.Fatalf("qisman missing: %v", res.Missing)
	}
}

func TestOrderDraft_DraftReadyPIIHidden(t *testing.T) {
	dp, _, st := deps(t)
	ctx := setStateWithQuote(st)
	h := &orderDraftHandler{d: dp}

	res, _ := h.do(ctx, orderDraftArgs{
		Action: "draft", CustomerName: "Ali", CustomerPhone: "+998901234567", Address: "Toshkent, Chilonzor",
	})
	if res.Error != "" || !res.Ready || res.Summary == nil {
		t.Fatalf("ready: %+v", res)
	}
	out, _ := json.Marshal(res)
	if strings.Contains(string(out), "Ali") || strings.Contains(string(out), "8901234567") {
		t.Fatalf("PII model kontekstiga tushib qolgan: %s", out)
	}
	if !strings.Contains(string(out), "mijoz #4567") {
		t.Fatalf("customer_ref kutilgan: %s", out)
	}
}

func TestOrderDraft_ConfirmSuccess(t *testing.T) {
	dp, _, st := deps(t)
	fo := &fakeOrders{res: curtains.OrderDraftResult{OrderID: "order-99", OrderNumber: "ALR-2026-000099"}}
	dp.Orders = fo
	ctx := setStateWithQuote(st)
	h := &orderDraftHandler{d: dp}

	res, _ := h.do(ctx, orderDraftArgs{
		Action: "confirm", CustomerName: "Ali", CustomerPhone: "+998901234567", Address: "Toshkent, Chilonzor",
		Note: "oldingi eshikdan kiring",
	})
	if res.Error != "" {
		t.Fatalf("xato: %s", res.Error)
	}
	if res.OrderID != "order-99" || res.OrderNumber != "ALR-2026-000099" || res.CustomerRef != "mijoz #4567" {
		t.Fatalf("natija: %+v", res)
	}
	if !strings.Contains(res.TotalDisplay, "5 712 000 so'm") {
		t.Fatalf("total_display: %q", res.TotalDisplay)
	}

	// Django'ga to'liq telefon yuborilgan, natijada yo'q
	fo.mu.Lock()
	if fo.called != 1 || fo.got.CustomerPhone != "+998901234567" {
		t.Fatalf("draft: %+v (calls=%d)", fo.got, fo.called)
	}
	fo.mu.Unlock()
	if strings.Contains(res.CustomerRef, "8901234567") {
		t.Fatalf("telefon natijada: %+v", res)
	}

	// items_json Django payload sxemasi (calc_cases.json input formati)
	var payload struct {
		Measurements []struct {
			Length string `json:"length"`
			Parts  []struct {
				Role string `json:"role"`
				Coef string `json:"coef"`
			} `json:"parts"`
		} `json:"measurements"`
		Services        []string `json:"services"`
		Delivery        bool     `json:"delivery"`
		DiscountPercent string   `json:"discount_percent"`
	}
	if err := json.Unmarshal([]byte(fo.got.ItemsJSON), &payload); err != nil {
		t.Fatalf("items_json sxema: %v — %s", err, fo.got.ItemsJSON)
	}
	if len(payload.Measurements) != 1 || payload.Measurements[0].Length != "3.000" ||
		payload.Measurements[0].Parts[0].Role != "portyera" || payload.Measurements[0].Parts[0].Coef != "2.5" {
		t.Fatalf("measurements: %+v", payload.Measurements)
	}
	if strings.Join(payload.Services, ",") != "sewing,tape" || !payload.Delivery || payload.DiscountPercent != "5" {
		t.Fatalf("services/delivery/discount: %+v", payload)
	}

	if s, _ := st.State(ctx, "s1"); s.Stage != curtains.StageOrder {
		t.Fatalf("stage=order kutilgan: %s", s.Stage)
	}
}

func TestOrderDraft_ConfirmDjangoError(t *testing.T) {
	dp, _, st := deps(t)
	dp.Orders = &fakeOrders{err: &curtains.ValidationError{Field: "order", Msg: "manzil to'liq emas"}}
	ctx := setStateWithQuote(st)

	res, _ := (&orderDraftHandler{d: dp}).do(ctx, orderDraftArgs{
		Action: "confirm", CustomerName: "Ali", CustomerPhone: "+998901234567",
		Address: "Toshkent, Chilonzor",
	})
	if res.Error != "manzil to'liq emas" {
		t.Fatalf("Django xatosi: %q", res.Error)
	}
	if s, _ := st.State(ctx, "s1"); s.Stage != curtains.StageNeed {
		// muvaffaqiyatsiz confirm stage ni o'zgartirmaydi
		t.Fatalf("stage o'zgarmasligi kerak: %s", s.Stage)
	}
}

func TestOrderDraft_ConfirmOrdersNil(t *testing.T) {
	dp, _, st := deps(t)
	ctx := setStateWithQuote(st)

	res, _ := (&orderDraftHandler{d: dp}).do(ctx, orderDraftArgs{
		Action: "confirm", CustomerName: "Ali", CustomerPhone: "+998901234567", Address: "X",
	})
	if res.Error != "buyurtma xizmati ulanmagan" {
		t.Fatalf("nil Orders: %q", res.Error)
	}
}
