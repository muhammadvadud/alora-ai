package tools

import (
	"strings"
	"testing"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

func TestCalculate_ExplicitServicesReachDjango(t *testing.T) {
	for _, services := range [][]string{{"sewing", "sewing"}, {}} {
		dp, calc, _ := deps(t)
		ctx, _ := ctxFor(curtains.RoleEmployee)
		out, err := (&curtainCalcHandler{d: dp}).do(ctx, curtainCalcArgs{
			CorniceLengthM: 3, HeightM: 2.6, Services: services,
			Parts: []calcPart{{VariantID: vPortyera, Role: "portyera", Coefficient: 2}},
		})
		if err != nil || out.Error != "" {
			t.Fatalf("calculate: %v %+v", err, out)
		}
		want := ""
		if len(services) > 0 {
			want = "sewing"
		}
		if strings.Join(calc.last.Services, ",") != want {
			t.Fatalf("services lost/duplicated: %v", calc.last.Services)
		}
	}
}

func TestPriceQuote_EmptyRemovesOmittedPreservesServices(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		dp, calc, state := deps(t)
		ctx := setStateWithQuote(state)
		st, _ := state.State(ctx, "s1")
		st.LastCalc = st.LastQuote
		st.LastQuote.Services = []curtains.ServiceLine{{Key: "sewing"}}
		_ = state.SetState(ctx, "s1", st)
		args := priceQuoteArgs{}
		if explicit {
			args.Services = []string{}
		}
		out, err := (&priceQuoteHandler{d: dp}).do(ctx, args)
		if err != nil || out.Error != "" {
			t.Fatalf("quote: %v %+v", err, out)
		}
		want := "sewing"
		if explicit {
			want = ""
		}
		if strings.Join(calc.last.Services, ",") != want {
			t.Fatalf("explicit=%v services=%v", explicit, calc.last.Services)
		}
	}
}

func TestDraft_RejectsMissingServiceAndStaleQuote(t *testing.T) {
	dp, _, state := deps(t)
	ctx := setStateWithQuote(state)
	h := &orderDraftHandler{d: dp}
	missing, _ := h.do(ctx, orderDraftArgs{Action: "draft", RequiredServices: []string{"install"}})
	if !strings.Contains(missing.Error, "install") || missing.CustomerFormRequired {
		t.Fatalf("missing service accepted: %+v", missing)
	}
	valid, _ := h.do(ctx, orderDraftArgs{Action: "draft", RequiredServices: []string{"sewing"}})
	if valid.Error != "" || !valid.CustomerFormRequired {
		t.Fatalf("valid quote refused: %+v", valid)
	}
	st, _ := state.State(ctx, "s1")
	st.QuoteNeedsReview = true
	_ = state.SetState(ctx, "s1", st)
	stale, _ := h.do(ctx, orderDraftArgs{Action: "draft"})
	if stale.Error == "" || stale.CustomerFormRequired {
		t.Fatalf("stale quote accepted: %+v", stale)
	}
}
