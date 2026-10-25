package grpc

import (
	"context"
	"testing"

	"github.com/muhammadvadud/alora-ai/gen/alorav1"
	"github.com/muhammadvadud/alora-ai/internal/agent"
)

func TestChat_PersistedUserMessageKeepsUsageIDWithoutDuplicateLog(t *testing.T) {
	reporter := &recordingReporter{}
	fake := &fakeStreamer{events: []agent.Event{{Kind: agent.KindContent, Content: "javob"}, {Kind: agent.KindDone, Usage: &agent.Usage{PromptTokens: 3}}}}
	srv := New(fake, Options{Reporter: reporter})
	stream := &fakeChatStream{ctx: context.Background()}
	req := &alorav1.ChatRequest{SessionId: "s1", UserId: "u1", TenantId: "t1", Message: "ismim Sinovbek", Context: map[string]string{"conversation_id": "c1", "message_id": "persisted-message"}}
	if err := srv.Chat(req, stream); err != nil {
		t.Fatal(err)
	}
	if len(reporter.logs) != 1 || reporter.logs[0].Role != "assistant" || len(reporter.ids) != 1 || reporter.ids[0].MessageID != "persisted-message" {
		t.Fatalf("takroriy user log yoki usage ID yo'qoldi: %+v %+v", reporter.logs, reporter.ids)
	}
}
