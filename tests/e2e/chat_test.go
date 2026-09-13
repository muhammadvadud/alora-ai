package e2e

import (
	"context"
	"net"
	"testing"

	"github.com/muhammadvadud/alora-agent/gen/alorav1"
	"github.com/muhammadvadud/alora-agent/internal/agent"
	grpctransport "github.com/muhammadvadud/alora-agent/internal/transport/grpc"
	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

func TestChatBufconnEventStream(t *testing.T) {
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
	defer conn.Close()

	stream, err := alorav1.NewAgentClient(conn).Chat(context.Background(), &alorav1.ChatRequest{SessionId: "e2e", Message: "salom"})
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
