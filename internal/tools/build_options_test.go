package tools

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
	"github.com/muhammadvadud/alora-ai/internal/session"
)

type optionsCalc struct {
	calls        atomic.Int32
	failStandard atomic.Bool
	mu           sync.Mutex
	seen         []curtains.CalcInput
}

func (f *optionsCalc) inputs() []curtains.CalcInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]curtains.CalcInput(nil), f.seen...)
}

func (f *optionsCalc) Calculate(_ context.Context, in curtains.CalcInput) (curtains.CalcResult, error) {
	f.calls.Add(1)
	f.mu.Lock()
	f.seen = append(f.seen, in)
	f.mu.Unlock()
	if f.failStandard.Load() && len(in.Measurements[0].Parts) == 2 {
		return curtains.CalcResult{}, errors.New("django unavailable")
	}
	return curtains.CalcResult{Total: decimal.NewFromInt(int64(len(in.Measurements[0].Parts) * 1000000))}, nil
}

type optionsState struct{ state curtains.SessionState }

func (s *optionsState) State(context.Context, string) (curtains.SessionState, error) {
	return s.state, nil
}
func (s *optionsState) SetState(_ context.Context, _ string, st curtains.SessionState) error {
	s.state = st
	return nil
}

func optionsDeps() (Deps, *optionsCalc, *optionsState) {
	clock := curtains.RealClock{}
	c := curtains.NewCatalog(clock, time.Hour)
	c.ApplySnapshot(curtains.Snapshot{Version: 1,
		// Qatlamlar teg orqali: portyera (eco/std/pre), tul (faqat standard — premium
		// paket pastroq segmentdan oladi), astar (premium blackout).
		Products: []curtains.Product{
			{ID: "eco", Segment: curtains.SegmentEconomy, VariantIDs: []string{"e"}},
			{ID: "std", Segment: curtains.SegmentStandard, VariantIDs: []string{"s"}},
			{ID: "tul", Segment: curtains.SegmentStandard, Transparency: "transparent", Tags: map[string]any{"tul": true}, VariantIDs: []string{"t"}},
			{ID: "pre", Segment: curtains.SegmentPremium, VariantIDs: []string{"p"}},
			{ID: "black", Segment: curtains.SegmentPremium, Transparency: "blackout", Tags: map[string]any{"astar": true}, VariantIDs: []string{"b"}},
		},
		Variants: []curtains.Variant{
			{ID: "e", ProductID: "eco", Price: decimal.NewFromInt(1), PriceUnit: curtains.PerSqm, IsActive: true},
			{ID: "s", ProductID: "std", Price: decimal.NewFromInt(2), PriceUnit: curtains.PerSqm, IsActive: true},
			{ID: "t", ProductID: "tul", Price: decimal.NewFromInt(1), PriceUnit: curtains.PerSqm, IsActive: true},
			{ID: "p", ProductID: "pre", Price: decimal.NewFromInt(3), PriceUnit: curtains.PerSqm, IsActive: true},
			{ID: "b", ProductID: "black", Price: decimal.NewFromInt(2), PriceUnit: curtains.PerSqm, IsActive: true},
		},
		// Katalogda "cornice" yo'q — premium paket uni qo'shmasligi va ogohlantirishi kerak.
		Services:  []curtains.Service{{Key: "sewing", IsActive: true}, {Key: "tape", IsActive: true}, {Key: "hook", IsActive: true}, {Key: "install", IsActive: true}},
		CalcRules: []curtains.CalcRule{{Key: "coef_default", Value: decimal.RequireFromString("2.5")}}})
	st := &optionsState{state: curtains.SessionState{CorniceLengthM: decimal.NewFromInt(4), HeightM: decimal.RequireFromString("2.6")}}
	calc := &optionsCalc{}
	return Deps{Catalog: c, Calc: calc, State: st, Clock: clock}, calc, st
}

func TestBuildOptions_ParallelAndState(t *testing.T) {
	d, calc, state := optionsDeps()
	ctx := session.WithID(context.Background(), "options-s1")
	res, err := (&buildOptionsHandler{d: d}).do(ctx, buildOptionsArgs{})
	if err != nil || res.Error != "" || calc.calls.Load() != 3 {
		t.Fatalf("uchta hisob: err=%v result=%+v calls=%d", err, res, calc.calls.Load())
	}
	if res.Options[0].Total == "" || res.Options[0].Display == nil || state.state.Stage != curtains.StageVariants {
		t.Fatalf("natija/display/state: %+v state=%+v", res.Options[0], state.state)
	}
	if len(res.Options[1].Variants) != 2 || res.Options[1].Variants[0].VariantID == "" || res.Options[1].Variants[1].VariantID == "" {
		t.Fatalf("paket variant ID'lari qaytishi kerak: %+v", res.Options[1].Variants)
	}
}

func TestBuildOptions_OneCalculationErrorKeepsOthers(t *testing.T) {
	d, calc, _ := optionsDeps()
	calc.failStandard.Store(true)
	ctx := session.WithID(context.Background(), "options-s1")
	res, _ := (&buildOptionsHandler{d: d}).do(ctx, buildOptionsArgs{})
	if res.Error != "" || res.Options[1].Error == "" || res.Options[0].Error != "" || res.Options[2].Error != "" {
		t.Fatalf("bitta xato qolgan variantlarni to'smasligi kerak: %+v", res)
	}
}

func TestBuildOptions_MissingCatalogServiceIsDroppedWithWarning(t *testing.T) {
	d, calc, _ := optionsDeps()
	ctx := session.WithID(context.Background(), "options-s2")
	res, err := (&buildOptionsHandler{d: d}).do(ctx, buildOptionsArgs{})
	if err != nil || res.Error != "" {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	premium := res.Options[2]
	if premium.Error != "" || premium.Total == "" {
		t.Fatalf("premium katalogda karniz bo'lmasa ham hisoblanishi kerak: %+v", premium)
	}
	for _, in := range calc.inputs() {
		for _, sv := range in.Services {
			if sv == "cornice" {
				t.Fatalf("katalogda yo'q 'cornice' xizmati hisobga yuborildi: %v", in.Services)
			}
		}
	}
	found := false
	for _, w := range res.Warnings {
		found = found || w == warnCorniceMissing
	}
	if !found {
		t.Fatalf("karniz ogohlantirishi yo'q: %v", res.Warnings)
	}
	// Tul premium'da yo'q — standard'dagi tul olinadi, lambrekan emas.
	for _, line := range premium.Lines {
		if strings.HasPrefix(line, "tul:") && !strings.Contains(line, "tul") {
			t.Fatalf("premium tul qatlami noto'g'ri: %q", line)
		}
	}
}
