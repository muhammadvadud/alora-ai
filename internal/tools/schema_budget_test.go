package tools

import (
	"context"
	"encoding/json"
	"testing"
)

// TestToolSchemaBudget — har tool'ning modelga ketadigan sxemasi (desc + JSON schema)
// chegarada bo'lsin: 2026-09-15 o'lchovda 6 tool ≈ 2.9k token — bir chaqiruvning 60%i.
// Enum ro'yxatlari zarur, tavsiflar qisqa bo'lishi kerak; oshsa — qisqartiriladi.
//
// ADR-020: `curtain_calculate` uchun alohida byudjet. Uning sxemasi `windows[]`
// bilan kengaydi (A1–A4: bir nechta oyna, har biriga alohida o'lcham va bo'laklar).
// `calcPart`/`calcAccessory` sxemalari top-levelda ham, `windows[]` ichida ham
// takrorlanadi — bitta qo'shimcha maydon 400+ belgi qo'shadi. Buni qisqartirish
// model uchun foydali ma'lumotni olib tashlashni talab qilardi, shuning uchun
// byudjet oshirildi. JAMI byudjet esa o'zgarmaydi — umumiy token o'sishi
// nazorat ostida.
func TestToolSchemaBudget(t *testing.T) {
	const (
		budget       = 1700
		totalBudget  = 11000
		calcExpanded = 3400 // ADR-020: windows[] bilan kengaygan sxema
	)
	perTool := map[string]int{"curtain_calculate": calcExpanded}

	all := append(Alora(Deps{}), AloraLookups(Deps{})...)
	total := 0
	for _, tl := range all {
		info, err := tl.Info(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		js, err := info.ToJSONSchema()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(js)
		n := len(info.Desc) + len(b)
		total += n
		limit := budget
		if custom, ok := perTool[info.Name]; ok {
			limit = custom
		}
		t.Logf("%-18s desc=%4d schema=%5d jami=%5d byudjet=%d", info.Name, len(info.Desc), len(b), n, limit)
		if n > limit {
			t.Errorf("%s: %d belgi > %d byudjet", info.Name, n, limit)
		}
	}
	t.Logf("jami=%d belgi ≈ %d token (byudjet %d)", total, total/3, totalBudget)
	if total > totalBudget {
		t.Errorf("jami sxema: %d belgi > %d byudjet — tavsiflarni qisqartiring", total, totalBudget)
	}
}
