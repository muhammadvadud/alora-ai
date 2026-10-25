package tools

import (
	"github.com/muhammadvadud/alora-ai/internal/curtains"
	"testing"
)

func TestBuildExplanation(t *testing.T) {
	var v *curtains.View = nil

	res := curtains.CalcResult{
		Measurements: []curtains.MeasurementResult{
			{
				Parts: []curtains.PartResult{
					{
						Role:          curtains.RolePortyera,
						SKU:           "Office Minimal",
						WidthM:        dec(2.5),
						HeightM:       dec(1.5),
						Coef:          dec(1.5),
						VisibleAreaM2: dec(5.625),
						WasteAreaM2:   dec(0.565),
						RawAreaM2:     dec(6.19),
						AreaM2:        dec(6.4),
						WastePercent:  dec(13.3),
						PriceUnit:     curtains.PerSqm,
						UnitPrice:     dec(45000),
						LineTotal:     dec(288000),
					},
				},
				Accessories: []curtains.AccessoryResult{
					{
						SKU:       "Montaj",
						Qty:       dec(1),
						PriceUnit: curtains.PerPiece,
						UnitPrice: dec(20000),
						LineTotal: dec(20000),
					},
				},
			},
		},
		Services:        []curtains.ServiceLine{},
		DeliveryAmount:  dec(30000),
		DiscountPercent: dec(5),
		DiscountAmount:  dec(18900),
		Total:           dec(338000),
	}

	lines := buildExplanation(v, res)
	if len(lines) == 0 {
		t.Fatal("Kutilgan explanation qatorlari chiqmadi")
	}

	expectedPart := "portyera Office Minimal: ko'rinadigan maydon: 2.5 m × K 1.5 × 1.5 m = 5.625 m²; chok/bukma zapasi 0.565 m²; zapasli maydon 6.19 m²; yaxlitlangan narxlanadigan maydon 6.4 m²; 6.4 m² × 45 000 so'm = 288 000 so'm"
	if lines[0] != expectedPart {
		t.Errorf("Kutilgan: %s\nHaqiqiy: %s", expectedPart, lines[0])
	}

	expectedAcc := "Bezak Montaj: 1 dona × 20 000 so'm = 20 000 so'm"
	if lines[1] != expectedAcc {
		t.Errorf("Kutilgan: %s\nHaqiqiy: %s", expectedAcc, lines[1])
	}

	expectedDel := "Yetkazish: 30 000 so'm"
	if lines[2] != expectedDel {
		t.Errorf("Kutilgan: %s\nHaqiqiy: %s", expectedDel, lines[2])
	}

	expectedDisc := "Chegirma 5%: -18 900 so'm"
	if lines[3] != expectedDisc {
		t.Errorf("Kutilgan: %s\nHaqiqiy: %s", expectedDisc, lines[3])
	}

	expectedTotal := "Jami: 338 000 so'm"
	if lines[4] != expectedTotal {
		t.Errorf("Kutilgan: %s\nHaqiqiy: %s", expectedTotal, lines[4])
	}
}
