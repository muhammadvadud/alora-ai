package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

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
	model            model.ToolCallingChatModel // tool'lar bog'langan
	tools            map[string]tool.InvokableTool
	store            session.Store
	systemPrompt     string
	maxStep          int
	maxHistory       int // modelga yuboriladigan eski xabarlar chegarasi (history.go)
	stream           bool
	stateHook        StageReporter // nil bo'lsa stage event'lar chiqmaydi
	provider         string        // done.usage uchun (main.go to'ldiradi)
	modelName        string
	systemPromptFunc func(context.Context, string) string
	fallback         model.ToolCallingChatModel
	fallbackProvider string
}

// fallbackAnswer — model matnli javob bermasa (masalan maxStep tugab qolsa)
// foydalanuvchi hech bo'lmasa nimadir ko'rsin. Alora uslubi (rules §6.4):
// narx aytilmaydi, hisobni qo'lda tekshirish tavsiya qilinadi.
const fallbackAnswer = "Kechirasiz, bu savolga hozir aniq javob bera olmadim. Qaytadan so'rang yoki hisobni qo'lda tekshirib ko'ring."

// Options — Chat sozlamalari. Struct sifatida, chunki pozitsion parametrlar ko'payib
// ketgan edi va MaxStep/MaxHistory qo'shni int'lar — ularni jimgina almashtirib
// yuborish oson bo'lardi.
type Options struct {
	SystemPrompt string
	// SystemPromptFunc har navbatda state/katalog o'zgarishi sabab prompt'ni qayta tuzadi.
	SystemPromptFunc func(context.Context, string) string
	// MaxStep — ReAct siklining maksimal qadami (tool -> model -> tool ...). 0 = 20.
	MaxStep int
	// MaxHistory — modelga yuboriladigan eski xabarlar chegarasi. 0 = 40 (history.go).
	MaxHistory int
	// Stream — javobni jonli (bo'lak-bo'lak) yuborish. false bo'lsa yaxlit keladi
	// va bitta content Event bo'ladi (Event shartnomasi o'zgarmaydi — transportlar bilmaydi).
	Stream bool
	// StateHook — suhbat bosqichini transportga xabar qiluvchi hook (stage.go).
	// nil bo'lsa agent bosqich/state haqida hech narsa chiqarmaydi.
	StateHook StageReporter
	// Provider/Model — done.usage markerlari (main.go shu nomlarni to'ldiradi).
	Provider         string
	Model            string
	Fallback         model.ToolCallingChatModel
	FallbackProvider string
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
	var fallback model.ToolCallingChatModel
	if opts.Fallback != nil {
		fallback, err = opts.Fallback.WithTools(infos)
		if err != nil {
			return nil, fmt.Errorf("fallback tool'larini bog'lash: %w", err)
		}
	}
	return &Chat{
		model: bound, tools: byName, store: store,
		systemPrompt: opts.SystemPrompt, maxStep: opts.MaxStep,
		maxHistory: opts.MaxHistory, stream: opts.Stream,
		stateHook: opts.StateHook, provider: opts.Provider, modelName: opts.Model,
		fallback: fallback, fallbackProvider: opts.FallbackProvider,
		systemPromptFunc: opts.SystemPromptFunc,
	}, nil
}

// Stream foydalanuvchi xabariga javobni Event oqimi sifatida qaytaradi.
// Ketma-ketlik: start -> (tool_call -> tool_args -> tool_result -> products?)*
// -> content* -> stage -> done (rules §6.1).
func (c *Chat) Stream(ctx context.Context, sessionID, input string) <-chan Event {
	out := make(chan Event, 16)

	// done.usage LatencyMs shu nuqtadan o'lchanadi.
	start := time.Now()

	go func() {
		defer close(out)
		emit(ctx, out, Event{Kind: KindStart})

		// Tool'lar topgan mahsulotlarni yig'uvchi kontekst — product_search kabi
		// tool'lar AttachProducts bilan belgilaydi, agent har natijadan keyin
		// yig'ilganini KindProducts event'iga aylantiradi (products.go).
		ctx, collector := WithProductCollector(ctx)

		// Tool'lar joriy sessiyani bilishi uchun (masalan oxirgi order'ga bog'lash).
		ctx = session.WithID(ctx, sessionID)
		systemPrompt := c.systemPrompt
		if c.systemPromptFunc != nil {
			systemPrompt = c.systemPromptFunc(ctx, sessionID)
		}

		history, err := c.store.History(ctx, sessionID)
		if err != nil {
			emit(ctx, out, Event{Kind: KindError, Err: err.Error()})
			return
		}

		msgs := make([]*schema.Message, 0, len(history)+4)
		if systemPrompt != "" {
			msgs = append(msgs, schema.SystemMessage(systemPrompt))
		}
		msgs = append(msgs, trimHistory(history, c.maxHistory)...)
		msgs = append(msgs, schema.UserMessage(input))

		// Shu navbatda tug'ilgan xabarlar (user + assistant + tool) shu indeksdan boshlanadi —
		// oxirida HAMMASI tarixga yoziladi.
		turnStart := len(msgs) - 1

		// Shu navbat davomidagi jamlanma hisob: bir nechta ask bo'lsa qo'shiladi.
		var (
			usage     llm.TokenUsage
			toolNames []string // done.usage uchun, chaqirilish tartibida
		)

		answered := false
		activeModel := c.model
		usedFallback := false
		for step := 0; step < c.maxStep; step++ {
			content, toolCalls, u, nextModel, switched, err := c.ask(ctx, msgs, out, activeModel)
			if err != nil {
				emit(ctx, out, Event{Kind: KindError, Err: err.Error()})
				return
			}
			activeModel = nextModel
			usedFallback = usedFallback || switched
			usage = addUsage(usage, u)
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
				toolNames = append(toolNames, tc.Function.Name)
				// State hook: bosqich to'g'ridan-to'g'ri tool argumentlaridan yangilanadi
				// (rules §4.5.2) — javob matni parse qilinmaydi.
				if c.stateHook != nil {
					c.stateHook.OnToolCall(ctx, sessionID, tc.Function.Name, tc.Function.Arguments)
				}
				if refs := collector.Drain(); len(refs) > 0 {
					emit(ctx, out, Event{Kind: KindProducts, Products: refs})
				}
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
		// Bosqich xabari javob tugashidan oldin (rules §4.5.5) — transport shu orqali
		// suhbat bosqichini ko'radi. Hook yo'q bo'lsa hech narsa chiqmaydi.
		if c.stateHook != nil {
			stage, summary := c.stateHook.Stage(ctx, sessionID)
			if stage != "" {
				emit(ctx, out, Event{Kind: KindStage, Content: stage, Result: summary})
			}
		}
		provider := c.provider
		if usedFallback {
			provider = c.fallbackProvider
		}
		emit(ctx, out, Event{
			Kind: KindDone,
			Usage: &Usage{
				Provider:         provider,
				Model:            c.modelName,
				PromptTokens:     usage.Prompt,
				CompletionTokens: usage.Completion,
				CachedTokens:     usage.Cached,
				ReasoningTokens:  usage.Reasoning,
				ToolCalls:        toolNames,
				LatencyMs:        time.Since(start).Milliseconds(),
			},
		})
	}()

	return out
}

// addUsage ikki token hisobini qo'shadi (bir navbatda bir nechta ask bo'lsa yig'indi).
func addUsage(a, b llm.TokenUsage) llm.TokenUsage {
	a.Prompt += b.Prompt
	a.Completion += b.Completion
	a.Cached += b.Cached
	a.Reasoning += b.Reasoning
	return a
}

// Reset sessiya tarixini tozalaydi.
func (c *Chat) Reset(ctx context.Context, sessionID string) error {
	return c.store.Reset(ctx, sessionID)
}

// ask model'dan bitta javob oladi: stream yoqilgan bo'lsa oqimni jonli o'qiydi,
// aks holda yaxlit so'rov yuboradi. Ikkalasi ham bir xil Event'larni chiqaradi.
func (c *Chat) ask(ctx context.Context, msgs []*schema.Message, out chan<- Event, active model.ToolCallingChatModel) (string, []schema.ToolCall, llm.TokenUsage, model.ToolCallingChatModel, bool, error) {
	content, calls, usage, err := c.askOnce(ctx, msgs, out, active)
	if err == nil {
		return content, calls, usage, active, active == c.fallback, nil
	}
	if active != c.model || !llm.IsRetryable(err) {
		return "", nil, llm.TokenUsage{}, active, false, err
	}
	select {
	case <-time.After(200 * time.Millisecond):
	case <-ctx.Done():
		return "", nil, llm.TokenUsage{}, active, false, ctx.Err()
	}
	content, calls, usage, err = c.askOnce(ctx, msgs, out, active)
	if err == nil {
		return content, calls, usage, active, false, nil
	}
	if !llm.IsRetryable(err) {
		return "", nil, llm.TokenUsage{}, active, false, err
	}
	if c.fallback == nil {
		return "", nil, llm.TokenUsage{}, active, false, err
	}
	slog.Warn("llm_fallback", "provider", c.fallbackProvider, "error", err)
	content, calls, usage, err = c.askOnce(ctx, msgs, out, c.fallback)
	if err != nil {
		return "", nil, llm.TokenUsage{}, c.fallback, true, err
	}
	return content, calls, usage, c.fallback, true, nil
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
