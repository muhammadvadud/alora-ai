package grpc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/muhammadvadud/alora-ai/gen/alorav1"
	"github.com/muhammadvadud/alora-ai/internal/agent"
	"github.com/muhammadvadud/alora-ai/internal/backend"
	"github.com/muhammadvadud/alora-ai/internal/curtains"
	"github.com/muhammadvadud/alora-ai/internal/session"
	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type fakeStreamer struct {
	events []agent.Event
	reset  bool
}

func (f *fakeStreamer) Stream(ctx context.Context, _, _ string) <-chan agent.Event {
	out := make(chan agent.Event, len(f.events))
	go func() {
		defer close(out)
		for _, event := range f.events {
			select {
			case out <- event:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func (f *fakeStreamer) Reset(context.Context, string) error { f.reset = true; return nil }

type fakeChatStream struct {
	ctx    context.Context
	mu     sync.Mutex
	events []*alorav1.ChatEvent
}

func (s *fakeChatStream) Send(event *alorav1.ChatEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}
func (s *fakeChatStream) SetHeader(metadata.MD) error  { return nil }
func (s *fakeChatStream) SendHeader(metadata.MD) error { return nil }
func (s *fakeChatStream) SetTrailer(metadata.MD)       {}
func (s *fakeChatStream) Context() context.Context     { return s.ctx }
func (s *fakeChatStream) SendMsg(any) error            { return nil }
func (s *fakeChatStream) RecvMsg(any) error            { return nil }

func TestServerChatFiltersThinkingAndPreservesOrder(t *testing.T) {
	fake := &fakeStreamer{events: []agent.Event{
		{Kind: agent.KindStart}, {Kind: agent.KindThinking, Content: "maxfiy"},
		{Kind: agent.KindContent, Content: "salom"}, {Kind: agent.KindDone},
	}}
	srv := New(fake, Options{})
	stream := &fakeChatStream{ctx: context.Background()}
	err := srv.Chat(&alorav1.ChatRequest{SessionId: "s1", UserId: "u1", TenantId: "t1", Locale: "uz", Message: "salom", Context: map[string]string{"role": "manager", "customer_mode": "1"}}, stream)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(stream.events) != 3 || stream.events[1].GetContent() != "salom" {
		t.Fatalf("unexpected events: %#v", stream.events)
	}
	info := requestInfo(&alorav1.ChatRequest{UserId: "u1", TenantId: "t1", Locale: "uz", Context: map[string]string{"role": "manager", "customer_mode": "1", "conversation_id": "c1"}})
	if info.Role != "manager" || !info.CustomerMode || info.ConversationID != "c1" {
		t.Fatalf("request info = %#v", info)
	}
}

type recordingReporter struct {
	logs  []backend.MessageLog
	usage []agent.Usage
}

func (r *recordingReporter) ReportUsage(_ context.Context, usage agent.Usage, _ backend.UsageIDs) error {
	r.usage = append(r.usage, usage)
	return nil
}
func (r *recordingReporter) LogMessage(_ context.Context, message backend.MessageLog) error {
	r.logs = append(r.logs, message)
	return nil
}

func TestServerChatReportsMessagesAndUsage(t *testing.T) {
	reporter := &recordingReporter{}
	fake := &fakeStreamer{events: []agent.Event{{Kind: agent.KindContent, Content: "javob"}, {Kind: agent.KindDone, Usage: &agent.Usage{PromptTokens: 3, CompletionTokens: 2}}}}
	srv := New(fake, Options{Reporter: reporter})
	stream := &fakeChatStream{ctx: context.Background()}
	req := &alorav1.ChatRequest{SessionId: "s1", UserId: "u1", TenantId: "t1", Message: "savol", Context: map[string]string{"conversation_id": "c1", "message_id": "m1"}}
	if err := srv.Chat(req, stream); err != nil {
		t.Fatal(err)
	}
	if len(reporter.logs) != 2 || reporter.logs[0].Role != "user" || reporter.logs[1].Role != "assistant" {
		t.Fatalf("logs = %#v", reporter.logs)
	}
	if reporter.logs[1].Content != "javob" || len(reporter.usage) != 1 || reporter.usage[0].CompletionTokens != 2 {
		t.Fatalf("reporter = %#v, %#v", reporter.logs, reporter.usage)
	}
}

func TestHealthDegradedWhenNotReady(t *testing.T) {
	srv := New(&fakeStreamer{}, Options{Ready: fakeReady{ok: false, reason: "katalog"}, Version: "test"})
	got, err := srv.Health(context.Background(), &alorav1.Empty{})
	if err != nil || got.GetStatus() != "degraded" || got.GetLastError() != "katalog" {
		t.Fatalf("health = %#v, err=%v", got, err)
	}
}

func TestAuthAndRateLimit(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-service-token", "secret"))
	if !authorized(ctx, "/alora.v1.Agent/Chat", "secret", true) {
		t.Fatal("valid token rejected")
	}
	if authorized(context.Background(), "/alora.v1.Agent/Chat", "secret", true) {
		t.Fatal("missing token accepted")
	}
	limiter := newRateLimiter()
	for i := 0; i < 20; i++ {
		if !limiter.allow("s") {
			t.Fatal("request rejected before limit")
		}
	}
	if limiter.allow("s") {
		t.Fatal("21st request accepted")
	}
}

func TestRecoveryUnary(t *testing.T) {
	_, err := recoveryUnary(context.Background(), nil, &grpcgo.UnaryServerInfo{FullMethod: "x"}, func(context.Context, any) (any, error) {
		panic("secret")
	})
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v", status.Code(err))
	}
}

func TestResetSessionChecksOwner(t *testing.T) {
	store := session.NewMemory()
	ownerCtx := curtains.WithUser(context.Background(), curtains.UserInfo{UserID: "owner"})
	if err := store.Append(ownerCtx, "s1", schema.UserMessage("salom")); err != nil {
		t.Fatal(err)
	}
	srv := New(&fakeStreamer{}, Options{Store: store})
	_, err := srv.ResetSession(context.Background(), &alorav1.ResetRequest{SessionId: "s1", UserId: "other"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("code = %v", status.Code(err))
	}
	if _, err := srv.ResetSession(context.Background(), &alorav1.ResetRequest{SessionId: "s1", UserId: "owner"}); err != nil {
		t.Fatal(err)
	}
}

func TestChatRejectsEmptyAndLongMessage(t *testing.T) {
	srv := New(&fakeStreamer{}, Options{MaxMessageChars: 3})
	stream := &fakeChatStream{ctx: context.Background()}
	for _, message := range []string{"", "abcd"} {
		err := srv.Chat(&alorav1.ChatRequest{SessionId: "s", UserId: "u", Message: message}, stream)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("message %q code = %v", message, status.Code(err))
		}
	}
}

// TestChatRejectsEmptyUserID — bo'sh (yoki bo'sh bo'luq) user_id bilan Chat rad etiladi:
// egaliksiz so'rov sessiya izolyatsiyasini buzardi (bo'sh user_id egasiz deb qaralardi).
func TestChatRejectsEmptyUserID(t *testing.T) {
	srv := New(&fakeStreamer{}, Options{})
	stream := &fakeChatStream{ctx: context.Background()}
	for _, userID := range []string{"", "   "} {
		err := srv.Chat(&alorav1.ChatRequest{SessionId: "s", UserId: userID, Message: "salom"}, stream)
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("user_id %q code = %v", userID, status.Code(err))
		}
	}
	if err := srv.Chat(&alorav1.ChatRequest{SessionId: "s", UserId: "u", Message: "salom"}, stream); err != nil {
		t.Fatalf("user_id to'ldirilgan so'rov rad etildi: %v", err)
	}
}

func TestServerSessionLimit(t *testing.T) {
	blocking := &blockingStreamer{release: make(chan struct{})}
	srv := New(blocking, Options{MaxSessions: 1})
	first := &fakeChatStream{ctx: context.Background()}
	done := make(chan error, 1)
	go func() { done <- srv.Chat(&alorav1.ChatRequest{SessionId: "s1", UserId: "u", Message: "x"}, first) }()
	for i := 0; i < 100 && srv.active.Load() == 0; i++ {
		time.Sleep(time.Millisecond)
	}
	second := srv.Chat(&alorav1.ChatRequest{SessionId: "s2", UserId: "u", Message: "x"}, &fakeChatStream{ctx: context.Background()})
	if status.Code(second) != codes.ResourceExhausted {
		t.Fatalf("code = %v", status.Code(second))
	}
	close(blocking.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

type blockingStreamer struct{ release chan struct{} }

func (b *blockingStreamer) Stream(ctx context.Context, _, _ string) <-chan agent.Event {
	out := make(chan agent.Event)
	if b.release == nil {
		b.release = make(chan struct{})
	}
	go func() {
		defer close(out)
		select {
		case <-b.release:
		case <-ctx.Done():
		}
	}()
	return out
}
func (b *blockingStreamer) Reset(context.Context, string) error { return nil }

type fakeReady struct {
	ok     bool
	reason string
}

func (f fakeReady) Ready() (bool, string) { return f.ok, f.reason }
