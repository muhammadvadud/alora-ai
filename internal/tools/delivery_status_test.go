package tools

import (
	"testing"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

func TestDisplayTotals_DeliveryStatus(t *testing.T) {
	cases := []struct {
		name   string
		result curtains.CalcResult
		status string
	}{
		{"excluded", curtains.CalcResult{}, "yetkazish hisobga kiritilmagan"},
		{"paid", curtains.CalcResult{DeliveryAmount: d("30000")}, "yetkazish hisobga kiritilgan"},
		{"included-zero", curtains.CalcResult{Services: []curtains.ServiceLine{{Key: "delivery", IsDelivery: true}}}, "yetkazish hisobga kiritilgan, ushbu hisobdagi narxi nol"},
		{"other-zero-service", curtains.CalcResult{Services: []curtains.ServiceLine{{Key: "sewing"}}}, "yetkazish hisobga kiritilmagan"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			display := displayTotals(tc.result)
			if display["delivery_status"] != tc.status || display["delivery"] != displayMoney(tc.result.DeliveryAmount) {
				t.Fatalf("yetkazish summasi va holati: %+v", display)
			}
		})
	}
}
