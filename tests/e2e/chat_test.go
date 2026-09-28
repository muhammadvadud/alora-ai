package e2e

import (
	"context"
	"net"
	"testing"

	"github.com/muhammadvadud/alora-ai/gen/alorav1"
	"github.com/muhammadvadud/alora-ai/internal/agent"
	grpctransport "github.com/muhammadvadud/alora-ai/internal/transport/grpc"
	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeAgent struct{}

func (fakeAgent) Stream(ctx context.Context, _, _ string) <-chan agent.Event {
	out := make(chan agent.Event, 3)
	out <- agent.Event{Kind: agent.KindStart}
	out <- agent.Event{Kind: agent.KindContent, Content: "tayyor"}
	out <- agent.Event{Kind: agent.KindDone}
	close(out)
	return out
}
func (fakeAgent) Reset(context.Context, string) error { return nil }

// startAgentServer — bufconn orqali in-process Agent server va uning klienti.
func startAgentServer(t *testing.T) alorav1.AgentClient {
	t.Helper()
	listener := bufconn.Listen(1 << 20)
	server := grpcgo.NewServer()
	alorav1.RegisterAgentServer(server, grpctransport.New(fakeAgent{}, grpctransport.Options{}))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	conn, err := grpcgo.NewClient("passthrough:///bufnet", grpcgo.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpcgo.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return alorav1.NewAgentClient(conn)
}

func TestChatBufconnEventStream(t *testing.T) {
	client := startAgentServer(t)
	stream, err := client.Chat(context.Background(), &alorav1.ChatRequest{SessionId: "e2e", UserId: "u1", TenantId: "t1", Message: "salom"})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for {
		event, recvErr := stream.Recv()
		if recvErr != nil {
			break
		}
		got = append(got, event.GetKind())
	}
	if len(got) != 3 || got[0] != "start" || got[1] != "content" || got[2] != "done" {
		t.Fatalf("event kinds = %v", got)
	}
}

// TestChatBufconnRejectsEmptyUserID — bo'sh user_id bilan so'rov sessiya izolyatsiyasini
// buzadi (Store egalikni bo'sh so'rov uchun tekshirmaydi), shuning uchun rad etiladi.
func TestChatBufconnRejectsEmptyUserID(t *testing.T) {
	client := startAgentServer(t)
	for _, userID := range []string{"", "   "} {
		stream, err := client.Chat(context.Background(), &alorav1.ChatRequest{SessionId: "e2e", UserId: userID, Message: "salom"})
		if err != nil {
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("user_id %q: code = %v", userID, status.Code(err))
			}
			continue
		}
		if _, err := stream.Recv(); status.Code(err) != codes.InvalidArgument {
			t.Fatalf("user_id %q: code = %v", userID, status.Code(err))
		}
	}
}
