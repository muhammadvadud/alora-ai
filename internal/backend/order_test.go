package backend

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/muhammadvadud/alora-ai/gen/alorav1"
	"github.com/muhammadvadud/alora-ai/internal/curtains"
	"github.com/muhammadvadud/alora-ai/internal/session"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// orderFake — Django Backend.SaveOrderDraft o'rnida; chaqiruvlar soni va metadata
// tekshiriladi, mode bo'yicha xato/uzilish simulyatsiyasi.
type orderFake struct {
	alorav1.UnimplementedBackendServer
	mu       sync.Mutex
	calls    int
	mode     string
	response *alorav1.OrderDraftResponse
	metadata metadata.MD
}

func (f *orderFake) SaveOrderDraft(ctx context.Context, _ *alorav1.OrderDraftRequest) (*alorav1.OrderDraftResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.metadata, _ = metadata.FromIncomingContext(ctx)
	switch f.mode {
	case "unavailable":
		return nil, status.Error(codes.Unavailable, "backend down")
	case "deadline":
		return nil, status.Error(codes.DeadlineExceeded, "backend timeout")
	case "error":
		return &alorav1.OrderDraftResponse{Success: false, Error: "manzil to'liq emas"}, nil
	}
	if f.response != nil {
		return f.response, nil
	}
	return &alorav1.OrderDraftResponse{Success: true, OrderId: "order-1", OrderNumber: "ALR-2026-001234"}, nil
}

func startOrderServer(t *testing.T, fake *orderFake) (*grpc.ClientConn, func()) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	alorav1.RegisterBackendServer(server, fake)
	go func() { _ = server.Serve(listener) }()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			return invoker(metadataContext(ctx, "test-token"), method, req, reply, cc, opts...)
		}))
	if err != nil {
		t.Fatal(err)
	}
	return conn, func() {
		_ = conn.Close()
		server.Stop()
		_ = listener.Close()
	}
}

func TestOrderSaveDraft_Success(t *testing.T) {
	fake := &orderFake{mode: "ok"}
	conn, cleanup := startOrderServer(t, fake)
	defer cleanup()
	client := NewOrderClient(conn, 3*time.Second)

	ctx := session.WithID(context.Background(), "session-1")
	ctx = curtains.WithUser(ctx, curtains.UserInfo{UserID: "user-1", TenantID: "tenant-1", ConversationID: "conv-1"})
	got, err := client.SaveOrderDraft(ctx, curtains.OrderDraft{
		CustomerName: "Ali", CustomerPhone: "+998901234567", Address: "Toshkent",
		ItemsJSON: `{"measurements":[{"length":"3.000","height":"2.500","parts":[],"accessories":[]}],"services":["sewing"],"delivery":true,"discount_percent":"5"}`,
		Note:      "o'lchov baribir",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.OrderID != "order-1" || got.OrderNumber != "ALR-2026-001234" {
		t.Fatalf("javob: %+v", got)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.calls != 1 {
		t.Fatalf("sinxron chaqiruv soni=%d, retry bo'lmasligi kerak", fake.calls)
	}
	if values := fake.metadata.Get("x-service-token"); len(values) == 0 || values[0] != "test-token" {
		t.Fatalf("x-service-token metadata yo'q: %v", fake.metadata)
	}
}

func TestOrderSaveDraft_ErrorAndUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mode    string
		want    string
		isValid bool
	}{
		{name: "error", mode: "error", want: "manzil to'liq emas", isValid: true},
		{name: "unavailable", mode: "unavailable", want: "order service unavailable"},
		{name: "deadline", mode: "deadline", want: "order service unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &orderFake{mode: tc.mode}
			conn, cleanup := startOrderServer(t, fake)
			defer cleanup()
			client := NewOrderClient(conn, 3*time.Second)
			_, err := client.SaveOrderDraft(context.Background(), curtains.OrderDraft{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("xato=%v, want %q", err, tc.want)
			}
			if tc.isValid && !curtains.IsValidation(err) {
				t.Fatalf("ValidationError kutilgan: %v", err)
			}
			if !tc.isValid && !errors.Is(err, ErrOrderUnavailable) {
				t.Fatalf("ErrOrderUnavailable kutilgan: %v", err)
			}
			fake.mu.Lock()
			calls := fake.calls
			fake.mu.Unlock()
			if calls != 1 {
				t.Fatalf("chaqiruv soni=%d, retry YO'Q bo'lishi kerak", calls)
			}
		})
	}
}
