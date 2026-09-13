package grpc

import (
	"encoding/json"

	"github.com/muhammadvadud/alora-ai/gen/alorav1"
	"github.com/muhammadvadud/alora-ai/internal/agent"
)

// toProto — pb mapping'ining yagona joyi; transport agent Event kontraktini o'zgartirmaydi.
func toProto(ev agent.Event) *alorav1.ChatEvent {
	if ev.Kind == agent.KindThinking {
		return nil
	}
	errText := ev.Err
	if ev.Kind == agent.KindError && ev.Tool == "" {
		errText = "javob berishda xato yuz berdi (LLM)"
	}
	out := &alorav1.ChatEvent{
		Kind: string(ev.Kind), Content: ev.Content, Tool: ev.Tool,
		Args: ev.Args, Result: ev.Result, Error: errText,
	}
	for _, product := range ev.Products {
		out.Products = append(out.Products, &alorav1.ProductRef{
			VariantId: product.VariantID, ProductName: product.ProductName,
			ColorName: product.ColorName, ColorHex: product.ColorHex,
			Price: product.Price, ImageUrl: product.ImageURL, Sku: product.SKU,
		})
	}
	if ev.Usage != nil {
		calls, _ := json.Marshal(ev.Usage.ToolCalls)
		out.Usage = &alorav1.Usage{
			Provider: ev.Usage.Provider, Model: ev.Usage.Model,
			PromptTokens: int32(ev.Usage.PromptTokens), CompletionTokens: int32(ev.Usage.CompletionTokens),
			CachedTokens: int32(ev.Usage.CachedTokens), ReasoningTokens: int32(ev.Usage.ReasoningTokens),
			ToolCallsJson: string(calls), LatencyMs: int32(ev.Usage.LatencyMs),
		}
	}
	return out
}
