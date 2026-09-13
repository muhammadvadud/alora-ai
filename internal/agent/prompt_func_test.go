package agent

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-agent/internal/session"
)

type promptCaptureModel struct{ messages [][]*schema.Message }

func (m *promptCaptureModel) Generate(_ context.Context, messages []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.messages = append(m.messages, messages)
	return schema.AssistantMessage("tayyor", nil), nil
}

func (m *promptCaptureModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

func (m *promptCaptureModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func TestChatSystemPromptFuncPerStream(t *testing.T) {
	model := &promptCaptureModel{}
	chat, err := NewChat(context.Background(), model, nil, session.NewMemory(), Options{
		SystemPromptFunc: func(_ context.Context, sessionID string) string { return "prompt:" + sessionID },
	})
	if err != nil {
		t.Fatal(err)
	}
	for range chat.Stream(context.Background(), "s1", "salom") {
	}
	for range chat.Stream(context.Background(), "s2", "salom") {
	}
	if len(model.messages) != 2 {
		t.Fatalf("model chaqiruvlari = %d, want 2", len(model.messages))
	}
	if got := model.messages[0][0].Content; got != "prompt:s1" {
		t.Errorf("birinchi prompt = %q", got)
	}
	if got := model.messages[1][0].Content; got != "prompt:s2" {
		t.Errorf("ikkinchi prompt = %q", got)
	}
}
