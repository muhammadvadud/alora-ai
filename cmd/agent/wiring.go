package main

import (
	"context"
	"encoding/json"

	"github.com/muhammadvadud/alora-agent/internal/curtains"
	"github.com/muhammadvadud/alora-agent/internal/prompt"
	"github.com/muhammadvadud/alora-agent/internal/session"
)

// stateHook faqat event uchun o'qiydi: state'ni tool'lar o'zi yozadi, shuning
// uchun agent qatlamida domen Apply qilish ikki marta yozishga olib kelmaydi.
type stateHook struct{ store session.Store }

func (h stateHook) OnToolCall(context.Context, string, string, string) {}

func (h stateHook) Stage(ctx context.Context, sessionID string) (string, string) {
	st, err := h.store.State(ctx, sessionID)
	if err != nil {
		return "", ""
	}
	raw, err := json.Marshal(st.Summary())
	if err != nil {
		return st.Stage.Backend(), ""
	}
	return st.Stage.Backend(), string(raw)
}

func promptFunc(store session.Store, catalog *curtains.Catalog, fallback string, inlineMax int) func(context.Context, string) string {
	builder := prompt.Builder{Fallback: fallback}
	return func(ctx context.Context, sessionID string) string {
		st, err := store.State(ctx, sessionID)
		if err != nil {
			st = curtains.NewSessionState()
		}
		view := catalog.View()
		base, _ := view.Prompt()
		user, _ := curtains.UserFromContext(ctx)
		baseText := ""
		if base.Content != "" {
			baseText = base.Content
		}
		return builder.Build(prompt.Input{
			Base: baseText, State: st, Knowledge: view.Knowledge(), KnowledgeInlineMax: inlineMax,
			CatalogVersion: view.Version, CatalogStale: view.Stale, Locale: user.Locale, CustomerMode: user.CustomerMode,
		})
	}
}
