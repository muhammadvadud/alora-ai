package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-ai/internal/session"
)

type fallbackTestModel struct {
	responses []error
	content   string
	calls     int
}

func (m *fallbackTestModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	m.calls++
	if len(m.responses) > 0 {
		err := m.responses[0]
		m.responses = m.responses[1:]
		if err != nil {
			return nil, err
		}
	}
	return schema.AssistantMessage(m.content, nil), nil
}

func (m *fallbackTestModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("fallback test stream ishlatilmasligi kerak")
}

func (m *fallbackTestModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func TestChat_FallbackAfterTwoRetryableErrors(t *testing.T) {
	primary := &fallbackTestModel{responses: []error{errors.New("503"), errors.New("upstream 503")}}
	fallback := &fallbackTestModel{content: "fallback javob"}
	chat, err := NewChat(context.Background(), primary, nil, session.NewMemory(), Options{
		Fallback: fallback, FallbackProvider: "claude", Provider: "deepseek", Stream: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	var done *Usage
	var content string
	for ev := range chat.Stream(context.Background(), "fallback", "salom") {
		if ev.Kind == KindContent {
			content += ev.Content
		}
		if ev.Kind == KindDone {
			done = ev.Usage
		}
	}
	if content != "fallback javob" || done == nil || done.Provider != "claude" {
		t.Fatalf("fallback natijasi = content %q usage %+v", content, done)
	}
	if primary.calls != 2 || fallback.calls != 1 {
		t.Fatalf("chaqiruvlar = primary %d fallback %d", primary.calls, fallback.calls)
	}
}

func TestChat_NonRetryableErrorDoesNotUseFallback(t *testing.T) {
	primary := &fallbackTestModel{responses: []error{errors.New("400 bad request")}}
	fallback := &fallbackTestModel{content: "ishlatilmasin"}
	chat, err := NewChat(context.Background(), primary, nil, session.NewMemory(), Options{Fallback: fallback, Stream: false})
	if err != nil {
		t.Fatal(err)
	}
	var gotError bool
	for ev := range chat.Stream(context.Background(), "non-retryable", "salom") {
		gotError = gotError || ev.Kind == KindError
	}
	if !gotError || fallback.calls != 0 {
		t.Fatalf("xato/fallback = %v/%d", gotError, fallback.calls)
	}
}

func TestChat_FallbackFailureDoesNotAppendHistory(t *testing.T) {
	primary := &fallbackTestModel{responses: []error{errors.New("503"), errors.New("503")}}
	fallback := &fallbackTestModel{responses: []error{errors.New("504")}}
	store := session.NewMemory()
	chat, err := NewChat(context.Background(), primary, nil, store, Options{Fallback: fallback, Stream: false})
	if err != nil {
		t.Fatal(err)
	}
	for range chat.Stream(context.Background(), "failed", "salom") {
	}
	history, err := store.History(context.Background(), "failed")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("xato navbat tarixga yozildi: %d", len(history))
	}
}
