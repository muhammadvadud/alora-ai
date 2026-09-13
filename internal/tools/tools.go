// Package tools agent uchun tool'larni beradi (yupqa adapter qatlami).
//
// Tuzilma: dto.go — kirish/chiqish struct'lari; <tool>.go — handler va konstruktor.
// Registratsiya main.go da (composition root). Domen mantiq internal/curtains da,
// hisob — Django (curtains.Calculator, ADR-009). gen/ importi bu paketda TAQIQ (rules §2).
package tools

import (
	"context"

	"github.com/cloudwego/eino/components/tool"

	"github.com/muhammadvadud/alora-agent/internal/curtains"
)

// StateStore — sessiya holatini o'qish/yozish (iste'molchi tomonda e'lon qilingan, rules §3.11).
// session.Store buni amalga oshiradi (1.30); testlarda in-memory.
type StateStore interface {
	State(ctx context.Context, sessionID string) (curtains.SessionState, error)
	SetState(ctx context.Context, sessionID string, st curtains.SessionState) error
}

// Deps — tool'lar uchun bog'liqliklar (main.go beradi). Global holat yo'q.
type Deps struct {
	Catalog *curtains.Catalog
	Calc    curtains.Calculator // Django CalcPreview klienti
	State   StateStore
	Clock   curtains.Clock
}

// Alora — barcha domen tool'lari bir joyda (main.go shu ro'yxatni register qiladi).
// Tartib muhim emas; nomlar rules §5.3 bo'yicha o'zgarmas.
func Alora(d Deps) []tool.BaseTool {
	return []tool.BaseTool{
		ProductSearch(d),
		CurtainCalculate(d),
		PriceQuote(d),
		SetContext(d),
	}
}

// AloraLookups — fason va bilim qidiruv tool'lari. Alora() ga qo'shilmaydi: o'sha
// ro'yxatning yaxlitligini TestAlora_ConstructorsAndNames tekshiradi, main.go esa
// knowledge_lookup'ni faqat qoidalar inline_max dan oshganda qo'shishi kerak
// (rules §4.6.2) — registratsiyani shart bilan main.go hal qiladi.
func AloraLookups(d Deps) []tool.BaseTool {
	return []tool.BaseTool{
		FasonLookup(d),
		KnowledgeLookup(d),
	}
}

// must — InferTool xatosi handler imzosidagi DASTURLASH xatosi (runtime'da yuz bermaydi),
// shuning uchun konstruktorlarni soddalashtirish uchun panic qilamiz (regexp.MustCompile uslubi).
func must(t tool.BaseTool, err error) tool.BaseTool {
	if err != nil {
		panic("tools: noto'g'ri tool ta'rifi: " + err.Error())
	}
	return t
}
