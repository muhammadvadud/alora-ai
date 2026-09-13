package backend

import (
	"context"
	"fmt"

	"github.com/muhammadvadud/alora-agent/gen/alorav1"
	"github.com/muhammadvadud/alora-agent/internal/pii"
	"google.golang.org/protobuf/encoding/protojson"
)

type MessageLog struct {
	SessionID, ConversationID string
	Role                      string
	Content                   string
	ToolCalls, ToolResult     any
	TokensIn, TokensOut       int
}

func (o *Outbox) LogMessage(ctx context.Context, msg MessageLog) error {
	toolCalls, err := jsonValue(msg.ToolCalls)
	if err != nil {
		return fmt.Errorf("tool calls: %w", err)
	}
	toolResult, err := jsonValue(msg.ToolResult)
	if err != nil {
		return fmt.Errorf("tool result: %w", err)
	}
	req := &alorav1.MessageLogRequest{SessionId: msg.SessionID, ConversationId: msg.ConversationID, Role: msg.Role, Content: pii.Scrub(msg.Content), ToolCallsJson: pii.Scrub(string(toolCalls)), ToolResultJson: pii.Scrub(string(toolResult)), TokensIn: int32(msg.TokensIn), TokensOut: int32(msg.TokensOut)}
	payload, err := protojson.Marshal(req)
	if err != nil {
		return fmt.Errorf("message log JSON: %w", err)
	}
	return o.Enqueue(ctx, "log", payload)
}

func jsonValue(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return marshalPayload(v)
}
