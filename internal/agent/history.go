package agent

import (
	"fmt"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
)

// Standart qiymatlar (config bermasa ishlatiladi).
//
// defaultMaxHistory: har navbat tarixga 4+ xabar qo'shadi (user + tool_call'li
// assistant + tool natijalari + yakuniy javob) va ularning HAMMASI har so'rovda
// modelga ketadi. Chegarasiz tarix kontekstni ham, hisobni ham portlatadi.
// ~40 xabar ≈ 8-10 navbat. Suhbat oqimi uzun bo'lsa (ko'p savol-javob) oshiring:
// oynadan chiqib ketgan ma'lumotni model qaytadan so'raydi.
const (
	defaultMaxStep            = 20
	defaultMaxHistory         = 40
	defaultMaxToolResultChars = 3000
)

// trimHistory tarixni oxirgi `max` xabar atrofida cheklaydi.
//
// MUHIM (invariant): oyna FAQAT User roldagi xabardan boshlanishi mumkin. Aks holda
// kontekst tool natijasidan yoki tool_calls li assistant'dan ajralib boshlanardi va
// provayder "tool_call_id ga mos assistant xabari yo'q" deb 400 qaytarardi. Shu sabab
// chegara oldinga — keyingi navbat boshiga suriladi.
//
// Bu — Chat.Stream dagi saqlash qoidasining ikkinchi yarmi: u yozishda navbatni yaxlit
// saqlaydi, bu esa o'qishda o'sha yaxlitlikni buzmaydi.
func trimHistory(msgs []*schema.Message, max int) []*schema.Message {
	if max <= 0 || len(msgs) <= max {
		return msgs
	}
	start := len(msgs) - max
	for start < len(msgs) && msgs[start].Role != schema.User {
		start++
	}
	if start == len(msgs) {
		// Oyna ichida navbat chegarasi yo'q — butun tarixni beramiz: noto'g'ri
		// kesish 400 beradi, uzunroq kontekst esa faqat qimmatroq.
		return msgs
	}
	return msgs[start:]
}

// limitToolResults model kontekstini og'irlashtiradigan eski tool natijalarini
// qisqartiradi. Store'dagi xabarlar o'zgarmasligi uchun faqat xabar struct'ining
// nusxasi tahrirlanadi; oxirgi ikki natija esa modelga to'liq kerak bo'ladi.
func limitToolResults(msgs []*schema.Message, maxChars int) []*schema.Message {
	if maxChars <= 0 || len(msgs) == 0 {
		return msgs
	}

	totalTools := 0
	for _, msg := range msgs {
		if msg.Role == schema.Tool {
			totalTools++
		}
	}

	seenTools := 0
	result := make([]*schema.Message, len(msgs))
	copy(result, msgs)
	for i, msg := range msgs {
		if msg.Role != schema.Tool {
			continue
		}
		seenTools++
		if seenTools > totalTools-2 || utf8.RuneCountInString(msg.Content) <= maxChars {
			continue
		}
		clone := *msg
		count := utf8.RuneCountInString(msg.Content)
		clone.Content = string([]rune(msg.Content)[:maxChars]) + fmt.Sprintf(" …[qisqartirildi, %d belgi]", count)
		result[i] = &clone
	}
	return result
}
