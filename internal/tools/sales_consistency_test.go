package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
	"github.com/muhammadvadud/alora-ai/internal/session"
)

func TestPriceQuote_ServiceFlagsNeverDuplicateExplicitServices(t *testing.T) {
	deps, calc, state := deps(t)
	ctx := setStateWithQuote(state)
	current, err := state.State(ctx, "s1")
	if err != nil {
		t.Fatal(err)
	}
	current.LastCalc = current.LastQuote
	if err := state.SetState(ctx, "s1", current); err != nil {
		t.Fatal(err)
	}
	result, err := (&priceQuoteHandler{d: deps}).do(ctx, priceQuoteArgs{
		Services: []string{"sewing", "sewing"}, NeedInstall: true, NeedCornice: true,
	})
	if err != nil || result.Error != "" {
		t.Fatalf("quote: %v %+v", err, result)
	}
	counts := map[string]int{}
	for _, service := range calc.last.Services {
		counts[service]++
		if counts[service] > 1 {
			t.Fatalf("takroriy xizmat narxlanmasligi kerak: %+v", calc.last.Services)
		}
	}
	if counts["sewing"] != 1 {
		t.Fatalf("mavjud xizmat yo'qolmasligi kerak: %+v", calc.last.Services)
	}
	requested := normalizeServiceKeys(append(services(priceQuoteArgs{Services: []string{"install", "cornice"}, NeedInstall: true}), "cornice"))
	if strings.Join(requested, ",") != "install,cornice" {
		t.Fatalf("bayroq+ro'yxat: %+v", requested)
	}
}

func TestOrderDraft_SelectedPackagePersistsWithoutRecalculation(t *testing.T) {
	deps, calc, state := optionsDeps()
	ctx := session.WithID(context.Background(), "sales-choice")
	_, err := (&buildOptionsHandler{d: deps}).do(ctx, buildOptionsArgs{})
	if err != nil {
		t.Fatal(err)
	}
	calls := calc.calls.Load()
	result, err := (&orderDraftHandler{d: deps}).do(ctx, orderDraftArgs{Action: "draft", OptionTier: "standard"})
	if err != nil || result.Error != "" || !result.CustomerFormRequired {
		t.Fatalf("draft: %v %+v", err, result)
	}
	if state.state.Stage != curtains.StageOrder || state.state.SelectedOption != curtains.OptionStandard {
		t.Fatalf("tanlov/order holati saqlanmadi: %+v", state.state)
	}
	if calc.calls.Load() != calls {
		t.Fatal("tayyor paket uchun yana hisob chaqirilmasligi kerak")
	}
	if !strings.Contains(state.state.PromptBlock(), "tanlangan_paket=standard") || !strings.Contains(state.state.PromptBlock(), "qoralama tayyor") {
		t.Fatalf("keyingi javob tanlovni bilishi kerak: %s", state.state.PromptBlock())
	}
	encoded, err := json.Marshal(state.state)
	if err != nil {
		t.Fatal(err)
	}
	var restored curtains.SessionState
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.SelectedOption != curtains.OptionStandard {
		t.Fatal("tanlov persistent holatda yo'qoldi")
	}
}

func TestRepeatedOptionSelectionKeepsAdjustedQuote(t *testing.T) {
	state := curtains.SessionState{LastOptions: map[curtains.OptionTier]curtains.OptionSnapshot{
		curtains.OptionStandard: {Result: curtains.CalcResult{Total: d("100000")}},
	}}
	if message := selectOption(&state, "standard"); message != "" {
		t.Fatal(message)
	}
	state.LastQuote = &curtains.CalcResult{Total: d("95000")}
	if message := selectOption(&state, "standard"); message != "" {
		t.Fatal(message)
	}
	if state.LastQuote == nil || !state.LastQuote.Total.Equal(d("95000")) {
		t.Fatal("aynan shu paketning keyingi taklifi yo'qolmasligi kerak")
	}
}

func TestFasonCard_ResolvesMeasurementsWithoutModelArithmetic(t *testing.T) {
	fason := curtains.Fason{Parts: []curtains.FasonPart{
		{Role: curtains.RolePortyera, WidthShare: d("0.6"), HeightShare: d("1"), DefaultCoef: d("2")},
		{Role: curtains.RoleKant, WidthShare: d("0.4"), HeightShare: d("1"), DefaultCoef: d("2")},
	}}
	state := curtains.SessionState{CorniceLengthM: d("3"), HeightM: d("2.7")}
	card := fasonCardFrom(testCatalog().View(), fason, "living", "", state)
	if card.Parts[0].WidthM != "1.8" || card.Parts[1].WidthM != "1.2" || card.Parts[1].HeightM != "2.7" {
		t.Fatalf("tayyor bo'lak o'lchamlari: %+v", card.Parts)
	}
	if card.Parts[1].VariantID != "" {
		t.Fatal("kant topilmasa portyera bilan almashtirilmasligi kerak")
	}
}

func TestBuildOptions_BudgetComparisonUsesReturnedTotal(t *testing.T) {
	deps, _, state := optionsDeps()
	ctx := session.WithID(context.Background(), "budget-options")
	result, err := (&buildOptionsHandler{d: deps}).do(ctx, buildOptionsArgs{BudgetMax: 3500000})
	if err != nil || result.Error != "" {
		t.Fatalf("paketlar: %v %+v", err, result)
	}
	if result.Options[0].BudgetNote != "budjetga sig'adi" || result.Options[2].BudgetNote != "budjetdan 500 000 so'm yuqori" || !state.state.BudgetMax.Equal(d("3500000")) {
		t.Fatalf("budjet va tayyor hisob: %+v", result.Options)
	}
}

func TestOptionVariants_ExposeActualVariantColor(t *testing.T) {
	plan := curtains.OptionPlan{Parts: []curtains.PartSpec{{Role: curtains.RolePortyera, VariantID: vPortyera}}}
	variants := optionVariants(plan, testCatalog().View())
	if len(variants) != 1 || variants[0].Color != "grafit" {
		t.Fatalf("rang nomdan taxmin qilinmasligi kerak: %+v", variants)
	}
}
