package backend

import (
	"context"
	"testing"
	"time"

	"github.com/muhammadvadud/alora-agent/gen/alorav1"
	"github.com/muhammadvadud/alora-agent/internal/agent"
	"github.com/muhammadvadud/alora-agent/internal/session"
	"google.golang.org/grpc"
)

func TestOutboxStoreRetriesAndDeadAfterTenFailures(t *testing.T) {
	store := session.NewMemoryOutbox()
	if err := store.Enqueue(context.Background(), "unknown", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for attempt := 0; attempt < 10; attempt++ {
		items, err := store.Claim(context.Background(), 50, now)
		if err != nil || len(items) != 1 {
			t.Fatalf("claim %d: items=%d err=%v", attempt, len(items), err)
		}
		if err := store.Fail(context.Background(), items[0].ID, now, attempt == 9); err != nil {
			t.Fatal(err)
		}
	}
	pending, dead, err := store.Pending(context.Background())
	if err != nil || pending != 0 || dead != 1 {
		t.Fatalf("pending=%d dead=%d err=%v", pending, dead, err)
	}
}

func TestRetryDelay(t *testing.T) {
	for _, tc := range []struct {
		attempts int
		want     time.Duration
	}{
		{1, time.Second}, {2, 5 * time.Second}, {3, 30 * time.Second}, {4, 2 * time.Minute}, {5, 10 * time.Minute}, {20, 10 * time.Minute},
	} {
		if got := retryDelay(tc.attempts); got != tc.want {
			t.Errorf("attempts=%d: got %v want %v", tc.attempts, got, tc.want)
		}
	}
}

type fakeBackendClient struct {
	fail        bool
	usage, logs int
}

func (f *fakeBackendClient) ReportUsage(context.Context, *alorav1.UsageRequest, ...grpc.CallOption) (*alorav1.Ack, error) {
	if f.fail {
		return nil, context.DeadlineExceeded
	}
	f.usage++
	return &alorav1.Ack{Success: true}, nil
}
func (f *fakeBackendClient) SaveOrderDraft(context.Context, *alorav1.OrderDraftRequest, ...grpc.CallOption) (*alorav1.OrderDraftResponse, error) {
	return &alorav1.OrderDraftResponse{Success: true}, nil
}
func (f *fakeBackendClient) LogMessage(context.Context, *alorav1.MessageLogRequest, ...grpc.CallOption) (*alorav1.Ack, error) {
	if f.fail {
		return nil, context.DeadlineExceeded
	}
	f.logs++
	return &alorav1.Ack{Success: true}, nil
}

func TestOutboxWorkerAckAfterBackendReturns(t *testing.T) {
	store := session.NewMemoryOutbox()
	client := &fakeBackendClient{fail: true}
	outbox := NewOutbox(store, client, OutboxOptions{Clock: fixedClock{when: time.Unix(100, 0)}})
	if err := outbox.ReportUsage(context.Background(), agent.Usage{Provider: "test"}, UsageIDs{SessionID: "s1"}); err != nil {
		t.Fatal(err)
	}
	if err := outbox.process(context.Background()); err != nil {
		t.Fatal(err)
	}
	pending, _, _ := store.Pending(context.Background())
	if pending != 1 {
		t.Fatalf("pending after unavailable backend = %d", pending)
	}
	client.fail = false
	// Retry jadvali keyingi urinishni kelajak vaqtiga qo'yadi; test workerning
	// ack yo'lini alohida yangi yozuv bilan tekshiradi.
	if err := outbox.LogMessage(context.Background(), MessageLog{SessionID: "s1", Role: "assistant", Content: "salom"}); err != nil {
		t.Fatal(err)
	}
	if err := outbox.process(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.logs != 1 {
		t.Fatalf("logs sent = %d", client.logs)
	}
}

type fixedClock struct{ when time.Time }

func (c fixedClock) Now() time.Time { return c.when }
