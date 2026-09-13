package prompt

import (
	"strings"
	"testing"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

func TestBuilder_Build(t *testing.T) {
	b := Builder{Fallback: FallbackSystemPrompt}

	t.Run("Fallback, next_question, blocks order", func(t *testing.T) {
		// NEGA: Base bo'sh bo'lganda FallbackSystemPrompt ishlatilishini,
		// next_question to'g'ri qo'shilishini va bloklar ketma-ketligini tekshiramiz.
		state := curtains.NewSessionState()
		// Boshlang'ich state uchun next_question "xona turi" bo'lishi kerak.

		in := Input{
			Base:         "", // Fallback ishlatiladi
			State:        state,
			CatalogStale: true,
		}

		res := b.Build(in)

		if !strings.HasPrefix(res, FallbackSystemPrompt) {
			t.Error("Base bo'sh bo'lganda Fallback bilan boshlanmadi")
		}

		idxState := strings.Index(res, "<state>")
		idxNext := strings.Index(res, "<next_question>")
		idxKnow := strings.Index(res, "<knowledge>")
		idxCat := strings.Index(res, "<catalog")

		if idxState >= idxNext || idxNext >= idxKnow || idxKnow >= idxCat {
			t.Errorf("Bloklar tartibi noto'g'ri: state=%d, next=%d, know=%d, cat=%d", idxState, idxNext, idxKnow, idxCat)
		}

		if !strings.Contains(res, "<next_question>xona turi</next_question>") {
			t.Errorf("Bo'sh state uchun next_question 'xona turi' bo'lishi kerak, lekin yo'q:\n%s", res)
		}

		if !strings.Contains(res, "stale=true") {
			t.Error("CatalogStale=true chiqishi kerak")
		}
	})

	t.Run("Knowledge filter, priority, sanitization, inline_max", func(t *testing.T) {
		// NEGA: Knowledge rule'lar to'g'ri filtrlanishi, priority bo'yicha saralanishi
		// va HTML teglar olib tashlanishi zarur.
		state := curtains.NewSessionState()
		state.Room = "living"

		knows := []curtains.KnowledgeRule{
			{ID: "1", Condition: map[string]any{"room": "kitchen"}, Recommendation: "Oshxona uchun rim parda.", Priority: 10},
			{ID: "2", Condition: map[string]any{"room": "living"}, Recommendation: "Mehmonxona uchun <script>alert(1)</script> qalin parda.", Priority: 15},
			{ID: "3", Condition: map[string]any{}, Recommendation: "Umumiy qoida.", Priority: 5},
			{ID: "4", Condition: map[string]any{"room": "living"}, Recommendation: "Ikkinchi mehmonxona.", Priority: 20},
		}

		in := Input{
			State:              state,
			Knowledge:          knows,
			KnowledgeInlineMax: 2,
		}

		res := b.Build(in)

		if strings.Contains(res, "Oshxona uchun rim parda.") {
			t.Error("room=kitchen bo'lgan rule room=living state'ga tushmasligi kerak")
		}

		if strings.Contains(res, "<script>") {
			t.Error("< va > belgilari sanitizatsiya qilinmagan")
		}
		if !strings.Contains(res, "alert(1)") {
			t.Error("Sanitizatsiyadan so'ng matn qolishi kerak (faqat teglar o'chadi)")
		}

		// Priority bo'yicha saralanganda: ID 4 (pri=20), ID 2 (pri=15), ID 3 (pri=5)
		// InlineMax=2 bo'lgani uchun faqat ID 4 va ID 2 chiqishi kerak.
		if !strings.Contains(res, "Ikkinchi mehmonxona") {
			t.Error("Eng yuqori priority rule chiqishi kerak")
		}
		if !strings.Contains(res, "alert(1)") {
			t.Error("Ikkinchi priority rule chiqishi kerak")
		}
		if strings.Contains(res, "Umumiy qoida") {
			t.Error("InlineMax=2 dan tashqaridagi rule chiqmasligi kerak")
		}
	})

	t.Run("CustomerMode va Locale", func(t *testing.T) {
		// NEGA: customer_mode yoniq bo'lsa ogohlantirish qo'shilishi va rus tili so'ralsa
		// shunga mos qator yozilishi kerak.
		in := Input{
			State:        curtains.NewSessionState(),
			CustomerMode: true,
			Locale:       "ru",
		}

		res := b.Build(in)

		if !strings.Contains(res, "<customer_mode>Hozir qurilma mijoz qo'lida") {
			t.Error("customer_mode bloki yo'q")
		}
		if !strings.Contains(res, "Javob tili: rus") {
			t.Error("ru locale uchun matn yo'q")
		}
	})

	t.Run("Truncation when length > 24000", func(t *testing.T) {
		// NEGA: Context window chegarasi buzilmasligi uchun juda uzun knowledge
		// kesilishi shart.
		state := curtains.NewSessionState()

		// 15000 belgili base
		baseStr := strings.Repeat("a", 15000)

		// 10000 belgili knowledge
		longKnow := strings.Repeat("b", 10000)
		knows := []curtains.KnowledgeRule{
			{Condition: map[string]any{}, Recommendation: longKnow, Priority: 1},
		}

		in := Input{
			Base:      baseStr,
			State:     state,
			Knowledge: knows,
		}

		res := b.Build(in)

		runes := []rune(res)
		if len(runes) > 24000 {
			t.Errorf("Kutilgan uzunlik <= 24000, keldi %d", len(runes))
		}
	})

	t.Run("FallbackSystemPrompt contents", func(t *testing.T) {
		// NEGA: Prompt TZ bo'yicha tuzatilganiga ishonch hosil qilish uchun, unda "ombor"
		// so'zi yo'qligini va "set_context" kabi zarur qismlar borligini tekshiramiz.
		if FallbackSystemPrompt == "" {
			t.Fatal("FallbackSystemPrompt bo'sh bo'lmasligi kerak (go:embed)")
		}

		lowerPrompt := strings.ToLower(FallbackSystemPrompt)
		if strings.Contains(lowerPrompt, "ombor") {
			t.Error("Fallback prompt ichida 'ombor' so'zi qatnashmasligi kerak (v1 da ombor yo'q)")
		}
		if !strings.Contains(lowerPrompt, "set_context") {
			t.Error("Fallback prompt ichida 'set_context' bo'lishi shart")
		}
	})
}
