package curtains

import (
	"strings"
	"testing"
)

// A1 (ADR-014): rulon/rim AI orqali hisoblanmaydi. Tekshiruv ROLY emas, qatlamga
// qaraydi — 2026-09-28 live'da model rolyni rad etib, "portyera" roli bilan
// ~2.5x noto'g'ri narxni hisoblagan edi.
//
// Qatlam LayerOf orqali aniqlanadi: teg > nom/kategoriya/matot turi evristikasi.
func TestCalcSupportLayer_RulonRimUnsupportedByLayerNotRole(t *testing.T) {
	v := searchView(Snapshot{
		Version: 1,
		Products: []Product{
			{ID: "p-rulon", Name: "Rolon Blackout", Tags: map[string]any{"rulon": true}, VariantIDs: []string{"v-rulon-1"}},
			{ID: "p-rim", Name: "Carmel Kitchen", Tags: map[string]any{"rim": true}, VariantIDs: []string{"v-rim-1"}},
			{ID: "p-astar", Name: "Astar soft", Tags: map[string]any{"astar": true}, VariantIDs: []string{"v-astar-1"}},
			{ID: "p-oddiy", Name: "Velvet Milano", VariantIDs: []string{"v-oddiy-1"}},
		},
		Variants: []Variant{
			{ID: "v-rulon-1", ProductID: "p-rulon", IsActive: true},
			{ID: "v-rim-1", ProductID: "p-rim", IsActive: true},
			{ID: "v-astar-1", ProductID: "p-astar", IsActive: true},
			{ID: "v-oddiy-1", ProductID: "p-oddiy", IsActive: true},
		},
	})

	for _, tc := range []struct {
		variant, layer string
		supported      bool
	}{
		{"v-rulon-1", "rulon", false},
		{"v-rim-1", "rim", false},
		{"v-astar-1", "astar", true},
		// Nomsiz hech qanday teg yo'q — LayerOf "portyera" qaytaradi, hisoblash mumkin.
		{"v-oddiy-1", "portyera", true},
		// Katalogda yo'q variant — "topilmadi" bu tekshiruvning ishi emas (validatsiya alohida).
		{"v-yoq", "", true},
	} {
		layer, supported := CalcSupportLayer(v, tc.variant)
		if layer != tc.layer || supported != tc.supported {
			t.Errorf("variant=%s: layer=%q supported=%v, kutilgan %q/%v",
				tc.variant, layer, supported, tc.layer, tc.supported)
		}
	}
}

// A1: LayerOf nom evristikasi (teg yo'q) ham rulon/rim ni ushlashi kerak — katalog
// yangi mahsulot kiritsa teg qo'yilmas ehtimoli bor. Eslatma: LayerOf "rulon"
// yozuvini qidiradi, katalog nomida "Rolon" (bitta l) bo'lsa teg ( kontrakt )
// ishonchli yo'l — shuning uchun bu yerda aniq "Rulon" yoziladi.
func TestCalcSupportProduct_NameHeuristicAlsoBlocks(t *testing.T) {
	v := searchView(Snapshot{
		Version: 1,
		Products: []Product{
			{ID: "p-rulon-nomi", Name: "Rulon Premium 300", VariantIDs: []string{"v-1"}},
			{ID: "p-rim-nomi", Name: "Yengil Rim Och", VariantIDs: []string{"v-2"}},
		},
		Variants: []Variant{
			{ID: "v-1", ProductID: "p-rulon-nomi", IsActive: true},
			{ID: "v-2", ProductID: "p-rim-nomi", IsActive: true},
		},
	})
	for _, id := range []string{"v-1", "v-2"} {
		if _, supported := CalcSupportLayer(v, id); supported {
			t.Errorf("variant=%s: nom evristikasi qatlamni aniqlagan, hisoblash taqiqlanmadi", id)
		}
	}
}

// A1: MsgNotCalcSupported — sotuvchiga ko'rsatiladigan bitta aniq matn (errors-as-results).
func TestMsgNotCalcSupported_StableText(t *testing.T) {
	if MsgNotCalcSupported == "" {
		t.Fatal("xato matni bo'sh")
	}
	// Matn Russian/English emas va narx vaqtini ("hozircha") o'z ichiga oladi —
	// sotuvchi mijozga shu yerda aytsin.
	for _, want := range []string{"rulon", "AI", "sotuvchi"} {
		if !containsText(MsgNotCalcSupported, want) {
			t.Errorf("matnda %q yo'q: %q", want, MsgNotCalcSupported)
		}
	}
}

func containsText(hay, needle string) bool { return strings.Contains(hay, needle) }
