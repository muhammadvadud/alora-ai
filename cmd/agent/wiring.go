package main

import (
	"context"
	"encoding/json"

	"github.com/cloudwego/eino/components/tool"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
	"github.com/muhammadvadud/alora-ai/internal/prompt"
	"github.com/muhammadvadud/alora-ai/internal/session"
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
			CatalogVersion: view.Version, CatalogStale: view.Stale, Locale: user.Locale, MessageLocale: user.MessageLocale, CustomerMode: user.CustomerMode,
		})
	}
}

// stageTools — har bir bosqich uchun ruxsat etilgan tool to'plami (xarajat opt. A).
// Asosiy qoida: bosqichga tegishli bo'lmagan tool'ni modelga ko'rsatmaslik —
// schema qancha kichik bo'lsa, prompt token va (DeepSeek'da) kesh shuncha tez.
// Dizayn:
//   - need: foydalanuvchi hali nima istagani aniq emas — faqat kontekst yig'ish.
//   - recommend: katalog tavsiyasi (product_search) + o'lcham/xona tahlili.
//   - measure/calc/variants/choose: narx/parametr hisobi markazda — curtain_calculate.
//   - order: buyurtma oqimi — order_draft taklif/qoralamani beradi (ADR-020: PII
//     yig'maydi, tasdiq sotuvchida), narx/parametr tekshiruv uchun price_quote.
//
// Har bir to'plamga set_context qo'shilgan (xona/o'lcham ma'lumotlarini olish uchun)
// va knowledge_lookup faqat dastlabki bosqichlarda — kech bosqichda aniq narx-tavsiya
// ishlaydi. Muhim noziklik: stagesiz (noto'g'ri) sessiya xavfsiz default oladi.
func stageTools(stage string) []string {
	switch stage {
	case "recommend", "measure":
		return []string{"set_context", "product_search", "fason_lookup", "knowledge_lookup", "curtain_calculate", "build_options"}
	case "calc", "variants", "choose":
		return []string{"set_context", "product_search", "fason_lookup", "curtain_calculate", "price_quote", "build_options", "order_draft"}
	case "order":
		// order_draft FAQAT taklif/qoralamaning o'zini beradi — mijoz ma'lumotlari
		// shu yerda yig'ilmaydi, shuning uchun order bosqichida set_context ham
		// kerak emas (ADR-020).
		return []string{"order_draft", "price_quote", "curtain_calculate"}
	default:
		return []string{"set_context", "product_search", "knowledge_lookup", "fason_lookup", "curtain_calculate", "build_options"}
	}
}

// toolSelectorFor — sessiya Stage'iga qarab Stream uchun tool to'plamini tanlaydi.
// Selector per-Stream chaqiriladi, Chat xoslashgan to'plamni egallamaydi —
// bir xil Chat bir nechta sessiyaga parallel xizmat qiladi. Tanlab olingan tool
// nomi ro'yxatda bo'lmasa (yovuz yoki eski config) hatto yozuv emas, shunchaki
// o'tkazib yuboriladi — noto'g'ri to'plam butun so'rovni yiqitmasin.
func toolSelectorFor(store session.Store) func(ctx context.Context, sessionID string, all []tool.BaseTool) []tool.BaseTool {
	return func(ctx context.Context, sessionID string, all []tool.BaseTool) []tool.BaseTool {
		st, err := store.State(ctx, sessionID)
		if err != nil {
			st = curtains.NewSessionState()
		}
		allowed := stageTools(st.Stage.Backend())
		allowedSet := make(map[string]bool, len(allowed))
		for _, name := range allowed {
			allowedSet[name] = true
		}
		selected := make([]tool.BaseTool, 0, len(allowed))
		for _, tt := range all {
			info, infoErr := tt.Info(ctx)
			if infoErr != nil || !allowedSet[info.Name] {
				continue
			}
			selected = append(selected, tt)
		}
		return selected
	}
}
