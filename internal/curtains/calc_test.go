package curtains_test

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"

	"github.com/muhammadvadud/alora-agent/internal/curtains"
	"github.com/muhammadvadud/alora-agent/internal/curtains/curtainstest"
)

// TestCalc_ReferenceCases — backend bilan UMUMIY 20 ta etalon (rules.md §0.3, §14.1).
// Har son Decimal sifatida solishtiriladi (Python Decimal "18.750000000" va Go "18.75"
// bir xil qiymat); warnings — matn sifatida.
func TestCalc_ReferenceCases(t *testing.T) {
	fx, src, err := curtainstest.Load(curtainstest.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	if fx.SchemaVersion != "alora.pricing.calc_cases.v1" || len(fx.Cases) != 20 {
		t.Fatalf("fixture: schema=%s, cases=%d (kutilgan v1, 20)", fx.SchemaVersion, len(fx.Cases))
	}
	for _, c := range fx.Cases {
		t.Run(c.Name, func(t *testing.T) {
			got, err := curtains.Calculate(src, c.Input.ToCalcInput(curtains.RoleManager))
			if err != nil {
				t.Fatalf("Calculate: %v", err)
			}
			gotJSON, _ := json.Marshal(got)
			var gotAny, wantAny any
			_ = json.Unmarshal(gotJSON, &gotAny)
			_ = json.Unmarshal(c.Expected, &wantAny)
			compare(t, "", wantAny, gotAny)
		})
	}
}

// compare — JSON daraxtini rekursiv solishtiradi; sonli string'lar Decimal bo'yicha teng bo'lsa o'tadi.
func compare(t *testing.T, path string, want, got any) {
	t.Helper()
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			t.Errorf("%s: obyekt kutilgan, %T keldi", path, got)
			return
		}
		for k, wv := range w {
			gv, ok := g[k]
			if !ok {
				t.Errorf("%s.%s: maydon yo'q", path, k)
				continue
			}
			compare(t, path+"."+k, wv, gv)
		}
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			t.Errorf("%s: ro'yxat uzunligi want=%d got=%v", path, len(w), got)
			return
		}
		for i := range w {
			compare(t, path+"["+itoa(i)+"]", w[i], g[i])
		}
	case string:
		gs, ok := got.(string)
		if !ok {
			t.Errorf("%s: want %q got %v", path, w, got)
			return
		}
		wd, werr := decimal.NewFromString(w)
		gd, gerr := decimal.NewFromString(gs)
		if werr == nil && gerr == nil {
			if !wd.Equal(gd) {
				t.Errorf("%s: want %s got %s", path, w, gs)
			}
			return
		}
		if w != gs {
			t.Errorf("%s: want %q got %q", path, w, gs)
		}
	default:
		if want != got {
			t.Errorf("%s: want %v got %v", path, want, got)
		}
	}
}

func itoa(i int) string { return decimal.NewFromInt(int64(i)).String() }

func TestCalc_MissingRuleIsValidationError(t *testing.T) {
	_, src, _ := curtainstest.Load(curtainstest.DefaultPath())
	broken := ruleless{src}
	_, err := curtains.Calculate(broken, curtains.CalcInput{Measurements: []curtains.MeasurementInput{{
		Length: decimal.NewFromInt(3), Height: decimal.NewFromInt(2),
		Parts: []curtains.PartInput{{VariantID: "11111111-1111-4111-8111-111111111111", Role: curtains.RolePortyera}},
	}}})
	if !curtains.IsValidation(err) {
		t.Fatalf("ValidationError kutilgan, keldi: %v", err)
	}
}

func TestCalc_UnknownVariant(t *testing.T) {
	_, src, _ := curtainstest.Load(curtainstest.DefaultPath())
	_, err := curtains.Calculate(src, curtains.CalcInput{Measurements: []curtains.MeasurementInput{{
		Length: decimal.NewFromInt(3), Height: decimal.NewFromInt(2),
		Parts: []curtains.PartInput{{VariantID: "yo'q", Role: curtains.RolePortyera}},
	}}})
	if !curtains.IsValidation(err) || err.Error() != "variant_id: Variant topilmadi" {
		t.Fatalf("kutilmagan xato: %v", err)
	}
}

// ruleless — coef_default qoidasini yashiradi (kalit topilmasa default o'ylab topilmasligini tekshirish uchun).
type ruleless struct{ curtains.PricingSource }

func (r ruleless) CalcRule(key string) (decimal.Decimal, bool) {
	if key == curtains.RuleCoefDefault {
		return decimal.Zero, false
	}
	return r.PricingSource.CalcRule(key)
}
