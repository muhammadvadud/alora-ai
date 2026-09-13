package llm

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/components/model"

	"github.com/muhammadvadud/alora-ai/internal/config"
)

func init() {
	register("claude", buildClaude)
}

func buildClaude(ctx context.Context, cfg config.LLMConfig) (model.ToolCallingChatModel, error) {
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048 // Claude uchun MaxTokens majburiy
	}
	cc := &claude.Config{
		APIKey:      cfg.APIKey,
		Model:       cfg.Model,
		MaxTokens:   maxTokens,
		Temperature: &cfg.Temperature,
	}
	if cfg.BaseURL != "" {
		cc.BaseURL = &cfg.BaseURL
	}
	return claude.NewChatModel(ctx, cc)
}
