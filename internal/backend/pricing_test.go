package backend

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/muhammadvadud/alora-ai/gen/alorav1"
	"github.com/muhammadvadud/alora-ai/internal/curtains"
	"github.com/muhammadvadud/alora-ai/internal/curtains/curtainstest"
	"github.com/muhammadvadud/alora-ai/internal/session"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type rawFixture struct {
	Cases []struct {
		Input    json.RawMessage `json:"input"`
		Expected json.RawMessage `json:"expected"`
	} `json:"cases"`
}

type pricingFake struct {
	alorav1.UnimplementedPricingServer
	mu       sync.Mutex
	expected map[string][]byte
	calls    int
	mode     string
	metadata metadata.MD
	tenantID string
}

func (f *pricingFake) CalcPreview(ctx context.Context, req *alorav1.CalcPreviewRequest) (*alorav1.CalcPreviewResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.tenantID = req.GetTenantId()
	f.metadata, _ = metadata.FromIncomingContext(ctx)
	if f.mode == "unavailable" {
		return nil, status.Error(codes.Unavailable, "backend down")
	}
	if f.mode == "deadline" {
		return nil, status.Error(codes.DeadlineExceeded, "backend timeout")
	}
	if f.mode == "fields" {
		return &alorav1.CalcPreviewResponse{Error: "invalid", ErrorFieldsJson: `{"z":"z xato","a":"a xato"}`}, nil
	}
	if f.mode == "error" {
		return &alorav1.CalcPreviewResponse{Error: "hisob xatosi"}, nil
	}
	return &alorav1.CalcPreviewResponse{ResultJson: string(f.expected[req.GetPayloadJson()])}, nil
}

func startPricingServer(t *testing.T, fake *pricingFake) (*grpc.ClientConn, func()) {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	alorav1.RegisterPricingServer(server, fake)
	go func() { _ = server.Serve(listener) }()
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithUnaryInterceptor(func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
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

func TestPricing_ReferenceCases(t *testing.T) {
	fixture, _, err := curtainstest.Load(curtainstest.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	rawBytes, err := os.ReadFile(curtainstest.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	var raw rawFixture
	if err := json.Unmarshal(rawBytes, &raw); err != nil {
		t.Fatal(err)
	}
	expected := make(map[string][]byte, len(raw.Cases))
	for i, c := range fixture.Cases {
		payload, err := json.Marshal(c.Input.ToCalcInput(curtains.RoleManager))
		if err != nil {
			t.Fatal(err)
		}
		var gotAny, wantAny any
		if err := json.Unmarshal(payload, &gotAny); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw.Cases[i].Input, &wantAny); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(gotAny, wantAny) {
			t.Fatalf("%s payload mismatch:\ngot  %s\nwant %s", c.Name, payload, raw.Cases[i].Input)
		}
		expected[string(payload)] = raw.Cases[i].Expected
	}
	fake := &pricingFake{expected: expected}
	conn, cleanup := startPricingServer(t, fake)
	defer cleanup()
	client := NewPricingClient(conn, 3*time.Second, "tenant-1")
	for i, c := range fixture.Cases {
		t.Run(c.Name, func(t *testing.T) {
			ctx := session.WithID(context.Background(), "session-1")
			got, err := client.Calculate(ctx, c.Input.ToCalcInput(curtains.RoleManager))
			if err != nil {
				t.Fatal(err)
			}
			var gotAny, wantAny any
			gotJSON, _ := json.Marshal(got)
			_ = json.Unmarshal(gotJSON, &gotAny)
			_ = json.Unmarshal(raw.Cases[i].Expected, &wantAny)
			compareJSON(t, "", wantAny, gotAny)
		})
	}
	fake.mu.Lock()
	if values := fake.metadata.Get("x-service-token"); len(values) == 0 || values[0] != "test-token" {
		t.Fatalf("service token metadata missing: %v", fake.metadata)
	}
	fake.mu.Unlock()
}

func TestPricing_ErrorFieldsAndFallback(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode string
		want string
	}{
		{name: "fields", mode: "fields", want: "a: a xato"},
		{name: "error", mode: "error", want: "calc: hisob xatosi"},
		{name: "unavailable", mode: "unavailable", want: "pricing service unavailable"},
		{name: "deadline", mode: "deadline", want: "pricing service unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &pricingFake{mode: tc.mode}
			conn, cleanup := startPricingServer(t, fake)
			defer cleanup()
			client := NewPricingClient(conn, 200*time.Millisecond, "tenant-1")
			_, err := client.Calculate(context.Background(), curtains.CalcInput{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("xato=%v, want %q", err, tc.want)
			}
			if (tc.mode == "fields" || tc.mode == "error") && !curtains.IsValidation(err) {
				t.Fatalf("ValidationError kutilgan: %v", err)
			}
			if tc.mode == "unavailable" || tc.mode == "deadline" {
				if !errors.Is(err, ErrPricingUnavailable) {
					t.Fatalf("ErrPricingUnavailable kutilgan: %v", err)
				}
			}
			fake.mu.Lock()
			calls := fake.calls
			fake.mu.Unlock()
			if tc.mode == "unavailable" && calls != 2 {
				t.Fatalf("retry calls=%d, want 2", calls)
			}
		})
	}
}

func TestPricing_UsesTenantFromContext(t *testing.T) {
	fake := &pricingFake{expected: map[string][]byte{`{"area_m2":"0"}`: []byte(`{"area_m2":"0"}`)}}
	conn, cleanup := startPricingServer(t, fake)
	defer cleanup()
	client := NewPricingClient(conn, time.Second, "constructor-tenant")
	ctx := curtains.WithUser(context.Background(), curtains.UserInfo{TenantID: "context-tenant"})
	_, _ = client.Calculate(ctx, curtains.CalcInput{})
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.tenantID != "context-tenant" {
		t.Fatalf("tenant = %q", fake.tenantID)
	}
}

func compareJSON(t *testing.T, path string, want, got any) {
	t.Helper()
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			t.Errorf("%s: object expected, got %T", path, got)
			return
		}
		for k, value := range w {
			compareJSON(t, path+"."+k, value, g[k])
		}
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			t.Errorf("%s: list length mismatch", path)
			return
		}
		for i := range w {
			compareJSON(t, path, w[i], g[i])
		}
	case string:
		gs, ok := got.(string)
		if !ok {
			t.Errorf("%s: want %q got %v", path, w, got)
			return
		}
		wd, we := decimal.NewFromString(w)
		gd, ge := decimal.NewFromString(gs)
		if we == nil && ge == nil && !wd.Equal(gd) {
			t.Errorf("%s: want %s got %s", path, w, gs)
		} else if (we != nil || ge != nil) && w != gs {
			t.Errorf("%s: want %q got %q", path, w, gs)
		}
	default:
		if !reflect.DeepEqual(want, got) {
			t.Errorf("%s: want %v got %v", path, want, got)
		}
	}
}
