// Package observability — slog sozlash, so'rov kontekst maydonlari va Prometheus metrikalari
// (rules.md §13). Bu paket kalitlar/telefon kabi maxfiy qiymatlarni qabul qiladigan API
// taqdim etmaydi — logga tushadigan yagona maydonlar ID'lar.
package observability

import (
	"context"
	"log/slog"
	"os"
)

// ctxKey — kontekst uchun xususiy kalit; boshqa paket bir xil kalit yozolmaydi,
// shuning uchun xato maydon qo'shib bo'lmaydi.
type ctxKey struct{}

// reqFields — WithRequest orqali kontekstga yozilgan, loggerga tushadigan maydonlar.
type reqFields struct {
	requestID string
	sessionID string
	userID    string
}

// SetupLogger JSON handler'li slog'ni stdout'ga o'rnatadi va default qiladi.
// Level springiz belgisi: debug|info|warn|error; tanib bo'lmasa info (yumshoq
// xato — noto'g'ri daraja ishni to'xtatmasligi kerak, rules §3.9).
func SetupLogger(level string) *slog.Logger {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
	slog.SetDefault(logger)
	return logger
}

// WithRequest transport qatlamidan kelgan ID'larni kontekstga yozadi. Keyingi log
// qatorlari shu ID'lar bilan chiqadi — bir so'rovni tarixda izlash uchun (rules §13.2).
// Bo'sh qiymatlar qo'shilmaydi, lekin kontekstga yozish bemalol (yagona joy).
func WithRequest(ctx context.Context, requestID, sessionID, userID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, &reqFields{
		requestID: requestID,
		sessionID: sessionID,
		userID:    userID,
	})
}

// Logger kontekstdagi so'rov maydonlari bilan boyitilgan loggerni qaytaradi.
// Kontekstda maydon bo'lmasa — default logger (masalan ishga tushirish qatorlari).
func Logger(ctx context.Context) *slog.Logger {
	f, ok := ctx.Value(ctxKey{}).(*reqFields)
	if !ok {
		return slog.Default()
	}
	return slog.Default().With(
		slog.String("request_id", f.requestID),
		slog.String("session_id", f.sessionID),
		slog.String("user_id", f.userID),
	)
}
