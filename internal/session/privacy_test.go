package session

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

func TestPrivateStore_ReadsLegacyHistoryWithoutMutatingOriginal(t *testing.T) {
	ctx := context.Background()
	underlying := NewMemory()
	message := schema.UserMessage("Mening ismim Sinovbek Auditov; manzilim Sinov ko'chasi 123")
	call := schema.AssistantMessage("", []schema.ToolCall{{ID: "call-1", Function: schema.FunctionCall{Name: "order_draft", Arguments: `{"customer_name":"Sinovbek Auditov","total":"500000000"}`}}})
	result := schema.ToolMessage(`{"customer_name":"Sinovbek Auditov","total":"500000000"}`, "call-1")
	if err := underlying.Append(ctx, "old", message, call, result); err != nil {
		t.Fatal(err)
	}
	history, err := WithPIIScrubbing(underlying).History(ctx, "old")
	if err != nil || len(history) != 3 {
		t.Fatalf("tarix: %v %+v", err, history)
	}
	if strings.Contains(history[0].Content, "Sinovbek") || strings.Contains(history[0].Content, "Sinov ko'chasi") || strings.Contains(history[1].ToolCalls[0].Function.Arguments, "Sinovbek") || strings.Contains(history[2].Content, "Sinovbek") {
		t.Fatalf("eski PII modelga o'tdi: %+v", history)
	}
	if history[1].ToolCalls[0].ID != "call-1" || history[2].ToolCallID != "call-1" || !strings.Contains(history[2].Content, `"total":"500000000"`) {
		t.Fatal("tool juftligi yoki moliyaviy dalil buzildi")
	}
	if !strings.Contains(message.Content, "Sinovbek") || !strings.Contains(call.ToolCalls[0].Function.Arguments, "Sinovbek") {
		t.Fatal("eski store/caller obyektlari o'qishda o'zgarmasligi kerak")
	}
}

func TestPrivateStore_ScrubsNewHistoryAndState(t *testing.T) {
	ctx := context.Background()
	underlying := NewMemory()
	store := WithPIIScrubbing(underlying)
	message := schema.UserMessage("Email: sinov@example.test; ismim Sinovbek Auditov")
	if err := store.Append(ctx, "new", message); err != nil {
		t.Fatal(err)
	}
	raw, err := underlying.History(ctx, "new")
	if err != nil || len(raw) != 1 || strings.Contains(raw[0].Content, "Sinovbek") || strings.Contains(raw[0].Content, "sinov@example.test") {
		t.Fatalf("yangi tarix: %v %+v", err, raw)
	}
	state := curtains.SessionState{Colors: []string{"oq", "ismim Sinovbek Auditov"}, Windows: []curtains.WindowSpec{{RoomName: "manzilim Sinov ko'chasi 123"}}}
	if err := store.SetState(ctx, "new", state); err != nil {
		t.Fatal(err)
	}
	got, err := store.State(ctx, "new")
	if err != nil || strings.Contains(got.Colors[1], "Sinovbek") || strings.Contains(got.Windows[0].RoomName, "Sinov ko'chasi") || got.Colors[0] != "oq" {
		t.Fatalf("state: %v %+v", err, got)
	}
	if !strings.Contains(state.Colors[1], "Sinovbek") || !strings.Contains(state.Windows[0].RoomName, "Sinov ko'chasi") {
		t.Fatal("kirish state o'zgartirildi")
	}
	if err := store.Reset(ctx, "new"); err != nil {
		t.Fatal(err)
	}
	gotHistory, err := store.History(ctx, "new")
	if err != nil || len(gotHistory) != 0 {
		t.Fatalf("reset: %v %+v", err, gotHistory)
	}
}

func TestPrivateMessages_TextPartsAndSpeakerNameCannotBypassMask(t *testing.T) {
	secret := "Ismim Sinovbek Auditov"
	message := &schema.Message{
		Role: schema.User, Name: "Sinovbek", ReasoningContent: secret,
		UserInputMultiContent:    []schema.MessageInputPart{{Type: schema.ChatMessagePartTypeText, Text: secret}},
		AssistantGenMultiContent: []schema.MessageOutputPart{{Type: schema.ChatMessagePartTypeText, Text: secret}},
	}
	message.MultiContent = slices.Grow(message.MultiContent, 1)[:1]
	message.MultiContent[0].Type = schema.ChatMessagePartTypeText
	message.MultiContent[0].Text = secret
	got := privateMessages([]*schema.Message{message})[0]
	for _, text := range []string{got.Name, got.ReasoningContent, got.MultiContent[0].Text, got.UserInputMultiContent[0].Text, got.AssistantGenMultiContent[0].Text} {
		if strings.Contains(text, "Sinovbek") {
			t.Fatalf("muqobil matn maydoni PII ni o'tkazdi: %s", text)
		}
	}
	if message.Name != "Sinovbek" || message.MultiContent[0].Text != secret || message.UserInputMultiContent[0].Text != secret || message.AssistantGenMultiContent[0].Text != secret {
		t.Fatal("asl xabar maydonlari o'zgarmasligi kerak")
	}
}
