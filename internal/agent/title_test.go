package agent

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-ai/internal/session"
)

// titleModel — scriptModel ustiga Generate qo'shadi: sarlavha so'rovi tool'siz
// bitta Generate bo'lishi kerak.
type titleModel struct {
	scriptModel
	generateCalls int
	generateErr   error
}

func (m *titleModel) Generate(_ context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.generateCalls++
	if m.generateErr != nil {
		return nil, m.generateErr
	}
	if len(msgs) != 2 || msgs[0].Role != schema.System || !strings.Contains(msgs[1].Content, "Foydalanuvchi:") {
		return nil, errNoTitlePrompt
	}
	return &schema.Message{
		Role:         schema.Assistant,
		Content:      "\"Mehmonxona uchun parda.\"\n",
		ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 30, CompletionTokens: 6}},
	}, nil
}

var errNoTitlePrompt = errors.New("sarlavha prompti kutilgan shaklda emas")

func (m *titleModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func runTitleStream(t *testing.T, m *titleModel, store session.Store, sessionID string) (title string, done Event) {
	t.Helper()
	chat, err := NewChat(context.Background(), m, []tool.BaseTool{findTool(t)}, store, Options{Stream: true, Title: true})
	if err != nil {
		t.Fatalf("NewChat: %v", err)
	}
	for ev := range chat.Stream(context.Background(), sessionID, "mehmonxonaga parda kerak") {
		switch ev.Kind {
		case KindTitle:
			title = ev.Content
		case KindDone:
			done = ev
		}
	}
	return title, done
}

func TestChatStream_TitleOnFirstTurn(t *testing.T) {
	m := &titleModel{}
	title, done := runTitleStream(t, m, session.NewMemory(), "s1")
	if m.generateCalls != 1 {
		t.Fatalf("Generate chaqiruvlari = %d, want 1", m.generateCalls)
	}
	if title != "Mehmonxona uchun parda" {
		t.Fatalf("title = %q", title)
	}
	// Sarlavha tokenlari done.usage'ga qo'shiladi: 10+12+30 / 5+4+6.
	if done.Usage == nil || done.Usage.PromptTokens != 52 || done.Usage.CompletionTokens != 15 {
		t.Fatalf("done.usage = %+v", done.Usage)
	}
}

func TestChatStream_NoTitleOnLaterTurns(t *testing.T) {
	store := session.NewMemory()
	if err := store.Append(context.Background(), "s2", schema.UserMessage("salom"), schema.AssistantMessage("salom!", nil)); err != nil {
		t.Fatal(err)
	}
	m := &titleModel{}
	title, _ := runTitleStream(t, m, store, "s2")
	if m.generateCalls != 0 || title != "" {
		t.Fatalf("keyingi navbatda sarlavha bo'lmasligi kerak: calls=%d title=%q", m.generateCalls, title)
	}
}

func TestChatStream_TitleErrorIsSilent(t *testing.T) {
	m := &titleModel{generateErr: context.DeadlineExceeded}
	title, done := runTitleStream(t, m, session.NewMemory(), "s3")
	if title != "" || done.Kind != KindDone {
		t.Fatalf("xato jim o'tishi kerak: title=%q done=%v", title, done.Kind)
	}
}

func TestCleanTitle(t *testing.T) {
	cases := map[string]string{
		"  «Oshxona pardasi».  ":    "Oshxona pardasi",
		"Sarlavha: Yotoqxona\nizoh": "Sarlavha: Yotoqxona",
		"**Blackout   tanlash**":    "Blackout tanlash",
		strings.Repeat("a", 100):    strings.Repeat("a", 71) + "…",
	}
	for in, want := range cases {
		if got := cleanTitle(in); got != want {
			t.Errorf("cleanTitle(%q) = %q, want %q", in, got, want)
		}
	}
}
