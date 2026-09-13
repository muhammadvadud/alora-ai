package curtains

import (
	"fmt"

	"github.com/shopspring/decimal"
)

var (
	// MinLengthM karnizning minimal uzunligi (metr)
	MinLengthM = decimal.RequireFromString("0.5")
	// MaxLengthM karnizning maksimal uzunligi (metr)
	MaxLengthM = decimal.RequireFromString("15")
	// MinHeightM pardaning minimal balandligi (metr)
	MinHeightM = decimal.RequireFromString("0.5")
	// MaxHeightM pardaning maksimal balandligi (metr)
	MaxHeightM = decimal.RequireFromString("6")
	// MinCoef minimal yig'ish koeffitsienti
	MinCoef = decimal.RequireFromString("1.0")
	// MaxCoef maksimal yig'ish koeffitsienti
	MaxCoef = decimal.RequireFromString("4.0")
)

const (
	// MaxParts bitta derazadagi bo'laklar maksimal soni
	MaxParts = 10
)

// Validate hisob-kitob uchun kiritilgan ma'lumotlarning to'g'riligini tekshiradi.
// Qoidalar rules.md §4.2.3 va ADR-010 ga asosan amalga oshirilgan.
func Validate(in CalcInput, lookup func(variantID string) (Variant, bool)) error {
	if in.DiscountPercent.LessThan(decimal.Zero) || in.DiscountPercent.GreaterThan(decimal.RequireFromString("100")) {
		return validationf("discount_percent", "chegirma foizi 0–100 oralig'ida bo'lishi kerak, kiritildi: %s", in.DiscountPercent.String())
	}

	for _, m := range in.Measurements {
		if m.Length.LessThan(MinLengthM) || m.Length.GreaterThan(MaxLengthM) {
			return validationf("cornice_length_m", "karniz uzunligi 0.5–15 m oralig'ida bo'lishi kerak, kiritildi: %s", m.Length.String())
		}
		if m.Height.LessThan(MinHeightM) || m.Height.GreaterThan(MaxHeightM) {
			return validationf("height_m", "balandlik 0.5–6 m oralig'ida bo'lishi kerak, kiritildi: %s", m.Height.String())
		}

		if len(m.Parts) < 1 || len(m.Parts) > MaxParts {
			return validationf("parts", "bo'laklar soni 1–10 ta bo'lishi kerak, kiritildi: %d", len(m.Parts))
		}

		lengthWithMargin := m.Length.Add(decimal.RequireFromString("0.01"))
		heightWithMargin := m.Height.Add(decimal.RequireFromString("0.01"))

		for i, part := range m.Parts {
			prefix := fmt.Sprintf("parts[%d]", i)

			if part.VariantID == "" {
				return validationf(prefix+".variant_id", "variant ko'rsatilmagan")
			}
			variant, ok := lookup(part.VariantID)
			if !ok {
				return validationf(prefix+".variant_id", "variant topilmadi")
			}
			if !variant.IsActive {
				return validationf(prefix+".variant_id", "variant faol emas")
			}
			if variant.PriceUnit != PerSqm {
				return validationf(prefix+".variant_id", "bu variant mato emas")
			}

			if !isValidRole(part.Role) {
				return validationf(prefix+".role", "noma'lum bo'lak roli, kiritildi: %s", part.Role)
			}

			if part.WidthM != nil {
				if part.WidthM.LessThanOrEqual(decimal.Zero) || part.WidthM.GreaterThan(lengthWithMargin) {
					return validationf(prefix+".width_m", "kenglik noldan katta va karniz uzunligidan oshmasligi kerak, kiritildi: %s", part.WidthM.String())
				}
			}

			if part.HeightM != nil {
				if part.HeightM.LessThanOrEqual(decimal.Zero) || part.HeightM.GreaterThan(heightWithMargin) {
					return validationf(prefix+".height_m", "balandlik noldan katta va karniz balandligidan oshmasligi kerak, kiritildi: %s", part.HeightM.String())
				}
			}

			if part.Coef != nil {
				if part.Coef.LessThan(MinCoef) || part.Coef.GreaterThan(MaxCoef) {
					return validationf(prefix+".coefficient", "koeffitsient 1.0–4.0 oralig'ida bo'lishi kerak, kiritildi: %s", part.Coef.String())
				}
			}
		}

		for i, acc := range m.Accessories {
			prefix := fmt.Sprintf("accessories[%d]", i)

			if acc.Qty.LessThanOrEqual(decimal.Zero) {
				return validationf(prefix+".qty", "miqdor noldan katta bo'lishi kerak, kiritildi: %s", acc.Qty.String())
			}

			if acc.VariantID == "" {
				return validationf(prefix+".variant_id", "variant ko'rsatilmagan")
			}
			variant, ok := lookup(acc.VariantID)
			if !ok {
				return validationf(prefix+".variant_id", "variant topilmadi")
			}
			if !variant.IsActive {
				return validationf(prefix+".variant_id", "variant faol emas")
			}
			if variant.PriceUnit != PerPiece && variant.PriceUnit != PerMeter {
				return validationf(prefix+".variant_id", "bu variant aksessuar emas")
			}
		}
	}

	return nil
}

func isValidRole(r PartRole) bool {
	switch r {
	case RolePortyera, RoleTul, RoleLambrekan, RoleKant, RoleDekor, RoleAstar:
		return true
	}
	return false
}
