package agent

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

// turn — bitta navbat: user + tool_call'li assistant + tool natijasi + javob.
func turn(text string) []*schema.Message {
	return []*schema.Message{
		schema.UserMessage(text),
		schema.AssistantMessage("", []schema.ToolCall{{ID: "c1", Function: schema.FunctionCall{Name: "t"}}}),
		schema.ToolMessage("natija", "c1"),
		schema.AssistantMessage("javob", nil),
	}
}

func build(n int) []*schema.Message {
	var out []*schema.Message
	for i := 0; i < n; i++ {
		out = append(out, turn("savol")...)
	}
	return out
}

func TestTrimHistoryKeepsShortHistory(t *testing.T) {
	msgs := build(2) // 8 xabar
	if got := trimHistory(msgs, 40); len(got) != len(msgs) {
		t.Errorf("qisqa tarix kesildi: %d -> %d", len(msgs), len(got))
	}
	if got := trimHistory(msgs, 0); len(got) != len(msgs) {
		t.Errorf("max=0 da kesildi: %d", len(got))
	}
	if got := trimHistory(msgs, -1); len(got) != len(msgs) {
		t.Errorf("max<0 da kesildi: %d", len(got))
	}
}

// ENG MUHIM: kesilgan tarix DOIM User'dan boshlanishi kerak — aks holda tool
// natijasi o'ziga mos assistant(tool_calls) xabarisiz qolib, provayder 400 beradi.
func TestTrimHistoryStartsAtUser(t *testing.T) {
	msgs := build(20) // 80 xabar

	for _, max := range []int{1, 2, 3, 4, 5, 7, 10, 13, 21, 40, 79} {
		got := trimHistory(msgs, max)
		if len(got) == 0 {
			t.Fatalf("max=%d: bo'sh qaytdi", max)
		}
		if got[0].Role != schema.User {
			t.Errorf("max=%d: kesim %q roldan boshlandi, User bo'lishi kerak", max, got[0].Role)
		}
		if len(got) > len(msgs) {
			t.Errorf("max=%d: tarix o'sib ketdi", max)
		}
		// Har tool xabari oldida tool_calls li assistant turishi kerak.
		for i, m := range got {
			if m.Role == schema.Tool && (i == 0 || len(got[i-1].ToolCalls) == 0 && got[i-1].Role != schema.Tool) {
				t.Errorf("max=%d: %d-o'rindagi tool xabari juftsiz qoldi", max, i)
			}
		}
	}
}

func TestTrimHistoryTrimsToTurnBoundary(t *testing.T) {
	msgs := build(3) // 12 xabar: navbatlar 0,4,8 dan boshlanadi
	// max=6 -> start=12-6=6 -> 6 tool, User emas -> oldinga 8 ga suriladi.
	got := trimHistory(msgs, 6)
	if len(got) != 4 {
		t.Fatalf("uzunlik = %d, want 4 (oxirgi navbat)", len(got))
	}
	if got[0].Role != schema.User {
		t.Errorf("User'dan boshlanmadi: %q", got[0].Role)
	}
}

// Oyna ichida User bo'lmasa — kesmaymiz (noto'g'ri kesish 400 beradi,
// uzun kontekst esa faqat qimmat).
func TestTrimHistoryNoUserInWindowKeepsAll(t *testing.T) {
	msgs := []*schema.Message{
		schema.UserMessage("savol"),
		schema.AssistantMessage("", []schema.ToolCall{{ID: "c1", Function: schema.FunctionCall{Name: "t"}}}),
		schema.ToolMessage("natija", "c1"),
		schema.ToolMessage("natija2", "c2"),
	}
	got := trimHistory(msgs, 2) // oxirgi 2 tasi tool — User yo'q
	if len(got) != len(msgs) {
		t.Errorf("uzunlik = %d, want %d (kesilmasligi kerak edi)", len(got), len(msgs))
	}
}
