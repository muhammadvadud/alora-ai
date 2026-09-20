package agent

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-ai/internal/llm"
)

// Sarlavha faqat suhbatning BIRINCHI navbatidan keyin, tool'siz bitta qisqa
// Generate bilan yaratiladi (~100 token). Xato bo'lsa jim o'tkaziladi — Django
// birinchi xabar matnidan zaxira sarlavha qo'yadi (generate_conversation_title).
const (
	titleMaxRunes   = 72 // rules §? Django CalcRule.conversation_title_max_len bilan uyg'un
	titleInputMax   = 600
	titleTimeout    = 8 * time.Second
	titleSystemText = "Sen parda do'koni suhbatlariga sarlavha qo'yuvchisan. Foydalanuvchi xabari va " +
		"yordamchi javobidan kelib chiqib suhbat mavzusini 3–6 so'zli qisqa sarlavha qilib yoz. " +
		"Faqat sarlavhaning o'zini qaytar: tirnoqsiz, nuqtasiz, emoji'siz, izohsiz. " +
		"Til — foydalanuvchi tili (o'zbek lotin yoki rus). Salomlashuvdan boshqa mazmun bo'lmasa " +
		"\"Yangi suhbat\" deb yoz."
)

// generateTitle — birinchi navbat uchun sarlavha. Asosiy modelning tool'siz
// nusxasi (c.model) ishlatiladi; usage done.usage'ga qo'shiladi.
func (c *Chat) generateTitle(ctx context.Context, m model.ToolCallingChatModel, input, answer string) (string, llm.TokenUsage) {
	if m == nil {
		return "", llm.TokenUsage{}
	}
	ctx, cancel := context.WithTimeout(ctx, titleTimeout)
	defer cancel()
	prompt := "Foydalanuvchi: " + clipRunes(strings.TrimSpace(input), titleInputMax) +
		"\nYordamchi: " + clipRunes(strings.TrimSpace(answer), titleInputMax)
	msg, err := m.Generate(ctx, []*schema.Message{
		schema.SystemMessage(titleSystemText),
		schema.UserMessage(prompt),
	})
	if err != nil {
		slog.Warn("suhbat sarlavhasini yaratib bo'lmadi", "error", err)
		return "", llm.TokenUsage{}
	}
	u, _ := llm.Usage(msg)
	return cleanTitle(msg.Content), u
}

// cleanTitle — modelning ortiqcha bezaklarini olib tashlaydi va uzunlikni cheklaydi.
func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	s = strings.Trim(s, "\"'«»“”`*#.! ")
	s = strings.Join(strings.Fields(s), " ")
	return clipRunes(s, titleMaxRunes)
}

func clipRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:max-1])) + "…"
}
