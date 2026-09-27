package llm

import (
	"context"
	"errors"
	"net"
	"strings"
	"syscall"
)

// IsRetryable ajratib beradi: provayder vaqtincha ishlamagan bo'lsa qayta urinish
// mumkin, autentifikatsiya yoki bekor qilingan so'rovda esa navbatni cho'zmaymiz.
func IsRetryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	// Ulanish uzilishi (TCP reset/yopilgan) — so'rov provayderga yetmagan, qayta yuborish
	// xavfsiz. SDK xatoni ko'pincha matnga o'raydi, shuning uchun markerlar ham tekshiriladi
	// (2026-09-27 live: DeepSeek "connection reset by peer" — navbat retry'siz yiqildi).
	if errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{"429", "500", "502", "503", "504", "rate limit", "overloaded", "timeout", "connection reset", "broken pipe", "unexpected eof"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}
