package session

import (
	"context"

	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
	"github.com/muhammadvadud/alora-ai/internal/pii"
)

type privateStore struct{ Store }

func WithPIIScrubbing(store Store) Store { return &privateStore{Store: store} }

func (s *privateStore) History(ctx context.Context, id string) ([]*schema.Message, error) {
	messages, err := s.Store.History(ctx, id)
	if err != nil {
		return nil, err
	}
	return privateMessages(messages), nil
}

func (s *privateStore) Append(ctx context.Context, id string, messages ...*schema.Message) error {
	return s.Store.Append(ctx, id, privateMessages(messages)...)
}

func (s *privateStore) State(ctx context.Context, id string) (curtains.SessionState, error) {
	state, err := s.Store.State(ctx, id)
	return privateState(state), err
}

func (s *privateStore) SetState(ctx context.Context, id string, state curtains.SessionState) error {
	return s.Store.SetState(ctx, id, privateState(state))
}

func privateState(state curtains.SessionState) curtains.SessionState {
	state.Construction = pii.Scrub(state.Construction)
	state.Colors = append([]string(nil), state.Colors...)
	for index, color := range state.Colors {
		state.Colors[index] = pii.Scrub(color)
	}
	state.Windows = append([]curtains.WindowSpec(nil), state.Windows...)
	for index, window := range state.Windows {
		state.Windows[index].RoomName = pii.Scrub(window.RoomName)
	}
	return state
}

func privateMessages(messages []*schema.Message) []*schema.Message {
	out := make([]*schema.Message, len(messages))
	for index, message := range messages {
		if message == nil {
			continue
		}
		copy := *message
		copy.Content = pii.Scrub(message.Content)
		copy.ReasoningContent = pii.Scrub(message.ReasoningContent)
		if copy.Role == schema.User && copy.Name != "" {
			copy.Name = "mijoz"
		}
		copy.MultiContent = append([]schema.ChatMessagePart(nil), message.MultiContent...)
		for partIndex, part := range copy.MultiContent {
			copy.MultiContent[partIndex].Text = pii.Scrub(part.Text)
		}
		copy.UserInputMultiContent = append([]schema.MessageInputPart(nil), message.UserInputMultiContent...)
		for partIndex, part := range copy.UserInputMultiContent {
			copy.UserInputMultiContent[partIndex].Text = pii.Scrub(part.Text)
		}
		copy.AssistantGenMultiContent = append([]schema.MessageOutputPart(nil), message.AssistantGenMultiContent...)
		for partIndex, part := range copy.AssistantGenMultiContent {
			copy.AssistantGenMultiContent[partIndex].Text = pii.Scrub(part.Text)
		}
		copy.ToolCalls = append([]schema.ToolCall(nil), message.ToolCalls...)
		for callIndex, call := range copy.ToolCalls {
			copy.ToolCalls[callIndex].Function.Arguments = pii.ScrubToolResult(call.Function.Arguments)
		}
		out[index] = &copy
	}
	return out
}
