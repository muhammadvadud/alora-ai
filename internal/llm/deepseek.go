package llm

import (
	"context"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-agent/internal/config"
)

func init() {
	register("deepseek", buildDeepSeek)
}

func buildDeepSeek(ctx context.Context, cfg config.LLMConfig) (model.ToolCallingChatModel, error) {
	c := &deepseek.ChatModelConfig{
		APIKey:      cfg.APIKey,
		Model:       cfg.Model,
		BaseURL:     cfg.BaseURL,
		MaxTokens:   cfg.MaxTokens,
		Temperature: cfg.Temperature,
		Timeout:     cfg.Timeout,
	}
	// Bo'sh bo'lsa yubormaymiz — provider standarti qoladi.
	if cfg.Thinking != "" {
		c.ThinkingConfig = &deepseek.ThinkingConfig{Type: cfg.Thinking}
	}
	m, err := deepseek.NewChatModel(ctx, c)
	if err != nil {
		return nil, err
	}

	// reasoning_effort ni eino'ning deepseek adapteri hali birinchi darajali maydon
	// sifatida bilmaydi — shu sabab so'rov tanasiga WithExtraFields orqali qo'shamiz.
	// U so'rov darajasidagi option bo'lgani uchun modelni o'raymiz: shunda daraja har
	// chaqiruvda avtomatik ketadi va agent/main qatlamlari bundan bexabar qoladi.
	if cfg.ReasoningEffort != "" {
		opt := deepseek.WithExtraFields(map[string]any{"reasoning_effort": cfg.ReasoningEffort})
		return &modelWithOptions{inner: m, opts: []model.Option{opt}}, nil
	}
	return m, nil
}

// modelWithOptions — har Generate/Stream chaqiruviga standart option'larni qo'shadigan o'ram.
type modelWithOptions struct {
	inner model.ToolCallingChatModel
	opts  []model.Option
}

func (m *modelWithOptions) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return m.inner.Generate(ctx, in, m.merge(opts)...)
}

func (m *modelWithOptions) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return m.inner.Stream(ctx, in, m.merge(opts)...)
}

// WithTools yangi model qaytaradi — o'ramni saqlab qolamiz, aks holda tool'lar
// bog'langach standart option'lar yo'qolardi.
func (m *modelWithOptions) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	inner, err := m.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &modelWithOptions{inner: inner, opts: m.opts}, nil
}

// merge: chaqiruvchi option'lari oxirida — kerak bo'lsa standartni ustidan yozadi.
func (m *modelWithOptions) merge(opts []model.Option) []model.Option {
	out := make([]model.Option, 0, len(m.opts)+len(opts))
	out = append(out, m.opts...)
	return append(out, opts...)
}

// Reasoning modelning "o'ylash" matnini qaytaradi (bo'lmasa ok=false).
//
// Reasoning provider'ga xos joyda (message.Extra) keladi — shu sabab uni O'QISH shu
// yerda, llm paketida turadi: agent qatlami provider'ni bilmasligi kerak.
func Reasoning(msg *schema.Message) (string, bool) {
	return deepseek.GetReasoningContent(msg)
}
