package curtains

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestChangedRequirementsInvalidateQuotes(t *testing.T) {
	for _, field := range []string{"budget_max", "cornice_length_m", "height_m", "colors", "style"} {
		t.Run(field, func(t *testing.T) {
			st := NewSessionState()
			st.LastCalc = &CalcResult{}
			st.LastQuote = &CalcResult{}
			st.LastOptions = map[OptionTier]OptionSnapshot{OptionStandard: {}}
			st.SelectedOption = OptionStandard
			st.Stage = StageOrder
			var value any = 2.0
			if field == "colors" || field == "style" {
				value = "new"
			}
			st.Apply("set_context", map[string]any{field: value}, time.Time{})
			if !st.QuoteNeedsReview || st.LastQuote != nil || len(st.LastOptions) != 0 || st.SelectedOption != "" || st.Stage == StageOrder {
				t.Fatalf("stale quote retained: %+v", st)
			}
			if !strings.Contains(st.PromptBlock(), "eskirgan") {
				t.Fatal("model must see stale status")
			}
			dimensions := field != "budget_max"
			if dimensions != (st.LastCalc == nil) {
				t.Fatal("material requirement changes discard the material calculation")
			}
		})
	}
}

func TestSameRequirementsKeepQuote(t *testing.T) {
	st := NewSessionState()
	st.BudgetMax = decimal.NewFromInt(2000000)
	st.LastQuote = &CalcResult{}
	st.Apply("set_context", map[string]any{"budget_max": 2000000}, time.Time{})
	if st.QuoteNeedsReview || st.LastQuote == nil {
		t.Fatal("unchanged requirement invalidated quote")
	}
}
