package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-agent/internal/llm"
	"github.com/muhammadvadud/alora-agent/internal/session"
)

// Chat — qo'lda yozilgan ReAct sikli: model.Stream() ni o'qib, matnni JONLI stream qiladi,
// tool_call'larni to'liq oqimdan yig'ib bajaradi va tool natijasi bilan qayta so'raydi.
//
// Nega qo'lda: eino react agentining StreamToolCallChecker'i tool'ni ishonchli aniqlash uchun
// butun oqimni o'qishga majbur bo'ladi (DeepSeek matnni tool_call'dan oldin yuboradi), bu esa
// yakuniy javobni buferlaydi — stream yo'qoladi. Bu sikl ikkalasini ham beradi.
type Chat struct {
	model        model.ToolCallingChatModel // tool'lar bog'langan
	tools        map[string]tool.InvokableTool
	store        session.Store
	systemPrompt string
	maxStep      int
	maxHistory   int // modelga yuboriladigan eski xabarlar chegarasi (history.go)
	stream       bool
}

// fallbackAnswer — model matnli javob bermasa (masalan maxStep tugab qolsa)
// foydalanuvchi hech bo'lmasa nimadir ko'rsin.
const fallbackAnswer = "Kechirasiz, javob bera olmadim. Qayta urinib ko'ring."

// Options — Chat sozlamalari. Struct sifatida, chunki pozitsion parametrlar ko'payib
// ketgan edi va MaxStep/MaxHistory qo'shni int'lar — ularni jimgina almashtirib
// yuborish oson bo'lardi.
type Options struct {
	SystemPrompt string
	// MaxStep — ReAct siklining maksimal qadami (tool -> model -> tool ...). 0 = 20.
	MaxStep int
	// MaxHistory — modelga yuboriladigan eski xabarlar chegarasi. 0 = 40 (history.go).
	MaxHistory int
	// Stream — javobni jonli (bo'lak-bo'lak) yuborish. false bo'lsa yaxlit keladi
	// va bitta content Event bo'ladi (Event shartnomasi o'zgarmaydi — transportlar bilmaydi).
	Stream bool
}

// NewChat model'ga tool'larni bog'laydi va Chat servisini yaratadi.
func NewChat(ctx context.Context, chatModel model.ToolCallingChatModel, tools []tool.BaseTool, store session.Store, opts Options) (*Chat, error) {
	if opts.MaxStep <= 0 {
		opts.MaxStep = defaultMaxStep
	}
	if opts.MaxHistory <= 0 {
		opts.MaxHistory = defaultMaxHistory
	}
	infos := make([]*schema.ToolInfo, 0, len(tools))
	byName := make(map[string]tool.InvokableTool, len(tools))
	for _, t := range tools {
		info, err := t.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("tool ma'lumotini olish: %w", err)
		}
		it, ok := t.(tool.InvokableTool)
		if !ok {
			return nil, fmt.Errorf("tool %q InvokableTool emas", info.Name)
		}
		infos = append(infos, info)
		byName[info.Name] = it
	}
	bound, err := chatModel.WithTools(infos)
	if err != nil {
		return nil, fmt.Errorf("tool'larni bog'lash: %w", err)
	}
	return &Chat{
		model: bound, tools: byName, store: store,
		systemPrompt: opts.SystemPrompt, maxStep: opts.MaxStep,
		maxHistory: opts.MaxHistory, stream: opts.Stream,
	}, nil
}

// Stream foydalanuvchi xabariga javobni Event oqimi sifatida qaytaradi.
// Ketma-ketlik: start -> (content* | tool_call -> tool_args -> tool_result)* -> content* -> done.
func (c *Chat) Stream(ctx context.Context, sessionID, input string) <-chan Event {
	out := make(chan Event, 16)

	go func() {
		defer close(out)
		emit(ctx, out, Event{Kind: KindStart})

		// Tool'lar joriy sessiyani bilishi uchun (masalan oxirgi order'ga bog'lash).
		ctx = session.WithID(ctx, sessionID)

		history, err := c.store.History(ctx, sessionID)
		if err != nil {
			emit(ctx, out, Event{Kind: KindError, Err: err.Error()})
			return
		}

		msgs := make([]*schema.Message, 0, len(history)+4)
		if c.systemPrompt != "" {
			msgs = append(msgs, schema.SystemMessage(c.systemPrompt))
		}
		msgs = append(msgs, trimHistory(history, c.maxHistory)...)
		msgs = append(msgs, schema.UserMessage(input))

		// Shu navbatda tug'ilgan xabarlar (user + assistant + tool) shu indeksdan boshlanadi —
		// oxirida HAMMASI tarixga yoziladi.
		turnStart := len(msgs) - 1

		answered := false
		for step := 0; step < c.maxStep; step++ {
			content, toolCalls, err := c.ask(ctx, msgs, out)
			if err != nil {
				emit(ctx, out, Event{Kind: KindError, Err: err.Error()})
				return
			}
			if len(toolCalls) == 0 {
				// Yakuniy javob. Bo'sh matnni tarixga yozmaymiz — quyida fallback qo'yiladi.
				if strings.TrimSpace(content) != "" {
					msgs = append(msgs, schema.AssistantMessage(content, nil))
					answered = true
				}
				break
			}
			msgs = append(msgs, schema.AssistantMessage(content, toolCalls))
			// Tool'larni bajaramiz va natijalarni suhbatga qo'shamiz.
			for _, tc := range toolCalls {
				result := c.runTool(ctx, tc)
				emit(ctx, out, Event{Kind: KindToolResult, Tool: tc.Function.Name, Result: result})
				msgs = append(msgs, schema.ToolMessage(result, tc.ID))
			}
		}

		// Matnli javob bo'lmadi: maxStep tugadi (model tool chaqiraverdi) yoki javob bo'sh.
		// Fallback msgs ga ham qo'shiladi — shunda tarixda bo'sh assistant qolmaydi.
		if !answered {
			slog.Warn("model matnli javob bermadi", "session", sessionID, "max_step", c.maxStep)
			emit(ctx, out, Event{Kind: KindContent, Content: fallbackAnswer})
			msgs = append(msgs, schema.AssistantMessage(fallbackAnswer, nil))
		}

		// Tarixga BUTUN navbat yoziladi: user + assistant (tool_call'lari bilan) + tool
		// natijalari. Faqat yakuniy matnni saqlash XATO edi: keyingi navbatda model o'zining
		// "topildi/yaratildi" degan gapini ko'rib, tool'ni qayta chaqirmasdan natijani
		// TO'QIB chiqarardi. Tool xabarlari — bajarilgan ishning yagona isboti.
		// Eslatma: assistant(tool_calls) va unga mos tool xabarlari juftligi buzilmasligi
		// kerak (provayder buni talab qiladi) — shu sabab kesim yaxlit yoziladi.
		if err := c.store.Append(ctx, sessionID, msgs[turnStart:]...); err != nil {
			slog.Warn("suhbat tarixini saqlab bo'lmadi", "session", sessionID, "err", err)
		}
		emit(ctx, out, Event{Kind: KindDone})
	}()

	return out
}

// Reset sessiya tarixini tozalaydi.
func (c *Chat) Reset(ctx context.Context, sessionID string) error {
	return c.store.Reset(ctx, sessionID)
}

// ask model'dan bitta javob oladi: stream yoqilgan bo'lsa oqimni jonli o'qiydi,
// aks holda yaxlit so'rov yuboradi. Ikkalasi ham bir xil Event'larni chiqaradi.
func (c *Chat) ask(ctx context.Context, msgs []*schema.Message, out chan<- Event) (string, []schema.ToolCall, error) {
	if !c.stream {
		return c.generate(ctx, msgs, out)
	}
	stream, err := c.model.Stream(ctx, msgs)
	if err != nil {
		return "", nil, err
	}
	return c.readStream(ctx, stream, out)
}

// generate — stream o'chirilgan yo'l: javob yaxlit keladi va bitta content Event bo'ladi.
func (c *Chat) generate(ctx context.Context, msgs []*schema.Message, out chan<- Event) (string, []schema.ToolCall, error) {
	msg, err := c.model.Generate(ctx, msgs)
	if err != nil {
		return "", nil, err
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
	return msg.Content, msg.ToolCalls, nil
}

// readStream bitta model oqimini o'qiydi: matnni JONLI content Event sifatida chiqaradi,
// tool_call'larni index bo'yicha yig'adi (nom -> tool_call, argumentlar EOF'da tool_args).
func (c *Chat) readStream(ctx context.Context, stream *schema.StreamReader[*schema.Message], out chan<- Event) (string, []schema.ToolCall, error) {
	defer stream.Close()

	var content strings.Builder
	type acc struct {
		id, name string
		args     strings.Builder
	}
	calls := map[int]*acc{}
	var order []int

	for {
		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", nil, err
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
	return content.String(), toolCalls, nil
}

// runTool tool_call'ni bajaradi va natija (yoki xato) matnini qaytaradi.
func (c *Chat) runTool(ctx context.Context, tc schema.ToolCall) string {
	t, ok := c.tools[tc.Function.Name]
	if !ok {
		return fmt.Sprintf(`{"error":"noma'lum tool: %s"}`, tc.Function.Name)
	}
	res, err := t.InvokableRun(ctx, tc.Function.Arguments)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return res
}

// emit ctx bekor bo'lsa bloklanmasdan chiqadi (iste'molchi ketgan bo'lsa).
func emit(ctx context.Context, out chan<- Event, ev Event) {
	select {
	case out <- ev:
	case <-ctx.Done():
	}
}
