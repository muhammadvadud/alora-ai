package curtains

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

// ValidationError — foydalanuvchi/LLM kiritmasi xatosi. Tool adapter uni
// {"error": "..."} natijaga aylantiradi (errors-as-results), Go error emas.
type ValidationError struct {
	Field string
	Msg   string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Msg }

// IsValidation — xato ValidationError ekanini tekshiradi (o'ralgan bo'lsa ham).
func IsValidation(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

func validationf(field, format string, args ...any) error {
	return &ValidationError{Field: field, Msg: fmt.Sprintf(format, args...)}
}

// PricingSource — hisob uchun kerakli katalog ma'lumotlari. Catalog.View() buni
// amalga oshiradi; testlarda fixture (curtainstest) beradi. Hisob shu interfeys
// orqali ishlaydi, shuning uchun u cache tuzilmasidan mustaqil.
type PricingSource interface {
	Variant(id string) (Variant, bool)
	CalcRule(key string) (decimal.Decimal, bool)
	Service(key string) (Service, bool)
	DiscountTiers() []DiscountTier
	DiscountPolicy() (DiscountPolicy, bool)
	OrderDefaults() (OrderDefaults, bool)
}

// ── Kirish ──────────────────────────────────────────────────────────────────

// PartInput — bitta mato bo'lagi. WidthM/HeightM nil bo'lsa o'lchovdan olinadi,
// Coef nil bo'lsa CalcRule coef_default (Django `_part_width/_part_height/_part_coef`).
type PartInput struct {
	VariantID string
	Role      PartRole
	WidthM    *decimal.Decimal
	HeightM   *decimal.Decimal
	Coef      *decimal.Decimal
}

// AccessoryInput — bezak/furnitura: dona yoki metr (variantning price_unit'iga qarab).
type AccessoryInput struct {
	VariantID string
	Qty       decimal.Decimal
}

// MeasurementInput — bitta deraza/o'lchov.
type MeasurementInput struct {
	Length      decimal.Decimal
	Height      decimal.Decimal
	Parts       []PartInput
	Accessories []AccessoryInput
}

// CalcInput — Django `calculate_preview(payload, user_role)` payload'i bilan bir xil.
type CalcInput struct {
	Measurements    []MeasurementInput
	Services        []string
	Delivery        bool
	DiscountPercent decimal.Decimal
	Role            UserRole
}

// ── Chiqish (Django preview JSON kalitlari bilan bir xil) ──────────────────

type PartResult struct {
	VariantID     string          `json:"variant_id"`
	SKU           string          `json:"sku"`
	Role          PartRole        `json:"role"`
	WidthM        decimal.Decimal `json:"width_m"`
	HeightM       decimal.Decimal `json:"height_m"`
	Coef          decimal.Decimal `json:"coef"`
	VisibleAreaM2 decimal.Decimal `json:"visible_area_m2"`
	RawAreaM2     decimal.Decimal `json:"raw_area_m2"`
	AreaM2        decimal.Decimal `json:"area_m2"`
	WasteAreaM2   decimal.Decimal `json:"waste_area_m2"`
	WastePercent  decimal.Decimal `json:"waste_percent"`
	PriceUnit     PriceUnit       `json:"price_unit"`
	UnitPrice     decimal.Decimal `json:"unit_price"`
	LineTotal     decimal.Decimal `json:"line_total"`
}

type AccessoryResult struct {
	VariantID string          `json:"variant_id"`
	SKU       string          `json:"sku"`
	Qty       decimal.Decimal `json:"qty"`
	PriceUnit PriceUnit       `json:"price_unit"`
	UnitPrice decimal.Decimal `json:"unit_price"`
	LineTotal decimal.Decimal `json:"line_total"`
}

type MeasurementResult struct {
	Length           decimal.Decimal   `json:"length"`
	Height           decimal.Decimal   `json:"height"`
	Parts            []PartResult      `json:"parts"`
	Accessories      []AccessoryResult `json:"accessories"`
	RawAreaM2        decimal.Decimal   `json:"raw_area_m2"`
	AreaM2           decimal.Decimal   `json:"area_m2"`
	AccessoriesTotal decimal.Decimal   `json:"accessories_total"`
}

type ServiceLine struct {
	Key        string          `json:"key"`
	Name       string          `json:"name"`
	Qty        decimal.Decimal `json:"qty"`
	PriceUnit  PriceUnit       `json:"price_unit"`
	UnitPrice  decimal.Decimal `json:"unit_price"`
	LineTotal  decimal.Decimal `json:"line_total"`
	IsDelivery bool            `json:"is_delivery"`
}

// CalcResult — to'liq hisob. Sonlar JSON'da string (decimal.MarshalJSON) — aniqlik yo'qolmaydi.
type CalcResult struct {
	Measurements          []MeasurementResult `json:"measurements"`
	Services              []ServiceLine       `json:"services"`
	Subtotal              decimal.Decimal     `json:"subtotal"`
	TierDiscountPercent   decimal.Decimal     `json:"tier_discount_percent"`
	ManualDiscountPercent decimal.Decimal     `json:"manual_discount_percent"`
	DiscountPercent       decimal.Decimal     `json:"discount_percent"`
	DiscountAmount        decimal.Decimal     `json:"discount_amount"`
	DeliveryAmount        decimal.Decimal     `json:"delivery_amount"`
	Total                 decimal.Decimal     `json:"total"`
	Warnings              []string            `json:"warnings"`
}
