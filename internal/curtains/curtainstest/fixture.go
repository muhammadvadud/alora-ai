// Package curtainstest — testlar uchun yordamchilar: backend bilan umumiy
// calc_cases.json etalonidan PricingSource va holatlar. Faqat _test.go dan ishlatiladi
// (rules.md §4.1.8: katalog faqat Django'dan to'ladi; bu — test istisnosi).
package curtainstest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/shopspring/decimal"

	"github.com/muhammadvadud/alora-agent/internal/curtains"
)

// Fixture — calc_cases.json ning to'liq mazmuni.
type Fixture struct {
	SchemaVersion string `json:"schema_version"`
	PricingSeed   struct {
		CalcRules []struct {
			Key         string          `json:"key"`
			Value       decimal.Decimal `json:"value"`
			Unit        string          `json:"unit"`
			Description string          `json:"description"`
		} `json:"calc_rules"`
		ServicePrices []struct {
			Key      string          `json:"key"`
			Name     string          `json:"name"`
			Price    decimal.Decimal `json:"price"`
			Unit     string          `json:"unit"`
			IsActive bool            `json:"is_active"`
		} `json:"service_prices"`
		DiscountTiers []struct {
			MinAmount decimal.Decimal `json:"min_amount"`
			Percent   decimal.Decimal `json:"percent"`
			IsActive  bool            `json:"is_active"`
		} `json:"discount_tiers"`
		DiscountPolicy struct {
			ManualMaxPercentEmployee decimal.Decimal `json:"manual_max_percent_employee"`
			ManualMaxPercentManager  decimal.Decimal `json:"manual_max_percent_manager"`
			StackWithTier            bool            `json:"stack_with_tier"`
		} `json:"discount_policy"`
		OrderDefaults struct {
			PrepaymentPercent decimal.Decimal `json:"prepayment_percent"`
			DeliveryAmount    decimal.Decimal `json:"delivery_amount"`
			DeadlineDays      int             `json:"deadline_days"`
		} `json:"order_defaults"`
	} `json:"pricing_seed"`
	CatalogSeed struct {
		Variants []struct {
			ID    string          `json:"id"`
			SKU   string          `json:"sku"`
			Price decimal.Decimal `json:"price"`
			Unit  string          `json:"unit"`
		} `json:"variants"`
	} `json:"catalog_seed"`
	Cases []Case `json:"cases"`
}

// Case — bitta etalon holat: kirish va Django kutgan natija (xom JSON).
type Case struct {
	Name     string          `json:"name"`
	Input    CaseInput       `json:"input"`
	Expected json.RawMessage `json:"expected"`
}

type CaseInput struct {
	Measurements []struct {
		Length decimal.Decimal `json:"length"`
		Height decimal.Decimal `json:"height"`
		Parts  []struct {
			VariantID string           `json:"variant_id"`
			Role      string           `json:"role"`
			WidthM    *decimal.Decimal `json:"width_m"`
			HeightM   *decimal.Decimal `json:"height_m"`
			Coef      *decimal.Decimal `json:"coef"`
		} `json:"parts"`
		Accessories []struct {
			VariantID string          `json:"variant_id"`
			Qty       decimal.Decimal `json:"qty"`
		} `json:"accessories"`
	} `json:"measurements"`
	Services        []string         `json:"services"`
	Delivery        bool             `json:"delivery"`
	DiscountPercent *decimal.Decimal `json:"discount_percent"`
}

// ToCalcInput — fixture kirishini domen kirishiga aylantiradi.
func (c CaseInput) ToCalcInput(role curtains.UserRole) curtains.CalcInput {
	in := curtains.CalcInput{Services: c.Services, Delivery: c.Delivery, Role: role}
	if c.Services == nil {
		in.Services = []string{}
	}
	if c.DiscountPercent != nil {
		in.DiscountPercent = *c.DiscountPercent
	}
	for _, m := range c.Measurements {
		mi := curtains.MeasurementInput{Length: m.Length, Height: m.Height}
		for _, p := range m.Parts {
			mi.Parts = append(mi.Parts, curtains.PartInput{
				VariantID: p.VariantID, Role: curtains.PartRole(p.Role),
				WidthM: p.WidthM, HeightM: p.HeightM, Coef: p.Coef,
			})
		}
		for _, a := range m.Accessories {
			mi.Accessories = append(mi.Accessories, curtains.AccessoryInput{VariantID: a.VariantID, Qty: a.Qty})
		}
		in.Measurements = append(in.Measurements, mi)
	}
	return in
}

// Source — fixture seed'lari asosidagi PricingSource.
type Source struct {
	variants map[string]curtains.Variant
	rules    map[string]decimal.Decimal
	services map[string]curtains.Service
	tiers    []curtains.DiscountTier
	policy   curtains.DiscountPolicy
	defaults curtains.OrderDefaults
}

func (s *Source) Variant(id string) (curtains.Variant, bool)      { v, ok := s.variants[id]; return v, ok }
func (s *Source) CalcRule(key string) (decimal.Decimal, bool)     { v, ok := s.rules[key]; return v, ok }
func (s *Source) Service(key string) (curtains.Service, bool)     { v, ok := s.services[key]; return v, ok }
func (s *Source) DiscountTiers() []curtains.DiscountTier          { return s.tiers }
func (s *Source) DiscountPolicy() (curtains.DiscountPolicy, bool) { return s.policy, true }
func (s *Source) OrderDefaults() (curtains.OrderDefaults, bool)   { return s.defaults, true }

// DefaultPath — repo ildizidagi tests/fixtures/calc_cases.json (shu fayl joylashuviga nisbatan).
func DefaultPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "tests", "fixtures", "calc_cases.json")
}

// Load — fixture faylini o'qiydi va PricingSource yasaydi.
func Load(path string) (*Fixture, *Source, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("fixture o'qish: %w", err)
	}
	var f Fixture
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, nil, fmt.Errorf("fixture parse: %w", err)
	}
	src := &Source{
		variants: map[string]curtains.Variant{},
		rules:    map[string]decimal.Decimal{},
		services: map[string]curtains.Service{},
	}
	for _, v := range f.CatalogSeed.Variants {
		src.variants[v.ID] = curtains.Variant{
			ID: v.ID, SKU: v.SKU, Price: v.Price, PriceUnit: curtains.PriceUnit(v.Unit), IsActive: true,
		}
	}
	for _, r := range f.PricingSeed.CalcRules {
		src.rules[r.Key] = r.Value
	}
	for _, s := range f.PricingSeed.ServicePrices {
		src.services[s.Key] = curtains.Service{
			Key: s.Key, Name: s.Name, Price: s.Price, Unit: curtains.PriceUnit(s.Unit), IsActive: s.IsActive,
		}
	}
	for i, t := range f.PricingSeed.DiscountTiers {
		src.tiers = append(src.tiers, curtains.DiscountTier{
			ID: fmt.Sprintf("tier-%d", i), MinAmount: t.MinAmount, Percent: t.Percent, IsActive: t.IsActive,
		})
	}
	p := f.PricingSeed.DiscountPolicy
	src.policy = curtains.DiscountPolicy{
		ManualMaxPercentEmployee: p.ManualMaxPercentEmployee,
		ManualMaxPercentManager:  p.ManualMaxPercentManager,
		StackWithTier:            p.StackWithTier,
	}
	d := f.PricingSeed.OrderDefaults
	src.defaults = curtains.OrderDefaults{
		PrepaymentPercent: d.PrepaymentPercent, DeliveryAmount: d.DeliveryAmount, DeadlineDays: d.DeadlineDays,
	}
	return &f, src, nil
}
