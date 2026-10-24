package tools

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

// fakeOrders — tools.Orders o'rnida chaqirilishini sanaydi. ADR-020 dan keyin
// order_draft bu interfeysni ishlatmasligi KUTILADI (TestOrderDraft_DoesNotCallOrders).
type fakeOrders struct {
	mu     sync.Mutex
	called int
	got    curtains.OrderDraft
}

func (f *fakeOrders) SaveOrderDraft(_ context.Context, d curtains.OrderDraft) (curtains.OrderDraftResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.called++
	f.got = d
	return curtains.OrderDraftResult{}, nil
}

// quoteFixture — price_quote natijasiga teng taklif (LastQuote).
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
	// quote — forma bayrog'i chiqmaydi, bu faqat ko'rsatish.
	if res.CustomerFormRequired {
		t.Fatalf("quote'da forma talabi bo'lmasligi kerak: %+v", res)
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

// B6 (ADR-020): draft — taklif + forma talabi. PII yig'ilmaydi, Django'ga
// yozilmaydi, tasdiq sotuvchida.
func TestOrderDraft_DraftRequiresCustomerForm(t *testing.T) {
	dp, _, st := deps(t)
	ctx := setStateWithQuote(st)
	h := &orderDraftHandler{d: dp}

	res, _ := h.do(ctx, orderDraftArgs{Action: "draft"})
	if res.Error != "" {
		t.Fatalf("draft xatosi: %s", res.Error)
	}
	if !res.CustomerFormRequired {
		t.Fatalf("customer_form_required=true kutilgan: %+v", res)
	}
	if res.Quote == nil || res.Summary == nil {
		t.Fatalf("draft taklif/summary bermadi: %+v", res)
	}
	if !strings.Contains(res.Hint, "ilova forma") {
		t.Fatalf("forma hinti: %q", res.Hint)
	}
	if !strings.Contains(res.TotalDisplay, "5 712 000 so'm") {
		t.Fatalf("total_display: %q", res.TotalDisplay)
	}
}

// B6: natija hech qanday PII (ism/telefon/manzil) saqlamasligi kerak.
func TestOrderDraft_ResultHasNoPIIFields(t *testing.T) {
	dp, _, st := deps(t)
	ctx := setStateWithQuote(st)

	res, _ := (&orderDraftHandler{d: dp}).do(ctx, orderDraftArgs{Action: "draft", Note: "eski eshikdan"})
	out, _ := json.Marshal(res)
	raw := string(out)
	for _, banned := range []string{"customer_name", "customer_phone", "address", "order_id", "order_number", "customer_ref"} {
		if strings.Contains(raw, banned) {
			t.Fatalf("natijada %q maydoni qoldi: %s", banned, raw)
		}
	}
}

// B6: PII argumentlari sxemada umuman yo'q — model ularni tool'ga yubora olmaydi.
func TestOrderDraft_ArgsHaveNoPII(t *testing.T) {
	raw, err := json.Marshal(orderDraftArgs{})
	if err != nil {
		t.Fatal(err)
	}
	props := string(raw)
	for _, banned := range []string{"customer_name", "customer_phone", "address", "confirm"} {
		if strings.Contains(props, banned) {
			t.Fatalf("argumentda %q maydoni qoldi: %s", banned, props)
		}
	}
}

// B6: action=confirm yo'q — AI buyurtmani tasdiqlamaydi.
func TestOrderDraft_ConfirmRejected(t *testing.T) {
	dp, _, st := deps(t)
	ctx := setStateWithQuote(st)

	res, _ := (&orderDraftHandler{d: dp}).do(ctx, orderDraftArgs{Action: "confirm"})
	if !strings.Contains(res.Error, "confirm") {
		t.Fatalf("confirm rad etilmishi kerak: %q", res.Error)
	}
}

// B6: order_draft Django'ga HECH QANDAY yozuv qilmaydi.
func TestOrderDraft_DoesNotCallOrders(t *testing.T) {
	dp, _, st := deps(t)
	fo := &fakeOrders{}
	dp.Orders = fo
	ctx := setStateWithQuote(st)

	if _, err := (&orderDraftHandler{d: dp}).do(ctx, orderDraftArgs{Action: "draft", Note: "x"}); err != nil {
		t.Fatal(err)
	}
	if fo.called != 0 {
		t.Fatalf("Orders chaqirildi (calls=%d) — ADR-020 bo'yicha yozilmaydi", fo.called)
	}
}
