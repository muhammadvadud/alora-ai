package curtains_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
	"github.com/shopspring/decimal"
)

func dec(s string) decimal.Decimal {
	return decimal.RequireFromString(s)
}

func decPtr(s string) *decimal.Decimal {
	d := decimal.RequireFromString(s)
	return &d
}

func TestValidate(t *testing.T) {
	mockLookup := func(variantID string) (curtains.Variant, bool) {
		switch variantID {
		case "v_fabric":
			return curtains.Variant{ID: "v_fabric", IsActive: true, PriceUnit: curtains.PerSqm}, true
		case "v_fabric_inactive":
			return curtains.Variant{ID: "v_fabric_inactive", IsActive: false, PriceUnit: curtains.PerSqm}, true
		case "v_acc_piece":
			return curtains.Variant{ID: "v_acc_piece", IsActive: true, PriceUnit: curtains.PerPiece}, true
		case "v_acc_meter":
			return curtains.Variant{ID: "v_acc_meter", IsActive: true, PriceUnit: curtains.PerMeter}, true
		case "v_acc_inactive":
			return curtains.Variant{ID: "v_acc_inactive", IsActive: false, PriceUnit: curtains.PerPiece}, true
		default:
			return curtains.Variant{}, false
		}
	}

	validPart := curtains.PartInput{
		VariantID: "v_fabric",
		Role:      curtains.RolePortyera,
		WidthM:    decPtr("3.0"),
		HeightM:   decPtr("2.8"),
		Coef:      decPtr("2.5"),
	}

	validAcc := curtains.AccessoryInput{
		VariantID: "v_acc_piece",
		Qty:       dec("10"),
	}

	validMeasurement := curtains.MeasurementInput{
		Length:      dec("3.5"),
		Height:      dec("2.8"),
		Parts:       []curtains.PartInput{validPart},
		Accessories: []curtains.AccessoryInput{validAcc},
	}

	validInput := curtains.CalcInput{
		Measurements:    []curtains.MeasurementInput{validMeasurement},
		DiscountPercent: dec("5.0"),
	}

	cases := []struct {
		name      string
		mutate    func(in *curtains.CalcInput)
		errField  string
		errString string
	}{
		{
			name:   "to'g'ri kirish o'tadi",
			mutate: func(in *curtains.CalcInput) {},
		},
		{
			name: "L=0.4",
			mutate: func(in *curtains.CalcInput) {
				in.Measurements[0].Length = dec("0.4")
			},
			errField: "cornice_length_m",
		},
		{
			name: "L=15.1",
			mutate: func(in *curtains.CalcInput) {
				in.Measurements[0].Length = dec("15.1")
			},
			errField: "cornice_length_m",
		},
		{
			name: "H=0.4",
			mutate: func(in *curtains.CalcInput) {
				in.Measurements[0].Height = dec("0.4")
			},
			errField: "height_m",
		},
		{
			name: "H=6.1",
			mutate: func(in *curtains.CalcInput) {
				in.Measurements[0].Height = dec("6.1")
			},
			errField: "height_m",
		},
		{
			name: "0 bo'lak",
			mutate: func(in *curtains.CalcInput) {
				in.Measurements[0].Parts = []curtains.PartInput{}
			},
			errField: "parts",
		},
		{
			name: "11 bo'lak",
			mutate: func(in *curtains.CalcInput) {
				var parts []curtains.PartInput
				for i := 0; i < 11; i++ {
					parts = append(parts, validPart)
				}
				in.Measurements[0].Parts = parts
			},
			errField: "parts",
		},
		{
			name: "K=0.9",
			mutate: func(in *curtains.CalcInput) {
				in.Measurements[0].Parts[0].Coef = decPtr("0.9")
			},
			errField: "parts[0].coefficient",
		},
		{
			name: "K=4.1",
			mutate: func(in *curtains.CalcInput) {
				in.Measurements[0].Parts[0].Coef = decPtr("4.1")
			},
			errField: "parts[0].coefficient",
		},
		{
			name: "qty=0",
			mutate: func(in *curtains.CalcInput) {
				in.Measurements[0].Accessories[0].Qty = dec("0")
			},
			errField: "accessories[0].qty",
		},
		{
			name: "noma'lum variant",
			mutate: func(in *curtains.CalcInput) {
				in.Measurements[0].Parts[0].VariantID = "v_unknown"
			},
			errField:  "parts[0].variant_id",
			errString: "variant topilmadi",
		},
		{
			name: "nofaol variant",
			mutate: func(in *curtains.CalcInput) {
				in.Measurements[0].Parts[0].VariantID = "v_fabric_inactive"
			},
			errField:  "parts[0].variant_id",
			errString: "variant faol emas",
		},
		{
			name: "aksessuar sifatida mato (PerSqm)",
			mutate: func(in *curtains.CalcInput) {
				in.Measurements[0].Accessories[0].VariantID = "v_fabric"
			},
			errField:  "accessories[0].variant_id",
			errString: "bu variant aksessuar emas",
		},
		{
			name: "mato sifatida aksessuar (PerPiece)",
			mutate: func(in *curtains.CalcInput) {
				in.Measurements[0].Parts[0].VariantID = "v_acc_piece"
			},
			errField:  "parts[0].variant_id",
			errString: "bu variant mato emas",
		},
		{
			name: "width > L",
			mutate: func(in *curtains.CalcInput) {
				in.Measurements[0].Parts[0].WidthM = decPtr("3.6")
			},
			errField: "parts[0].width_m",
		},
		{
			name: "discount 101",
			mutate: func(in *curtains.CalcInput) {
				in.DiscountPercent = dec("101")
			},
			errField: "discount_percent",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := cloneInput(validInput)
			tc.mutate(&in)

			err := curtains.Validate(in, mockLookup)
			if tc.errField == "" {
				if err != nil {
					t.Fatalf("kutilmagan xato: %v", err)
				}
			} else {
				if err == nil {
					t.Fatalf("xato kutilgandi, lekin nil qaytdi")
				}
				var ve *curtains.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("xato turi ValidationError emas: %T", err)
				}

				if ve.Field != tc.errField {
					t.Errorf("kutilgan maydon %q, haqiqiy xato maydoni: %q (msg: %q)", tc.errField, ve.Field, ve.Msg)
				}

				if tc.errString != "" && !strings.Contains(ve.Msg, tc.errString) {
					t.Errorf("xatoda %q kutilgandi, haqiqiy xato xabari: %q", tc.errString, ve.Msg)
				}
			}
		})
	}
}

func cloneInput(in curtains.CalcInput) curtains.CalcInput {
	out := in
	if in.Measurements != nil {
		out.Measurements = make([]curtains.MeasurementInput, len(in.Measurements))
		for i, m := range in.Measurements {
			outM := m
			if m.Parts != nil {
				outM.Parts = make([]curtains.PartInput, len(m.Parts))
				copy(outM.Parts, m.Parts)
			}
			if m.Accessories != nil {
				outM.Accessories = make([]curtains.AccessoryInput, len(m.Accessories))
				copy(outM.Accessories, m.Accessories)
			}
			out.Measurements[i] = outM
		}
	}
	return out
}
