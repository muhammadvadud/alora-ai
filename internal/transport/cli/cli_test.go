package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muhammadvadud/alora-agent/internal/agent"
)

func TestRenderThinkingChunksWithSingleLabel(t *testing.T) {
	events := make(chan agent.Event, 5)
	events <- agent.Event{Kind: agent.KindThinking, Content: "Foyd"}
	events <- agent.Event{Kind: agent.KindThinking, Content: "alanuvchi "}
	events <- agent.Event{Kind: agent.KindThinking, Content: "salom dedi."}
	events <- agent.Event{Kind: agent.KindContent, Content: "Salom!"}
	events <- agent.Event{Kind: agent.KindDone}
	close(events)

	var stdout, stderr bytes.Buffer
	renderTo(events, &stdout, &stderr)

	if got := strings.Count(stdout.String(), "💭"); got != 1 {
		t.Fatalf("thinking belgisi soni = %d, 1 bo'lishi kerak; output=%q", got, stdout.String())
	}
	if !strings.Contains(stdout.String(), "Foyd\033[0m\033[2malanuvchi ") {
		t.Fatalf("thinking chunk'lari orasiga belgi qo'shildi: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "salom dedi.\033[0m\nAgent: Salom!") {
		t.Fatalf("javob thinking'dan alohida qatorda emas: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("kutilmagan stderr: %q", stderr.String())
	}
}
