package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-agent/internal/config"
)

func TestMockProvider_HotelScript(t *testing.T) {
	m, err := buildMock(context.Background(), config.LLMConfig{Model: "test-mock"})
	if err != nil {
		t.Fatal(err)
	}
	model := m
	messages := []*schema.Message{schema.UserMessage("Mehmonxona uchun hi-tech parda kerak")}

	first := recvOne(t, model, messages)
	if got := first.ToolCalls[0].Function.Name; got != "set_context" {
		t.Fatalf("birinchi tool = %q, set_context kutilgan", got)
	}
	messages = append(messages, first, schema.ToolMessage(`{"state":{},"next_question":""}`, first.ToolCalls[0].ID, schema.WithToolName("set_context")))
	second := recvOne(t, model, messages)
	if got := second.ToolCalls[0].Function.Name; got != "product_search" {
		t.Fatalf("ikkinchi tool = %q, product_search kutilgan", got)
	}
	productResult := `{"products":[{"variant_id":"v-1","name":"Atlas Roma"},{"variant_id":"v-2","name":"Velvet Grafit"}]}`
	messages = append(messages, second, schema.ToolMessage(productResult, second.ToolCalls[0].ID, schema.WithToolName("product_search")))
	text := collectText(t, model, messages)
	if text != "Mana mos matolar: Atlas Roma, Velvet Grafit" {
		t.Fatalf("yakuniy matn = %q", text)
	}
}

func TestMockProvider_CalculationUsesPreviousProduct(t *testing.T) {
	m, err := buildMock(context.Background(), config.LLMConfig{Model: "test-mock"})
	if err != nil {
		t.Fatal(err)
	}
	messages := []*schema.Message{
		schema.UserMessage("Karniz 4.2 m, balandlik 2.8 m, hisobla"),
		schema.ToolMessage(`{"products":[{"variant_id":"v-9","name":"Atlas"}]}`, "old", schema.WithToolName("product_search")),
	}
	call := recvOne(t, m, messages)
	if call.ToolCalls[0].Function.Name != "curtain_calculate" {
		t.Fatalf("tool = %q", call.ToolCalls[0].Function.Name)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(call.ToolCalls[0].Function.Arguments), &args); err != nil {
		t.Fatal(err)
	}
	if args["cornice_length_m"] != 4.2 || args["height_m"] != 2.8 {
		t.Fatalf("o'lchamlar = %#v", args)
	}
	parts := args["parts"].([]any)
	if parts[0].(map[string]any)["variant_id"] != "v-9" {
		t.Fatalf("variant = %#v", parts[0])
	}
}

func TestMockProvider_WithToolsReturnsWrapper(t *testing.T) {
	m, err := buildMock(context.Background(), config.LLMConfig{Model: "test-mock"})
	if err != nil {
		t.Fatal(err)
	}
	bound, err := m.WithTools([]*schema.ToolInfo{{Name: "product_search"}})
	if err != nil {
		t.Fatal(err)
	}
	if bound == m {
		t.Fatal("WithTools yangi model wrapper qaytarishi kerak")
	}
	if bound == nil {
		t.Fatal("WithTools nil model qaytardi")
	}
}

func recvOne(t *testing.T, m model.ToolCallingChatModel, messages []*schema.Message) *schema.Message {
	t.Helper()
	stream, err := m.Stream(context.Background(), messages)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	message, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func collectText(t *testing.T, m model.ToolCallingChatModel, messages []*schema.Message) string {
	t.Helper()
	stream, err := m.Stream(context.Background(), messages)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	var text string
	for {
		message, recvErr := stream.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				return text
			}
			t.Fatal(recvErr)
		}
		text += message.Content
	}
}
