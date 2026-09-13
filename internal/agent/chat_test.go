package agent

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-agent/internal/session"
)

// scriptModel — Stream skripti bilan ishlaydigan mock: birinchi chaqiruv
// tool_call qaytaradi, ikkinchisi yakuniy matnni. ResponseMeta.Usage provider
// shaklida (cached/reasoning bilan) to'ldirilgan — agregatni shu orqali tekshiramiz.
type scriptModel struct {
	streamCalls int
}

func (m *scriptModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return nil, errors.New("stream rejimida Generate chaqirilmasligi kerak")
}

func (m *scriptModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.streamCalls++
	switch m.streamCalls {
	case 1:
		return schema.StreamReaderFromArray([]*schema.Message{{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				Type:     "function",
				ID:       "call_1",
				Function: schema.FunctionCall{Name: "find_product", Arguments: `{"room":"living"}`},
			}},
			ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
				PromptTokens:            10,
				CompletionTokens:        5,
				PromptTokenDetails:      schema.PromptTokenDetails{CachedTokens: 3},
				CompletionTokensDetails: schema.CompletionTokensDetails{ReasoningTokens: 2},
			}},
		}}), nil
	case 2:
		return schema.StreamReaderFromArray([]*schema.Message{{
			Role:    schema.Assistant,
			Content: "Mana siz uchun bir nechta variant topildi.",
			ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
				PromptTokens:            12,
				CompletionTokens:        4,
				PromptTokenDetails:      schema.PromptTokenDetails{CachedTokens: 1},
				CompletionTokensDetails: schema.CompletionTokensDetails{ReasoningTokens: 1},
			}},
		}}), nil
	default:
		return nil, errors.New("kutilmagan uchinchi Stream chaqiruvi")
	}
}

func (m *scriptModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

type findArgs struct{}

type findResult struct {
	Found int `json:"found"`
}

// findTool — natijasida AttachProducts orqali mahsulot belgilaydigan oddiy tool.
// Bu real product_search adapterining agent bilan bog'lanish nuqtasini qamrab oladi.
func findTool(t *testing.T) tool.BaseTool {
	tt, err := utils.InferTool("find_product", "mahsulot topadi", func(ctx context.Context, _ findArgs) (findResult, error) {
		AttachProducts(ctx, []ProductRef{{VariantID: "v1", ProductName: "Ipak parda", ColorName: "bej", Price: 125000}})
		return findResult{Found: 1}, nil
	})
	if err != nil {
		t.Fatalf("InferTool: %v", err)
	}
	return tt
}

// fakeStateHook — StateReporter'ni yozib oluvchi test dublikasi.
type fakeStateHook struct {
	tools []string
}

func (h *fakeStateHook) OnToolCall(_ context.Context, _, tool, _ string) {
	h.tools = append(h.tools, tool)
}

func (h *fakeStateHook) Stage(_ context.Context, _ string) (string, string) {
	return "recommend", `{"room":"living"}`
}

// Aynan rules §6.1 tartib: start -> tool_call -> tool_args -> tool_result ->
// products -> content -> stage -> done. done.usage agregat qiymatlarni olib
// keladi; products event tool topgan mahsulotni; stage event bosqichni.
func TestChatStream_FullEventOrder(t *testing.T) {
	store := session.NewMemory()
	hook := &fakeStateHook{}
	chat, err := NewChat(context.Background(), &scriptModel{}, []tool.BaseTool{findTool(t)}, store, Options{
		Stream:    true,
		StateHook: hook,
		Provider:  "deepseek",
		Model:     "deepseek-chat",
	})
	if err != nil {
		t.Fatalf("NewChat: %v", err)
	}

	var (
		kinds        []Kind
		productsSeen []Event
		stageSeen    []Event
		doneSeen     []Event
	)
	for ev := range chat.Stream(context.Background(), "s1", "mehmonxonaga parda kerak") {
		kinds = append(kinds, ev.Kind)
		switch ev.Kind {
		case KindProducts:
			productsSeen = append(productsSeen, ev)
		case KindStage:
			stageSeen = append(stageSeen, ev)
		case KindDone:
			doneSeen = append(doneSeen, ev)
		}
	}

	want := []Kind{KindStart, KindToolCall, KindToolArgs, KindToolResult, KindProducts, KindContent, KindStage, KindDone}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("event tartibi = %v, want %v", kinds, want)
	}

	if len(productsSeen) != 1 {
		t.Fatalf("products event soni = %d, want 1", len(productsSeen))
	}
	p := productsSeen[0]
	if len(p.Products) != 1 || p.Products[0].VariantID != "v1" || p.Products[0].ProductName != "Ipak parda" {
		t.Errorf("products event noto'g'ri: %+v", p.Products)
	}

	if len(stageSeen) != 1 {
		t.Fatalf("stage event soni = %d, want 1", len(stageSeen))
	}
	if stageSeen[0].Content != "recommend" || stageSeen[0].Result != `{"room":"living"}` {
		t.Errorf("stage event = %+v, want Content=recommend Result={\"room\":\"living\"}", stageSeen[0])
	}

	if len(doneSeen) != 1 || doneSeen[0].Usage == nil {
		t.Fatalf("done.usage yo'q: %+v", doneSeen)
	}
	u := doneSeen[0].Usage
	if u.PromptTokens != 22 || u.CompletionTokens != 9 || u.CachedTokens != 4 || u.ReasoningTokens != 3 {
		t.Errorf("done.usage agregat = %+v, want Prompt=22 Completion=9 Cached=4 Reasoning=3", u)
	}
	if u.Provider != "deepseek" || u.Model != "deepseek-chat" {
		t.Errorf("done.usage markerlari = %s/%s, want deepseek/deepseek-chat", u.Provider, u.Model)
	}
	if !reflect.DeepEqual(u.ToolCalls, []string{"find_product"}) {
		t.Errorf("done.usage ToolCalls = %v, want [find_product]", u.ToolCalls)
	}
	if u.LatencyMs < 0 {
		t.Errorf("LatencyMs manfiy: %d", u.LatencyMs)
	}

	if !reflect.DeepEqual(hook.tools, []string{"find_product"}) {
		t.Errorf("StateHook.OnToolCall = %v, want [find_product]", hook.tools)
	}

	// Tarix invariantiga tegmadi: butun navbat yozilgan (user + tool_call'li
	// assistant + tool natijasi + yakuniy javob).
	h, err := store.History(context.Background(), "s1")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(h) != 4 || h[0].Role != schema.User || h[1].Role != schema.Assistant ||
		h[2].Role != schema.Tool || h[3].Role != schema.Assistant {
		t.Errorf("tarix yaxlit yozilmagan: %d xabar, roles=%v", len(h), rolesOf(h))
	}
}

// StateHook nil bo'lsa stage event chiqmaydi (bosqich yo'q), done hali ham Usage oladi.
func TestChatStream_NoStateHook_NoStageEvent(t *testing.T) {
	chat, err := NewChat(context.Background(), &scriptModel{}, []tool.BaseTool{findTool(t)}, session.NewMemory(), Options{
		Stream: true,
	})
	if err != nil {
		t.Fatalf("NewChat: %v", err)
	}

	var kinds []Kind
	done := (Event{}).Usage
	for ev := range chat.Stream(context.Background(), "s2", "mehmonxonaga parda kerak") {
		kinds = append(kinds, ev.Kind)
		if ev.Kind == KindDone {
			done = ev.Usage
		}
	}

	for _, k := range kinds {
		if k == KindStage {
			t.Fatal("StateHook nil bo'lsa ham stage event chiqdi")
		}
	}
	want := []Kind{KindStart, KindToolCall, KindToolArgs, KindToolResult, KindProducts, KindContent, KindDone}
	if !reflect.DeepEqual(kinds, want) {
		t.Errorf("event tartibi = %v, want %v", kinds, want)
	}
	if done == nil || done.ToolCalls == nil {
		t.Errorf("done.usage yo'q: %+v", done)
	}
}

func rolesOf(msgs []*schema.Message) []schema.RoleType {
	out := make([]schema.RoleType, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.Role)
	}
	return out
}
