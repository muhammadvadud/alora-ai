package session

import "context"

type ctxKey struct{}

// WithID sessiya ID'sini context'ga joylaydi (tool'lar joriy sessiyani bilishi uchun).
func WithID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// IDFromContext context'dagi sessiya ID'sini qaytaradi (bo'lmasa bo'sh satr).
func IDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}
