package tools

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/muhammadvadud/alora-agent/internal/curtains"
	"github.com/muhammadvadud/alora-agent/internal/session"
)

const (
	errCatalogNotReady = "katalog yuklanmadi, narx aytib bo'lmaydi"
	errCalcUnavailable = "hisob xizmati vaqtincha ishlamayapti"
	warnStale          = "narxlar yangilanmagan bo'lishi mumkin (katalog eskirgan)"
)

// displayMoney — "5382000" → "5 382 000 so'm". LLM sonni o'zi formatlamasin.
func displayMoney(d decimal.Decimal) string {
	s := d.Round(0).StringFixed(0)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String() + " so'm"
	}
	return b.String() + " so'm"
}

// displayTotals — Django natijasidan o'qiladigan summalar. Sonlar hisobdan, matn bu yerda.
func displayTotals(r curtains.CalcResult) map[string]string {
	return map[string]string{
		"subtotal": displayMoney(r.Subtotal),
		"discount": displayMoney(r.DiscountAmount) + " (" + r.DiscountPercent.StringFixed(0) + "%)",
		"delivery": displayMoney(r.DeliveryAmount),
		"total":    displayMoney(r.Total),
	}
}

// toAny — struct → JSON → any. Django natijasi LLM'ga aynan kelganicha ketadi (rules §4.2.4).
func toAny(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}

func dec(f float64) decimal.Decimal { return decimal.NewFromFloat(f) }

func decPtr(f float64) *decimal.Decimal {
	if f == 0 {
		return nil
	}
	d := decimal.NewFromFloat(f)
	return &d
}

// readyView — katalog tayyor bo'lmasa xato matni; stale bo'lsa ogohlantirish.
func readyView(d Deps) (*curtains.View, string, bool) {
	if d.Catalog == nil {
		return nil, errCatalogNotReady, false
	}
	v := d.Catalog.View()
	if !v.Ready() {
		return nil, errCatalogNotReady, false
	}
	return v, "", v.Stale
}

// loadState / saveState — sessiya holati (State nil bo'lsa — dev/CLI: holat saqlanmaydi).
func loadState(ctx context.Context, d Deps) (curtains.SessionState, string) {
	sid := session.IDFromContext(ctx)
	if d.State == nil || sid == "" {
		return curtains.NewSessionState(), sid
	}
	st, err := d.State.State(ctx, sid)
	if err != nil {
		return curtains.NewSessionState(), sid
	}
	return st, sid
}

func saveState(ctx context.Context, d Deps, sid string, st curtains.SessionState) {
	if d.State == nil || sid == "" {
		return
	}
	_ = d.State.SetState(ctx, sid, st) // xato chat'ni to'xtatmaydi; keyingi tool qayta yozadi
}

func now(d Deps) time.Time {
	if d.Clock == nil {
		return time.Now()
	}
	return d.Clock.Now()
}

// argsMap — SessionState.Apply uchun DTO'ni map'ga aylantiradi.
func argsMap(v any) map[string]any {
	m, _ := toAny(v).(map[string]any)
	return m
}

// userRole — kontekstda rol bo'lmasa employee (eng qat'iy chegara) — xavfsiz default.
func userRole(ctx context.Context) curtains.UserRole {
	if u, ok := curtains.UserFromContext(ctx); ok && u.Role != "" {
		return u.Role
	}
	return curtains.RoleEmployee
}
