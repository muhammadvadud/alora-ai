package agent

// chat_model.go — model chaqiruvi: stream/yaxlit so'rov, retry va fallback.
// chat.go dan ajratilgan (rules §2: bitta fayl 500 qatordan oshmaydi).

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-ai/internal/llm"
)

// ask model'dan bitta javob oladi: stream yoqilgan bo'lsa oqimni jonli o'qiydi,
// aks holda yaxlit so'rov yuboradi. Ikkalasi ham bir xil Event'larni chiqaradi.
//
// primary/fallbackM — shu Stream uchun bog'langan modellar (har Stream'da bir marta
// WithTools chaqiriladi). Fallback ichida yana fallback bo'lmaydi — retry faqat
// asosiy modelda, aktiv model fallback bo'lsa xato darhol qaytadi.
func (c *Chat) ask(ctx context.Context, msgs []*schema.Message, out chan<- Event, active, primary, fallbackM model.ToolCallingChatModel) (string, []schema.ToolCall, llm.TokenUsage, model.ToolCallingChatModel, bool, error) {
	content, calls, usage, err := c.askOnce(ctx, msgs, out, active)
	if err == nil {
		// switched: javob fallback modeldan keldi (done.usage provider'i shu).
		return content, calls, usage, active, active != primary, nil
	}
	if active != primary || !llm.IsRetryable(err) {
		return "", nil, llm.TokenUsage{}, active, false, err
	}
	select {
	case <-time.After(200 * time.Millisecond):
	case <-ctx.Done():
		return "", nil, llm.TokenUsage{}, active, false, ctx.Err()
	}
	content, calls, usage, err = c.askOnce(ctx, msgs, out, primary)
	if err == nil {
		return content, calls, usage, primary, false, nil
	}
	if !llm.IsRetryable(err) {
		return "", nil, llm.TokenUsage{}, primary, false, err
	}
	if fallbackM == nil {
		return "", nil, llm.TokenUsage{}, primary, false, err
	}
	slog.Warn("llm_fallback", "provider", c.fallbackProvider, "error", err)
	content, calls, usage, err = c.askOnce(ctx, msgs, out, fallbackM)
	if err != nil {
		return "", nil, llm.TokenUsage{}, fallbackM, true, err
	}
	return content, calls, usage, fallbackM, true, nil
}

func (c *Chat) askOnce(ctx context.Context, msgs []*schema.Message, out chan<- Event, active model.ToolCallingChatModel) (string, []schema.ToolCall, llm.TokenUsage, error) {
	if !c.stream {
		return c.generate(ctx, active, msgs, out)
	}
	stream, err := active.Stream(ctx, msgs)
	if err != nil {
		return "", nil, llm.TokenUsage{}, err
	}
	return c.readStream(ctx, stream, out)
}

// generate — stream o'chirilgan yo'l: javob yaxlit keladi va bitta content Event bo'ladi.
func (c *Chat) generate(ctx context.Context, active model.ToolCallingChatModel, msgs []*schema.Message, out chan<- Event) (string, []schema.ToolCall, llm.TokenUsage, error) {
	msg, err := active.Generate(ctx, msgs)
	if err != nil {
		return "", nil, llm.TokenUsage{}, err
	}
	if r, ok := llm.Reasoning(msg); ok && r != "" {
		emit(ctx, out, Event{Kind: KindThinking, Content: r})
	}
	if msg.Content != "" {
		emit(ctx, out, Event{Kind: KindContent, Content: msg.Content})
	}
	for _, tc := range msg.ToolCalls {
		emit(ctx, out, Event{Kind: KindToolCall, Tool: tc.Function.Name})
		emit(ctx, out, Event{Kind: KindToolArgs, Tool: tc.Function.Name, Args: tc.Function.Arguments})
	}
	u, _ := llm.Usage(msg)
	return msg.Content, msg.ToolCalls, u, nil
}

// readStream bitta model oqimini o'qiydi: matnni JONLI content Event sifatida chiqaradi,
// tool_call'larni index bo'yicha yig'adi (nom -> tool_call, argumentlar EOF'da tool_args).
func (c *Chat) readStream(ctx context.Context, stream *schema.StreamReader[*schema.Message], out chan<- Event) (string, []schema.ToolCall, llm.TokenUsage, error) {
	defer stream.Close()

	var content strings.Builder
	type acc struct {
		id, name string
		args     strings.Builder
	}
	calls := map[int]*acc{}
	var order []int
	// Usage odatda oqimning oxirgi chunk'ida keladi — har chunk'ni alohida qo'shamiz,
	// shunda provider uni qaysi chunk'da yuborishidan qat'i nazar hisob yo'qolmaydi.
	var usage llm.TokenUsage

	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", nil, llm.TokenUsage{}, err
		}
		// O'ylash (reasoning) javobdan OLDIN va alohida oqim bo'lib keladi — uni
		// content'ga qo'shmaymiz (u javob emas), alohida Event qilamiz.
		if r, ok := llm.Reasoning(chunk); ok && r != "" {
			emit(ctx, out, Event{Kind: KindThinking, Content: r})
		}
		if chunk.Content != "" {
			content.WriteString(chunk.Content)
			emit(ctx, out, Event{Kind: KindContent, Content: chunk.Content}) // JONLI stream
		}
		if u, ok := llm.Usage(chunk); ok {
			usage = addUsage(usage, u)
		}
		for _, tc := range chunk.ToolCalls {
			idx := 0
			if tc.Index != nil {
				idx = *tc.Index
			}
			a, ok := calls[idx]
			if !ok {
				a = &acc{}
				calls[idx] = a
				order = append(order, idx)
			}
			if tc.ID != "" {
				a.id = tc.ID
			}
			if a.name == "" && tc.Function.Name != "" {
				a.name = tc.Function.Name
				emit(ctx, out, Event{Kind: KindToolCall, Tool: a.name}) // nom keldi
			}
			a.args.WriteString(tc.Function.Arguments)
		}
	}

	var toolCalls []schema.ToolCall
	for _, idx := range order {
		a := calls[idx]
		i := idx
		toolCalls = append(toolCalls, schema.ToolCall{
			Index:    &i,
			ID:       a.id,
			Type:     "function",
			Function: schema.FunctionCall{Name: a.name, Arguments: a.args.String()},
		})
		emit(ctx, out, Event{Kind: KindToolArgs, Tool: a.name, Args: a.args.String()})
	}
	return content.String(), toolCalls, usage, nil
}
