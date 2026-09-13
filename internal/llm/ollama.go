package llm

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino/components/model"

	"github.com/muhammadvadud/alora-ai/internal/config"
)

func init() {
	register("ollama", buildOllama)
}

func buildOllama(ctx context.Context, cfg config.LLMConfig) (model.ToolCallingChatModel, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	return ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: baseURL,
		Model:   cfg.Model,
		Timeout: cfg.Timeout,
	})
}
