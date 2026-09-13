package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

// quoteWithCalc — avval curtain_calculate natijasini state'ga yozadi, keyin
// price_quote chaqiradi (price_quote calc'siz error qaytaradi — rules §6.6).
func quoteWithCalc(t *testing.T, ctx context.Context, args priceQuoteArgs) priceQuoteResult {
	t.Helper()
	dp, _, _ := deps(t)
	_, _ = (&curtainCalcHandler{d: dp}).do(ctx, curtainCalcArgs{CorniceLengthM: 4.2, HeightM: 2.6,
		Parts: []calcPart{{VariantID: vPortyera, Role: "portyera"}}})
	res, err := (&priceQuoteHandler{d: dp}).do(ctx, args)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	return res
}

// chegirma rol chegarasi fail-closed bo'lishi shart (rules §5.6): noma'lum
// yoki bo'sh rol manager chegarasiga tushib qolmasligi kerak (avvalgi teshik:
// userRole noma'lum rolni uzatib yuborar, price_quote šu holda manager limitini
// qo'llardi).
func TestPriceQuote_UnknownRole_FailClosed(t *testing.T) {
	for _, role := range []string{"", "admin", "MANAGER", "Employee", "director"} {
		ctx, _ := ctxFor(curtains.UserRole(role))
		res := quoteWithCalc(t, ctx, priceQuoteArgs{DiscountPct: 5})
		if !strings.Contains(res.Error, "foydalanuvchi roli noma'lum") {
			t.Errorf("rol %q: fail-closed xato kutilgan, olindi %q", role, res.Error)
		}
	}
}

// employee/manager chegaralari o'z qiymatida tekshiriladi; role katalogidagi
// policy: employee max 5%, manager max 15% (testCatalog).
func TestPriceQuote_RoleLimits(t *testing.T) {
	cases := []struct {
		role     curtains.UserRole
		discount float64
		wantErr  string
	}{
		{role: curtains.RoleEmployee, discount: 5, wantErr: ""},
		{role: curtains.RoleEmployee, discount: 12, wantErr: "chegirma chegarasi 5% — 12%"},
		{role: curtains.RoleManager, discount: 12, wantErr: ""},
		{role: curtains.RoleManager, discount: 16, wantErr: "chegirma chegarasi 15% — 16%"},
		{role: "", discount: 5, wantErr: "foydalanuvchi roli noma'lum"},
		{role: "admin", discount: 5, wantErr: "foydalanuvchi roli noma'lum"},
		{role: "MANAGER", discount: 5, wantErr: "foydalanuvchi roli noma'lum"},
	}
	for _, tc := range cases {
		ctx, _ := ctxFor(tc.role)
		res := quoteWithCalc(t, ctx, priceQuoteArgs{DiscountPct: tc.discount})
		switch {
		case tc.wantErr == "" && res.Error != "":
			t.Errorf("rol %q, chegirma %v%%: kutilmagan xato: %q", tc.role, tc.discount, res.Error)
		case tc.wantErr != "" && !strings.Contains(res.Error, tc.wantErr):
			t.Errorf("rol %q, chegirma %v%%: %q tarkibida %q yo'q", tc.role, tc.discount, res.Error, tc.wantErr)
		}
	}
}
