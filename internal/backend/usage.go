package backend

import (
	"context"
	"fmt"

	"github.com/muhammadvadud/alora-ai/gen/alorav1"
	"github.com/muhammadvadud/alora-ai/internal/agent"
	"google.golang.org/protobuf/encoding/protojson"
)

type UsageIDs struct{ SessionID, UserID, TenantID, ConversationID, MessageID string }

func (o *Outbox) ReportUsage(ctx context.Context, ev agent.Usage, ids UsageIDs) error {
	calls, err := marshalPayload(ev.ToolCalls)
	if err != nil {
		return fmt.Errorf("tool calls: %w", err)
	}
	req := &alorav1.UsageRequest{SessionId: ids.SessionID, UserId: ids.UserID, TenantId: ids.TenantID, ConversationId: ids.ConversationID, MessageId: ids.MessageID, Usage: &alorav1.Usage{
		Provider: ev.Provider, Model: ev.Model, PromptTokens: int32(ev.PromptTokens), CompletionTokens: int32(ev.CompletionTokens), CachedTokens: int32(ev.CachedTokens), ReasoningTokens: int32(ev.ReasoningTokens), ToolCallsJson: string(calls), LatencyMs: int32(ev.LatencyMs), CostUsd: 0, CostUzs: 0,
	}}
	payload, err := protojson.Marshal(req)
	if err != nil {
		return fmt.Errorf("usage JSON: %w", err)
	}
	return o.Enqueue(ctx, "usage", payload)
}
