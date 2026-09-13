package curtains

import (
	"context"
	"encoding/json"

	"github.com/shopspring/decimal"
)

// Calculator hisob formulasini Go'da takrorlamaslik uchun Django preview xizmatiga
// delegatsiya qilinadi (ADR-009); domen faqat shu shartnomani e'lon qiladi.
type Calculator interface {
	Calculate(ctx context.Context, in CalcInput) (CalcResult, error)
}

type calcPayload struct {
	Measurements []measurementPayload `json:"measurements"`
	Services     []string             `json:"services"`
	Delivery     bool                 `json:"delivery,omitempty"`
	Discount     string               `json:"discount_percent,omitempty"`
}

type measurementPayload struct {
	Length      string             `json:"length"`
	Height      string             `json:"height"`
	Parts       []partPayload      `json:"parts"`
	Accessories []accessoryPayload `json:"accessories"`
}

type partPayload struct {
	VariantID string   `json:"variant_id"`
	Role      PartRole `json:"role"`
	WidthM    *string  `json:"width_m,omitempty"`
	HeightM   *string  `json:"height_m,omitempty"`
	Coef      *string  `json:"coef,omitempty"`
}

type accessoryPayload struct {
	VariantID string `json:"variant_id"`
	Qty       string `json:"qty"`
}

// MarshalJSON Django payload'ini decimal qiymatlarni matn va nil o'lchovlarni
// omitsiz shaklga aylantiradi; services bo'sh bo'lsa ham [] bo'lib qoladi.
func (in CalcInput) MarshalJSON() ([]byte, error) {
	p := calcPayload{Measurements: make([]measurementPayload, 0, len(in.Measurements)), Services: in.Services}
	if p.Services == nil {
		p.Services = []string{}
	}
	if in.Delivery {
		p.Delivery = true
	}
	if !in.DiscountPercent.IsZero() {
		p.Discount = decimalText(in.DiscountPercent)
	}
	for _, m := range in.Measurements {
		mp := measurementPayload{
			Length:      decimalText(m.Length),
			Height:      decimalText(m.Height),
			Parts:       make([]partPayload, 0, len(m.Parts)),
			Accessories: make([]accessoryPayload, 0, len(m.Accessories)),
		}
		for _, part := range m.Parts {
			pp := partPayload{VariantID: part.VariantID, Role: part.Role}
			if part.WidthM != nil {
				v := decimalText(*part.WidthM)
				pp.WidthM = &v
			}
			if part.HeightM != nil {
				v := decimalText(*part.HeightM)
				pp.HeightM = &v
			}
			if part.Coef != nil {
				v := decimalText(*part.Coef)
				pp.Coef = &v
			}
			mp.Parts = append(mp.Parts, pp)
		}
		for _, accessory := range m.Accessories {
			mp.Accessories = append(mp.Accessories, accessoryPayload{
				VariantID: accessory.VariantID,
				Qty:       decimalText(accessory.Qty),
			})
		}
		p.Measurements = append(p.Measurements, mp)
	}
	return json.Marshal(p)
}

func decimalText(value decimal.Decimal) string {
	if exponent := value.Exponent(); exponent < 0 {
		return value.StringFixed(-exponent)
	}
	return value.String()
}
