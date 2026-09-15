package tools

import (
	"context"
	"encoding/json"
	"testing"
)

// TestToolSchemaBudget — har tool'ning modelga ketadigan sxemasi (desc + JSON schema)
// chegarada bo'lsin: 2026-09-15 o'lchovda 6 tool ≈ 2.9k token — bir chaqiruvning 60%i.
// Byudjet: ≤ 1700 belgi (~570 token) har tool uchun — enum ro'yxatlari zarur, tavsiflar qisqa. Oshsa — tavsif/enum'larni qisqartir.
func TestToolSchemaBudget(t *testing.T) {
	const budget = 1700
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
		t.Logf("%-18s desc=%4d schema=%5d jami=%5d", info.Name, len(info.Desc), len(b), n)
		if n > budget {
			t.Errorf("%s: %d belgi > %d byudjet", info.Name, n, budget)
		}
	}
	t.Logf("jami=%d belgi ≈ %d token", total, total/3)
}
