package tools

import (
	"encoding/json"
	"testing"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

// maxResultBytes — tool natijasi model kontekstiga ko'p token olib ketmasligi
// uchun qat'iy chek (rules §5: natija ≤ 2 KB). 10 bo'lakli hisob — eng og'ir hol.
const maxResultBytes = 2048

func TestToolResults_10PartsFit2KB(t *testing.T) {
	dp, _, _ := deps(t)
	ctx, _ := ctxFor(curtains.RoleEmployee)

	parts := make([]calcPart, 10)
	for i := range parts {
		parts[i] = calcPart{VariantID: vPortyera, Role: "portyera", Coefficient: 2.5}
	}
	calc, _ := (&curtainCalcHandler{d: dp}).do(ctx, curtainCalcArgs{
		CorniceLengthM: 4, HeightM: 2.6, RoomName: "Mehmonxona", Parts: parts,
	})
	checkResultSize(t, "curtain_calculate", calc)

	quote, _ := (&priceQuoteHandler{d: dp}).do(ctx, priceQuoteArgs{DiscountPct: 5, NeedDelivery: true})
	checkResultSize(t, "price_quote", quote)
}

func checkResultSize(t *testing.T, name string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("%s: marshal: %v", name, err)
	}
	t.Logf("%s: %d B", name, len(b))
	if len(b) > maxResultBytes {
		t.Fatalf("%s natijasi %d B — limit %d B; qisqartirish kerak:\n%s", name, len(b), maxResultBytes, b)
	}
}
