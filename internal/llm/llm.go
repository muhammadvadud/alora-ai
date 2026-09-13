// Package llm turli LLM provayderlarni yagona interfeys ortida beradi.
// Har provider alohida faylda init() orqali registry'ga qo'shiladi —
// yangi provider qo'shish = yangi fayl, mavjudlarini o'chirish = faylni o'chirish.
package llm

import (
	"context"
	"fmt"
	"sort"

	"github.com/cloudwego/eino/components/model"

	"github.com/muhammadvadud/alora-agent/internal/config"
)

// Builder sozlamalardan tool chaqira oladigan chat modelini quradi.
type Builder func(ctx context.Context, cfg config.LLMConfig) (model.ToolCallingChatModel, error)

var registry = map[string]Builder{}

// register provayderni ro'yxatga qo'shadi (provider fayllarining init() da chaqiriladi).
func register(name string, b Builder) {
	registry[name] = b
}

// New tanlangan provider uchun chat modelini quradi.
func New(ctx context.Context, cfg config.LLMConfig) (model.ToolCallingChatModel, error) {
	build, ok := registry[cfg.Provider]
	if !ok {
		return nil, fmt.Errorf("noma'lum LLM provider %q (mavjud: %v)", cfg.Provider, Providers())
	}
	m, err := build(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("%s modelini yaratish: %w", cfg.Provider, err)
	}
	return m, nil
}

// Providers ro'yxatga olingan provayderlar nomlarini qaytaradi.
func Providers() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
