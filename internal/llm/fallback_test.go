package llm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
)

func TestIsRetryable(t *testing.T) {
	var timeout net.Error = timeoutError{}
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"deadline", context.DeadlineExceeded, true},
		{"wrapped deadline", fmt.Errorf("request: %w", context.DeadlineExceeded), true},
		{"network timeout", timeout, true},
		{"503", errors.New("upstream returned 503"), true},
		{"rate limit", errors.New("rate limit exceeded"), true},
		{"400", errors.New("bad request 400"), false},
		{"auth", errors.New("401 unauthorized"), false},
		{"canceled", context.Canceled, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Fatalf("IsRetryable(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "network failure" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }
