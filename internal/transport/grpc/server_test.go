package grpc

import (
	"context"
	"sync"
	"testing"

	"github.com/muhammadvadud/alora-agent/gen/alorav1"
	"github.com/muhammadvadud/alora-agent/internal/agent"
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
	err := srv.Chat(&alorav1.ChatRequest{SessionId: "s1", UserId: "u1", TenantId: "t1", Locale: "uz", Context: map[string]string{"role": "manager", "customer_mode": "1"}}, stream)
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

type fakeReady struct {
	ok     bool
	reason string
}

func (f fakeReady) Ready() (bool, string) { return f.ok, f.reason }
