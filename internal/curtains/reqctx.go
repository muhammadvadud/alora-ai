package curtains

import "context"

// UserInfo — so'rov egasi haqida tool'larga kerak bo'lgan minimal ma'lumot
// (Django ChatRequest + context kalitlari, GO-CONTRACT §1.1). Mijoz PII bu yerda YO'Q.
type UserInfo struct {
	UserID         string
	TenantID       string
	Role           UserRole
	Locale         string
	MessageLocale  string // joriy xabar matnidan aniqlangan til (MessageLocale), bo'sh — noma'lum
	CustomerMode   bool
	ConversationID string
}

type userKey struct{}

// WithUser — transport qatlami so'rov kontekstiga foydalanuvchini qo'yadi.
func WithUser(ctx context.Context, u UserInfo) context.Context {
	return context.WithValue(ctx, userKey{}, u)
}

// UserFromContext — tool'lar rol/rejimni shu yerdan oladi; yo'q bo'lsa bo'sh (rol = manager
// chegarasi EMAS — price_quote bo'sh rolni employee kabi qat'iy chegara bilan ko'radi).
func UserFromContext(ctx context.Context) (UserInfo, bool) {
	u, ok := ctx.Value(userKey{}).(UserInfo)
	return u, ok
}
