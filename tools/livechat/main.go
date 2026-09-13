// livechat — qo'lda live sinov vositasi: gRPC agent bilan bir nechta xabar almashadi va
// event tartibi, TTFT, token sarfini ko'rsatadi. Ishga tushirish:
//   go run ./tools/livechat "xabar 1" "xabar 2"
// (agent :9095 da, token devtoken — dev sozlamasi).
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/muhammadvadud/alora-ai/gen/alorav1"
)

func main() {
	conn, err := grpc.NewClient("localhost:9095", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	c := alorav1.NewAgentClient(conn)
	sid := fmt.Sprintf("live-%d", time.Now().Unix())
	for i, msg := range os.Args[1:] {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		ctx = metadata.AppendToOutgoingContext(ctx, "x-service-token", "devtoken", "x-request-id", fmt.Sprintf("req-%d", i))
		fmt.Printf("\n>>> USER: %s\n", msg)
		st, err := c.Chat(ctx, &alorav1.ChatRequest{SessionId: sid, UserId: "u1", TenantId: "t1", Locale: "uz", Message: msg, Context: map[string]string{"conversation_id": "c1", "role": "employee"}})
		if err != nil {
			panic(err)
		}
		var content strings.Builder
		start := time.Now()
		first := time.Duration(0)
		for {
			ev, err := st.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				fmt.Println("ERR:", err)
				break
			}
			switch ev.Kind {
			case "content":
				if first == 0 {
					first = time.Since(start)
				}
				content.WriteString(ev.Content)
			case "tool_call":
				fmt.Printf("  [tool_call] %s\n", ev.Tool)
			case "tool_result":
				r := ev.Result
				if len(r) > 160 {
					r = r[:160] + "…"
				}
				fmt.Printf("  [tool_result] %s\n", r)
			case "products":
				fmt.Printf("  [products] %d ta\n", len(ev.Products))
			case "stage":
				fmt.Printf("  [stage] %s\n", ev.Content)
			case "error":
				fmt.Printf("  [error] %s\n", ev.Error)
			case "done":
				u := ev.Usage
				if u != nil {
					fmt.Printf("  [done] ttft=%s total=%s tokens=%d/%d provider=%s\n", first.Round(time.Millisecond), time.Since(start).Round(time.Millisecond), u.PromptTokens, u.CompletionTokens, u.Provider)
				}
			}
		}
		fmt.Printf("<<< AI: %s\n", content.String())
		cancel()
	}
}
