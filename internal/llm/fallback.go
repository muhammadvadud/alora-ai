package llm

import (
	"context"
	"errors"
	"net"
	"strings"
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
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{"429", "500", "502", "503", "504", "rate limit", "overloaded", "timeout"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}
