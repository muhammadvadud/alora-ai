package llm

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"

	"github.com/muhammadvadud/alora-agent/internal/config"
)

func init() {
	register("openai", buildOpenAI)
}

func buildOpenAI(ctx context.Context, cfg config.LLMConfig) (model.ToolCallingChatModel, error) {
	oc := &openai.ChatModelConfig{
		APIKey:      cfg.APIKey,
		Model:       cfg.Model,
		Timeout:     cfg.Timeout,
		Temperature: &cfg.Temperature,
	}
	if cfg.BaseURL != "" {
		oc.BaseURL = cfg.BaseURL
	}
	if cfg.MaxTokens > 0 {
		oc.MaxTokens = &cfg.MaxTokens
	}
	return openai.NewChatModel(ctx, oc)
}
