package grpc

import (
	"testing"

	"github.com/muhammadvadud/alora-agent/internal/agent"
)

func TestToProtoMapsProductsAndUsage(t *testing.T) {
	got := toProto(agent.Event{
		Kind:     agent.KindDone,
		Products: []agent.ProductRef{{VariantID: "v1", ProductName: "Mato", Price: 12.5, SKU: "SKU-1"}},
		Usage:    &agent.Usage{Provider: "deepseek", Model: "model", PromptTokens: 3, ToolCalls: []string{"product_search"}, LatencyMs: 9},
	})
	if got == nil || len(got.GetProducts()) != 1 || got.GetProducts()[0].GetVariantId() != "v1" {
		t.Fatalf("products = %#v", got)
	}
	if got.GetUsage().GetProvider() != "deepseek" || got.GetUsage().GetToolCallsJson() != `["product_search"]` {
		t.Fatalf("usage = %#v", got.GetUsage())
	}
}

func TestToProtoFiltersThinking(t *testing.T) {
	if got := toProto(agent.Event{Kind: agent.KindThinking, Content: "internal"}); got != nil {
		t.Fatalf("thinking event mapped: %#v", got)
	}
}

func TestToProtoSanitizesProviderError(t *testing.T) {
	got := toProto(agent.Event{Kind: agent.KindError, Err: "https://secret.example/key=abc"})
	if got.GetError() != "javob berishda xato yuz berdi (LLM)" {
		t.Fatalf("error = %q", got.GetError())
	}
	tool := toProto(agent.Event{Kind: agent.KindError, Tool: "product_search", Err: "katalog xatosi"})
	if tool.GetError() != "katalog xatosi" {
		t.Fatalf("tool error = %q", tool.GetError())
	}
}
