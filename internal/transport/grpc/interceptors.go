package grpc

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/muhammadvadud/alora-agent/gen/alorav1"
	"github.com/muhammadvadud/alora-agent/internal/observability"
	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type interceptorOptions struct {
	token        string
	authRequired bool
	metrics      *observability.Metrics
	limiter      *rateLimiter
}

type requestIDContextKey struct{}

func unaryInterceptors(o interceptorOptions) []grpcgo.UnaryServerInterceptor {
	return []grpcgo.UnaryServerInterceptor{
		recoveryUnary,
		requestIDUnary,
		authUnary(o),
		loggingUnaryInterceptor(o),
		metricsUnary(o),
		o.limiter.unary,
	}
}

func streamInterceptors(o interceptorOptions) []grpcgo.StreamServerInterceptor {
	return []grpcgo.StreamServerInterceptor{
		recoveryStream,
		requestIDStream,
		authStream(o),
		loggingStreamInterceptor(o),
		metricsStream(o),
		o.limiter.stream,
	}
}

func requestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func metadataValue(ctx context.Context, key string) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func withRequestID(ctx context.Context) context.Context {
	id := metadataValue(ctx, "x-request-id")
	if id == "" {
		id = requestID()
	}
	ctx = context.WithValue(ctx, requestIDContextKey{}, id)
	return observability.WithRequest(ctx, id, sessionIDFromContext(ctx), "")
}

func sessionIDFromContext(ctx context.Context) string {
	return metadataValue(ctx, "x-session-id")
}

func recoveryUnary(ctx context.Context, req any, info *grpcgo.UnaryServerInfo, handler grpcgo.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "grpc panic", "method", info.FullMethod, "panic", r)
			err = status.Error(codes.Internal, "ichki server xatosi")
		}
	}()
	return handler(ctx, req)
}

func recoveryStream(srv any, ss grpcgo.ServerStream, info *grpcgo.StreamServerInfo, handler grpcgo.StreamHandler) (err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("grpc stream panic", "method", info.FullMethod, "panic", r)
			err = status.Error(codes.Internal, "ichki server xatosi")
		}
	}()
	return handler(srv, ss)
}

func requestIDUnary(ctx context.Context, req any, info *grpcgo.UnaryServerInfo, handler grpcgo.UnaryHandler) (any, error) {
	return handler(withRequestID(ctx), req)
}

type wrappedStream struct {
	grpcgo.ServerStream
	ctx context.Context
}

func (s *wrappedStream) Context() context.Context { return s.ctx }

func (s *wrappedStream) RecvMsg(msg any) error {
	if err := s.ServerStream.RecvMsg(msg); err != nil {
		return err
	}
	if req, ok := msg.(*alorav1.ChatRequest); ok {
		id, _ := s.ctx.Value(requestIDContextKey{}).(string)
		s.ctx = observability.WithRequest(s.ctx, id, req.GetSessionId(), req.GetUserId())
	}
	return nil
}

func requestIDStream(srv any, ss grpcgo.ServerStream, info *grpcgo.StreamServerInfo, handler grpcgo.StreamHandler) error {
	return handler(srv, &wrappedStream{ServerStream: ss, ctx: withRequestID(ss.Context())})
}

func authorized(ctx context.Context, method, token string, required bool) bool {
	if strings.HasSuffix(method, "/Health") {
		return true
	}
	provided := metadataValue(ctx, "x-service-token")
	if token == "" {
		return !required && provided == ""
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(token)) == 1
}

func authUnary(o interceptorOptions) grpcgo.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpcgo.UnaryServerInfo, handler grpcgo.UnaryHandler) (any, error) {
		if !authorized(ctx, info.FullMethod, o.token, o.authRequired) {
			return nil, status.Error(codes.Unauthenticated, "xizmat autentifikatsiyasi talab qilinadi")
		}
		return handler(ctx, req)
	}
}

func authStream(o interceptorOptions) grpcgo.StreamServerInterceptor {
	return func(srv any, ss grpcgo.ServerStream, info *grpcgo.StreamServerInfo, handler grpcgo.StreamHandler) error {
		if !authorized(ss.Context(), info.FullMethod, o.token, o.authRequired) {
			return status.Error(codes.Unauthenticated, "xizmat autentifikatsiyasi talab qilinadi")
		}
		return handler(srv, ss)
	}
}

func loggingUnary(ctx context.Context, req any, info *grpcgo.UnaryServerInfo, handler grpcgo.UnaryHandler) (any, error) {
	started := time.Now()
	resp, err := handler(ctx, req)
	observability.Logger(ctx).Info("grpc request", "method", info.FullMethod, "latency_ms", time.Since(started).Milliseconds(), "code", status.Code(err).String())
	return resp, err
}

func loggingUnaryInterceptor(_ interceptorOptions) grpcgo.UnaryServerInterceptor {
	return loggingUnary
}

func loggingStream(srv any, ss grpcgo.ServerStream, info *grpcgo.StreamServerInfo, handler grpcgo.StreamHandler) error {
	started := time.Now()
	err := handler(srv, ss)
	observability.Logger(ss.Context()).Info("grpc request", "method", info.FullMethod, "latency_ms", time.Since(started).Milliseconds(), "code", status.Code(err).String())
	return err
}

func loggingStreamInterceptor(_ interceptorOptions) grpcgo.StreamServerInterceptor {
	return loggingStream
}

func metricsUnary(o interceptorOptions) grpcgo.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpcgo.UnaryServerInfo, handler grpcgo.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if o.metrics != nil {
			o.metrics.ChatRequests.WithLabelValues(status.Code(err).String()).Inc()
		}
		return resp, err
	}
}

func metricsStream(o interceptorOptions) grpcgo.StreamServerInterceptor {
	return func(srv any, ss grpcgo.ServerStream, info *grpcgo.StreamServerInfo, handler grpcgo.StreamHandler) error {
		err := handler(srv, ss)
		if o.metrics != nil {
			o.metrics.ChatRequests.WithLabelValues(status.Code(err).String()).Inc()
		}
		return err
	}
}

type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]rateEntry
}

type rateEntry struct {
	started time.Time
	count   int
}

func newRateLimiter() *rateLimiter { return &rateLimiter{entries: make(map[string]rateEntry)} }

func (r *rateLimiter) allow(key string) bool {
	if key == "" {
		key = "anonymous"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	e := r.entries[key]
	if e.started.IsZero() || now.Sub(e.started) >= time.Minute {
		r.entries[key] = rateEntry{started: now, count: 1}
		return true
	}
	if e.count >= 20 {
		return false
	}
	e.count++
	r.entries[key] = e
	return true
}

func (r *rateLimiter) unary(ctx context.Context, req any, info *grpcgo.UnaryServerInfo, handler grpcgo.UnaryHandler) (any, error) {
	if !r.allow(sessionKey(ctx, req)) {
		return nil, status.Error(codes.ResourceExhausted, "sessiya so'rovlar chegarasiga yetdi")
	}
	return handler(ctx, req)
}

func (r *rateLimiter) stream(srv any, ss grpcgo.ServerStream, info *grpcgo.StreamServerInfo, handler grpcgo.StreamHandler) error {
	return handler(srv, &rateStream{ServerStream: ss, limiter: r, method: info.FullMethod})
}

type rateStream struct {
	grpcgo.ServerStream
	limiter *rateLimiter
	method  string
	checked bool
}

func (s *rateStream) RecvMsg(msg any) error {
	if err := s.ServerStream.RecvMsg(msg); err != nil {
		return err
	}
	if s.checked {
		return nil
	}
	s.checked = true
	if !s.limiter.allow(sessionKey(s.Context(), msg)) {
		return status.Error(codes.ResourceExhausted, "sessiya so'rovlar chegarasiga yetdi")
	}
	return nil
}

func sessionKey(ctx context.Context, req any) string {
	if chatReq, ok := req.(*alorav1.ChatRequest); ok {
		return chatReq.GetSessionId()
	}
	if resetReq, ok := req.(*alorav1.ResetRequest); ok {
		return resetReq.GetSessionId()
	}
	return metadataValue(ctx, "x-session-id")
}
