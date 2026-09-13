package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync/atomic"
	"time"

	"github.com/muhammadvadud/alora-agent/gen/alorav1"
	"github.com/muhammadvadud/alora-agent/internal/agent"
	"github.com/muhammadvadud/alora-agent/internal/backend"
	"github.com/muhammadvadud/alora-agent/internal/curtains"
	"github.com/muhammadvadud/alora-agent/internal/observability"
	"github.com/muhammadvadud/alora-agent/internal/session"
	voicepkg "github.com/muhammadvadud/alora-agent/internal/voice"
	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

// Streamer — agent Chat'ni test va keyingi transportlar uchun kichik kontraktga ajratadi.
type Streamer interface {
	Stream(context.Context, string, string) <-chan agent.Event
	Reset(context.Context, string) error
}

type Reporter interface {
	ReportUsage(context.Context, agent.Usage, backend.UsageIDs) error
	LogMessage(context.Context, backend.MessageLog) error
}

type Options struct {
	ServiceToken       string
	AuthRequired       bool
	MaxSessions        int
	MaxMessageChars    int
	RateLimitPerMinute int
	Store              session.Store
	Version            string
	Ready              observability.Readiness
	Metrics            *observability.Metrics
	Health             func() (status string, lastErr string)
	Reporter           Reporter
	STT                voicepkg.STT
	TTS                voicepkg.TTS
	TTSVoice           string
	STTLang            string
}

type Server struct {
	alorav1.UnimplementedAgentServer
	chat            Streamer
	ready           observability.Readiness
	health          func() (string, string)
	reporter        Reporter
	metrics         *observability.Metrics
	stt             voicepkg.STT
	tts             voicepkg.TTS
	ttsVoice        string
	sttLang         string
	version         string
	maxSessions     int64
	maxMessageChars int
	store           session.Store
	active          atomic.Int64
}

func New(chat Streamer, opts Options) *Server {
	if opts.Ready == nil {
		opts.Ready = readyOK{}
	}
	if opts.Health == nil {
		opts.Health = func() (string, string) { return "ok", "" }
	}
	maxSessions := opts.MaxSessions
	if maxSessions <= 0 {
		maxSessions = 100
	}
	maxMessageChars := opts.MaxMessageChars
	if maxMessageChars <= 0 {
		maxMessageChars = 4000
	}
	return &Server{chat: chat, ready: opts.Ready, health: opts.Health, version: opts.Version, reporter: opts.Reporter, metrics: opts.Metrics,
		maxSessions: int64(maxSessions), maxMessageChars: maxMessageChars, store: opts.Store,
		stt: opts.STT, tts: opts.TTS, ttsVoice: opts.TTSVoice, sttLang: opts.STTLang}
}

func (s *Server) acquireSession() bool {
	for {
		active := s.active.Load()
		if active >= s.maxSessions {
			return false
		}
		if s.active.CompareAndSwap(active, active+1) {
			return true
		}
	}
}

func validateMessage(message string, maxChars int) error {
	if len([]rune(message)) == 0 {
		return status.Error(codes.InvalidArgument, "xabar bo'sh bo'lishi mumkin emas")
	}
	if len([]rune(message)) > maxChars {
		return status.Error(codes.InvalidArgument, "xabar juda uzun")
	}
	return nil
}

func (s *Server) Chat(req *alorav1.ChatRequest, stream grpcgo.ServerStreamingServer[alorav1.ChatEvent]) error {
	if s.chat == nil {
		return status.Error(codes.FailedPrecondition, "agent tayyor emas")
	}
	if err := validateMessage(req.GetMessage(), s.maxMessageChars); err != nil {
		return err
	}
	if !s.acquireSession() {
		return status.Error(codes.ResourceExhausted, "agent band (sessiya chegarasi)")
	}
	defer s.active.Add(-1)

	ctx := session.WithID(stream.Context(), req.GetSessionId())
	info := requestInfo(req)
	ctx = WithRequestInfo(ctx, info)
	ctx = curtains.WithUser(ctx, curtains.UserInfo{
		UserID: info.UserID, TenantID: info.TenantID, Role: curtains.UserRole(info.Role),
		Locale: info.Locale, CustomerMode: info.CustomerMode, ConversationID: info.ConversationID,
	})
	info = requestInfo(req)
	if s.reporter != nil {
		if err := s.reporter.LogMessage(ctx, backend.MessageLog{SessionID: req.GetSessionId(), ConversationID: info.ConversationID, Role: "user", Content: req.GetMessage()}); err != nil {
			slog.Error("foydalanuvchi xabarini outbox'ga qo'shib bo'lmadi", "error", err, "session_id", req.GetSessionId())
		}
	}
	var assistantContent string
	for ev := range s.chat.Stream(ctx, req.GetSessionId(), req.GetMessage()) {
		if ev.Kind == agent.KindThinking {
			continue
		}
		if ev.Kind == agent.KindContent {
			assistantContent += ev.Content
		}
		if ev.Kind == agent.KindError && ev.Tool == "" {
			slog.ErrorContext(ctx, "LLM javob xatosi", "error", ev.Err)
		}
		if ev.Kind == agent.KindToolResult && s.reporter != nil {
			if err := s.reporter.LogMessage(ctx, backend.MessageLog{SessionID: req.GetSessionId(), ConversationID: info.ConversationID, Role: "tool", Content: ev.Result, ToolResult: ev.Result}); err != nil {
				slog.Error("tool xabarini outbox'ga qo'shib bo'lmadi", "error", err, "session_id", req.GetSessionId())
			}
		}
		if ev.Kind == agent.KindDone && s.reporter != nil {
			if err := s.reporter.LogMessage(ctx, backend.MessageLog{SessionID: req.GetSessionId(), ConversationID: info.ConversationID, Role: "assistant", Content: assistantContent, TokensIn: usageIn(ev.Usage), TokensOut: usageOut(ev.Usage)}); err != nil {
				slog.Error("assistant xabarini outbox'ga qo'shib bo'lmadi", "error", err, "session_id", req.GetSessionId())
			}
			if ev.Usage != nil {
				ids := backend.UsageIDs{SessionID: req.GetSessionId(), UserID: info.UserID, TenantID: info.TenantID, ConversationID: info.ConversationID, MessageID: req.GetContext()["message_id"]}
				if err := s.reporter.ReportUsage(ctx, *ev.Usage, ids); err != nil {
					slog.Error("usage outbox'ga qo'shib bo'lmadi", "error", err, "session_id", req.GetSessionId())
				}
			}
		}
		mapped := toProto(ev)
		if mapped == nil {
			continue
		}
		if err := stream.Send(mapped); err != nil {
			return err
		}
	}
	return nil
}

func usageIn(usage *agent.Usage) int {
	if usage == nil {
		return 0
	}
	return usage.PromptTokens
}
func usageOut(usage *agent.Usage) int {
	if usage == nil {
		return 0
	}
	return usage.CompletionTokens
}

func requestInfo(req *alorav1.ChatRequest) RequestInfo {
	values := req.GetContext()
	return RequestInfo{
		UserID: req.GetUserId(), TenantID: req.GetTenantId(), Locale: req.GetLocale(),
		Role: values["role"], CustomerMode: values["customer_mode"] == "1",
		ConversationID: values["conversation_id"],
	}
}

func (s *Server) ResetSession(ctx context.Context, req *alorav1.ResetRequest) (*alorav1.ResetResponse, error) {
	if s.chat == nil {
		return nil, status.Error(codes.FailedPrecondition, "agent tayyor emas")
	}
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id bo'sh bo'lishi mumkin emas")
	}
	resetCtx := session.WithID(ctx, req.GetSessionId())
	if s.store != nil {
		owner, err := s.store.Owner(resetCtx, req.GetSessionId())
		if err != nil {
			return nil, status.Error(codes.Internal, "sessiya egasini tekshirib bo'lmadi")
		}
		if owner != "" && owner != req.GetUserId() {
			return nil, status.Error(codes.PermissionDenied, "sessiya sizga tegishli emas")
		}
	}
	if err := s.chat.Reset(resetCtx, req.GetSessionId()); err != nil {
		return nil, status.Error(codes.Internal, "sessiyani tozalab bo'lmadi")
	}
	return &alorav1.ResetResponse{Success: true}, nil
}

func (s *Server) Health(context.Context, *alorav1.Empty) (*alorav1.HealthResponse, error) {
	state, lastErr := s.health()
	ready, reason := s.ready.Ready()
	if !ready {
		state = "degraded"
		if lastErr == "" {
			lastErr = reason
		}
	}
	if state != "ok" {
		state = "degraded"
	}
	return &alorav1.HealthResponse{Status: state, Version: s.version, ActiveSessions: int32(s.active.Load()), LastError: lastErr}, nil
}

type readyOK struct{}

func (readyOK) Ready() (bool, string) { return true, "" }

type ServeConfig struct {
	ServiceToken       string
	AuthRequired       bool
	MaxSessions        int
	MaxMessageChars    int
	RateLimitPerMinute int
	Version            string
	Ready              observability.Readiness
	Metrics            *observability.Metrics
}

func Serve(ctx context.Context, addr string, srv *Server, cfg ServeConfig) error {
	maxSessions := cfg.MaxSessions
	if maxSessions <= 0 {
		maxSessions = 100
	}
	limiter := newRateLimiter(cfg.RateLimitPerMinute)
	server := grpcgo.NewServer(
		grpcgo.KeepaliveParams(keepalive.ServerParameters{Time: 30 * time.Second}),
		grpcgo.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{MinTime: 10 * time.Second, PermitWithoutStream: true}),
		grpcgo.MaxRecvMsgSize(4<<20), grpcgo.MaxSendMsgSize(4<<20),
		grpcgo.MaxConcurrentStreams(uint32(maxSessions)),
		grpcgo.ChainUnaryInterceptor(unaryInterceptors(interceptorOptions{
			token: cfg.ServiceToken, authRequired: cfg.AuthRequired, metrics: cfg.Metrics, limiter: limiter,
		})...),
		grpcgo.ChainStreamInterceptor(streamInterceptors(interceptorOptions{
			token: cfg.ServiceToken, authRequired: cfg.AuthRequired, metrics: cfg.Metrics, limiter: limiter,
		})...),
	)
	alorav1.RegisterAgentServer(server, srv)

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	select {
	case err := <-serveErr:
		if errors.Is(err, grpcgo.ErrServerStopped) {
			return nil
		}
		return fmt.Errorf("grpc server: %w", err)
	case <-ctx.Done():
		stopped := make(chan struct{})
		go func() { server.GracefulStop(); close(stopped) }()
		select {
		case <-stopped:
			return nil
		case <-time.After(15 * time.Second):
			server.Stop()
			return nil
		}
	}
}
