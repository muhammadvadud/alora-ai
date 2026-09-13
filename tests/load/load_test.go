//go:build load

package load

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/muhammadvadud/alora-ai/gen/alorav1"
	"github.com/muhammadvadud/alora-ai/internal/agent"
	"github.com/muhammadvadud/alora-ai/internal/session"
	"github.com/muhammadvadud/alora-ai/internal/tools"
	grpctransport "github.com/muhammadvadud/alora-ai/internal/transport/grpc"
	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func setupBufconnServer(t *testing.T) alorav1.AgentClient {
	store := session.NewMemory()
	cat := setupCatalog()
	deps := tools.Deps{
		Catalog: cat,
		Calc:    fakeCalc{},
		State:   store,
		Orders:  fakeOrders{},
	}
	aloraTools := tools.Alora(deps)

	chat, err := agent.NewChat(context.Background(), &loadMockModel{}, aloraTools, store, agent.Options{Stream: true})
	if err != nil {
		t.Fatalf("agent.NewChat: %v", err)
	}

	listener := bufconn.Listen(1 << 20)
	server := grpcgo.NewServer()
	alorav1.RegisterAgentServer(server, grpctransport.New(chat, grpctransport.Options{}))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpcgo.NewClient("passthrough:///bufnet", grpcgo.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpcgo.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	return alorav1.NewAgentClient(conn)
}

func TestLoad_50ParallelSessions(t *testing.T) {
	client := setupBufconnServer(t)

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)
	startGoroutines := runtime.NumGoroutine()
	startTime := time.Now()

	const numSessions = 50
	const reqsPerSession = 5

	var wg sync.WaitGroup
	wg.Add(numSessions)

	var mu sync.Mutex
	var latencies []time.Duration

	for i := 0; i < numSessions; i++ {
		sessionID := fmt.Sprintf("sess-%d", i)
		go func(sid string) {
			defer wg.Done()
			for j := 0; j < reqsPerSession; j++ {
				reqStart := time.Now()
				req := &alorav1.ChatRequest{
					SessionId: sid,
					Message:   "salom",
				}
				stream, err := client.Chat(context.Background(), req)
				if err != nil {
					t.Errorf("Chat error: %v", err)
					return
				}

				var gotDone bool
				for {
					ev, err := stream.Recv()
					if err != nil {
						break
					}
					if ev.Kind == "error" {
						t.Errorf("Unexpected error event: %v", ev)
					}
					if ev.Kind == "done" {
						gotDone = true
					}
				}
				if !gotDone {
					t.Errorf("Did not receive done event")
				}

				latency := time.Since(reqStart)
				mu.Lock()
				latencies = append(latencies, latency)
				mu.Unlock()
			}
		}(sessionID)
	}

	wg.Wait()
	totalTime := time.Since(startTime)

	runtime.GC()
	runtime.ReadMemStats(&m2)

	// Wait 1s for any delayed goroutines to finish
	time.Sleep(1 * time.Second)
	endGoroutines := runtime.NumGoroutine()

	heapDiff := int64(m2.HeapAlloc) - int64(m1.HeapAlloc)

	t.Logf("Umumiy vaqt (Total time): %v", totalTime)
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		p95 := latencies[len(latencies)*95/100]
		t.Logf("p95 javob vaqti (latency): %v", p95)
	}
	t.Logf("Xotira farqi (HeapAlloc diff): %d MB", heapDiff/(1024*1024))
	t.Logf("Goroutine soni (start=%d, end=%d)", startGoroutines, endGoroutines)

	if heapDiff > 200*1024*1024 {
		t.Errorf("Xotira 200MB dan ko'p ishlatildi (HeapAlloc diff exceeded): %v", heapDiff)
	}

	gDiff := endGoroutines - startGoroutines
	if gDiff < -5 || gDiff > 5 {
		t.Errorf("Goroutine sizishi aniqlandi (leak detected): diff = %d", gDiff)
	}
}

func TestLoad_RateLimitPerSession(t *testing.T) {
	// NEGA: rate limiter interceptor grpctransport.Serve ichida qilingan,
	// uni tashqariga chiqarmasdan bufconn bilan ishlatish qiyin. Serve
	// 127.0.0.1:0 ga ulanganda band qilingan portni qaytarmaydi.
	t.Skip("rate limiter interceptor Options'da emas, Serve() ichida berilganligi va unexported bo'lganligi sababli bufconn bilan ulash imkonsiz")
}

func TestLoad_ClientCancelReleases(t *testing.T) {
	client := setupBufconnServer(t)

	startGoroutines := runtime.NumGoroutine()

	for i := 0; i < 20; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		req := &alorav1.ChatRequest{SessionId: fmt.Sprintf("c-%d", i), Message: "salom"}
		stream, err := client.Chat(ctx, req)
		if err == nil {
			_, _ = stream.Recv()
		}
		cancel() // darhol bekor qilish
	}

	time.Sleep(1 * time.Second)
	endGoroutines := runtime.NumGoroutine()

	gDiff := endGoroutines - startGoroutines
	if gDiff < -5 || gDiff > 5 {
		t.Errorf("Bekor qilingan streamlar goroutine'larni bo'shatmadi (Goroutines didn't return to normal after cancel), farq (diff): %d", gDiff)
	}
}
