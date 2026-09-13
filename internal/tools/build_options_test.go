package tools

import (
	"context"
	"errors"
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
}

func (f *optionsCalc) Calculate(_ context.Context, in curtains.CalcInput) (curtains.CalcResult, error) {
	f.calls.Add(1)
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
		Products:  []curtains.Product{{ID: "eco", Segment: curtains.SegmentEconomy, VariantIDs: []string{"e"}}, {ID: "std", Segment: curtains.SegmentStandard, Transparency: "transparent", VariantIDs: []string{"s"}}, {ID: "pre", Segment: curtains.SegmentPremium, Transparency: "transparent", VariantIDs: []string{"p"}}, {ID: "black", Segment: curtains.SegmentPremium, Transparency: "blackout", VariantIDs: []string{"b"}}},
		Variants:  []curtains.Variant{{ID: "e", ProductID: "eco", Price: decimal.NewFromInt(1), PriceUnit: curtains.PerSqm, IsActive: true}, {ID: "s", ProductID: "std", Price: decimal.NewFromInt(2), PriceUnit: curtains.PerSqm, IsActive: true}, {ID: "p", ProductID: "pre", Price: decimal.NewFromInt(3), PriceUnit: curtains.PerSqm, IsActive: true}, {ID: "b", ProductID: "black", Price: decimal.NewFromInt(2), PriceUnit: curtains.PerSqm, IsActive: true}},
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
