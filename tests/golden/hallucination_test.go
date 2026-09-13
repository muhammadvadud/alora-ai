package golden

import (
	"strings"
	"testing"
)

// Harness'ning o'zi to'qilgan narxni ushlab olishini isbotlaydi (tasks 1.42,
// rules §14.5): bazada YO'Q mahsulot ("atlas Roma") so'raladi, mock model esa
// narx aytishga urinadi. Tool natijasida bunday son yo'q, shuning uchun qadam
// hallucination bilan muvaffaqiyatsiz bo'lishi kerak — "ushendi" deganni isbot.
func TestHallucination_HarnessDetectsInventedPrice(t *testing.T) {
	res := runScenario(scenario{
		Name: "atlas-roma",
		Steps: []step{{
			User:        "To'y xonasiga atlas Roma pardasi kerak",
			ExpectTools: []string{"product_search"},
			LLM: []llmStep{
				{Tools: []llmTool{{Name: "product_search", Args: map[string]any{"room": "wedding", "kind": "fabric"}}}},
				{Text: "Bu atlas Roma ajoyib variant, 2 500 000 so'm turadi."},
			},
		}},
	})

	if res.ok() {
		t.Fatal("harness to'qilgan narxni ushlamadi")
	}
	if !strings.Contains(res.describe(), "hallucination") {
		t.Fatalf("xato hallucination deb belgilanmadi:\n%s", res.describe())
	}
}

// Narx tool natijasida bo'lsa, uni javobda aytish MUMKIN (rules §14.4): bu
// ssenariyda 5 070 000 (price_quote total) va 2 535 000 (prepayment) raqamlari
// toollardan chiqqan, shuning uchun qadam o'tishi shart (0 hallucination emas,
// balki har narx asosli).
func TestHallucination_PassesWhenPriceComesFromTools(t *testing.T) {
	res := runScenario(scenario{
		Name: "narx-tooldan",
		Steps: []step{{
			User:        "Karniz 4 m, balandlik 2.6 m, grafit mato. Hisob va avansni ayt.",
			ExpectTools: []string{"curtain_calculate", "price_quote"},
			ExpectStage: "calc",
			LLM: []llmStep{
				{Tools: []llmTool{{Name: "curtain_calculate", Args: map[string]any{
					"cornice_length_m": 4.0,
					"height_m":         2.6,
					"parts": []any{map[string]any{
						"variant_id": VHiTechGrafit, "role": "portyera", "coefficient": 2.5,
					}},
				}}}},
				{Tools: []llmTool{{Name: "price_quote", Args: map[string]any{"need_delivery": true}}}},
				{Text: "Jami 5 070 000 so'm turadi, avans 2 535 000 so'm."},
			},
		}},
	})

	if !res.ok() {
		t.Fatalf("tool natijasidagi narx hallucination deb qayd etildi:\n%s", res.describe())
	}
}
