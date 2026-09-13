package agent

import (
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-ai/internal/session"
)

// selectState — WithTools'ga berilgan tool nomlari va Stream aylanish hisobi
// (bir nechta WithTools nusxasi bitta holatni ulashadi — xuddi real o'ramlar kabi).
type selectState struct {
	mu       sync.Mutex
	bound    [][]string
	calls    int
	callTool string // birinchi Stream aylanishida chaqiriladigan tool ("" = darhol matn)
}

// selectRecorder — mock model: WithTools'ga berilgan tool'larni yozadi, Stream'da
// belgilangan tool'ni chaqirib keyingi aylanishda matnli javob qaytaradi. Stream
// ichidagi WithTools chaqiruvini "model sxemalari = selector natijasi" deb
// tekshirish shu yozuvchi orqali.
type selectRecorder struct{ s *selectState }

func newSelectRecorder(callTool string) *selectRecorder {
	return &selectRecorder{s: &selectState{callTool: callTool}}
}

func (m *selectRecorder) WithTools(infos []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	names := make([]string, len(infos))
	for i, info := range infos {
		names[i] = info.Name
	}
	m.s.mu.Lock()
	m.s.bound = append(m.s.bound, names)
	m.s.mu.Unlock()
	return &selectRecorder{s: m.s}, nil
}

func (m *selectRecorder) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return nil, errors.New("stream rejimida Generate chaqirilmasligi kerak")
}

func (m *selectRecorder) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.s.mu.Lock()
	m.s.calls++
	n := m.s.calls
	callTool := m.s.callTool
	m.s.mu.Unlock()
	if n == 1 && callTool != "" {
		return schema.StreamReaderFromArray([]*schema.Message{{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				ID: "call_sel_1", Type: "function",
				Function: schema.FunctionCall{Name: callTool, Arguments: `{}`},
			}},
		}}), nil
	}
	return schema.StreamReaderFromArray([]*schema.Message{{
		Role: schema.Assistant, Content: "tayyor",
	}}), nil
}

// namedTool berilgan nomli oddiy InvokableTool yaratadi (registratsiya testi uchun).
func namedTool(t *testing.T, name string) tool.BaseTool {
	t.Helper()
	tt, err := utils.InferTool(name, "test tool", func(context.Context, struct{}) (map[string]any, error) {
		return map[string]any{"ok": true}, nil
	})
	if err != nil {
		t.Fatalf("InferTool(%s): %v", name, err)
	}
	return tt
}

func selectorToolNames(names []string) func(_ context.Context, _ string, all []tool.BaseTool) []tool.BaseTool {
	return func(_ context.Context, _ string, all []tool.BaseTool) []tool.BaseTool {
		want := make(map[string]bool, len(names))
		for _, n := range names {
			want[n] = true
		}
		selected := make([]tool.BaseTool, 0, len(names))
		for _, tt := range all {
			info, err := tt.Info(context.Background())
			if err != nil {
				continue
			}
			if want[info.Name] {
				selected = append(selected, tt)
			}
		}
		return selected
	}
}

// Modelga berilgan tool sxemalari selector natijasiga TENG bo'ladi: tanlov
// bosqichga qarab to'plamni kichraytirsa ham, Stream shu to'plamni WithTools
// orqali bog'laydi (buyurtmachi xarajat talabi).
func TestChatStream_ToolSelector_FiltersModelTools(t *testing.T) {
	tools := []tool.BaseTool{namedTool(t, "tool_a"), namedTool(t, "tool_b"), namedTool(t, "tool_c")}
	rec := newSelectRecorder("tool_b")
	chat, err := NewChat(context.Background(), rec, tools, session.NewMemory(), Options{
		Stream:       true,
		ToolSelector: selectorToolNames([]string{"tool_b"}),
	})
	if err != nil {
		t.Fatalf("NewChat: %v", err)
	}

	for ev := range chat.Stream(context.Background(), "s1", "salom") {
		if ev.Kind == KindError {
			t.Fatalf("kutilmagan error: %s", ev.Err)
		}
	}

	rec.s.mu.Lock()
	bound := append([][]string(nil), rec.s.bound...)
	rec.s.mu.Unlock()
	if len(bound) != 1 || !reflect.DeepEqual(bound[0], []string{"tool_b"}) {
		t.Fatalf("modelga bog'langan tool'lar = %v, want [[tool_b]]", bound)
	}
}

// Selector nil bo'lsa HAMMA tool sxemalari modelga beriladi (eski xatti-harakat).
func TestChatStream_ToolSelectorNil_AllTools(t *testing.T) {
	tools := []tool.BaseTool{namedTool(t, "tool_a"), namedTool(t, "tool_b"), namedTool(t, "tool_c")}
	rec := newSelectRecorder("tool_a")
	chat, err := NewChat(context.Background(), rec, tools, session.NewMemory(), Options{Stream: true})
	if err != nil {
		t.Fatalf("NewChat: %v", err)
	}

	for ev := range chat.Stream(context.Background(), "s1", "salom") {
		if ev.Kind == KindError {
			t.Fatalf("kutilmagan error: %s", ev.Err)
		}
	}

	rec.s.mu.Lock()
	bound := append([][]string(nil), rec.s.bound...)
	rec.s.mu.Unlock()
	if len(bound) != 1 || len(bound[0]) != 3 {
		t.Fatalf("modelga bog'langan tool'lar = %v, want 3 nomli ro'yxat", bound)
	}
	for _, name := range []string{"tool_a", "tool_b", "tool_c"} {
		if !containsName(bound[0], name) {
			t.Fatalf("modelga bog'langan tool'lar = %v, %s topilmadi", bound[0], name)
		}
	}
}

// Selector to'plamida yo'q tool'ni model chaqirsa ijro xaritasi ham shu to'plamdan —
// runTool "noma'lum tool" qaytaradi va ReAct o'zini tuzatib davom etadi.
func TestChatStream_ToolOutsideSelection_UnknownTool(t *testing.T) {
	tools := []tool.BaseTool{namedTool(t, "tool_a"), namedTool(t, "tool_b")}
	// Model tool_b chaqiradi, lekin selector faqat tool_a ni berdi.
	rec := newSelectRecorder("tool_b")
	chat, err := NewChat(context.Background(), rec, tools, session.NewMemory(), Options{
		Stream:       true,
		ToolSelector: selectorToolNames([]string{"tool_a"}),
	})
	if err != nil {
		t.Fatalf("NewChat: %v", err)
	}

	var results []string
	for ev := range chat.Stream(context.Background(), "s1", "salom") {
		if ev.Kind == KindToolResult {
			results = append(results, ev.Result)
		}
	}
	if len(results) != 1 || !strings.Contains(results[0], "noma'lum tool") {
		t.Fatalf("tanlovdan tashqari tool natijasi = %v, want noma'lum tool", results)
	}
}

func containsName(xs []string, name string) bool {
	for _, x := range xs {
		if x == name {
			return true
		}
	}
	return false
}

// budgetModel — ikki aylanishda mos ravishda 25k va 20k prompt token qaytaradi:
// umumiy 45k byudjet (40k) dan oshadi, sikl ikkinchi aylanishda to'xtashi kerak.
type budgetModel struct {
	calls int
}

func (m *budgetModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (m *budgetModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	sr, err := m.Stream(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	defer sr.Close()
	var out schema.Message
	for {
		chunk, recvErr := sr.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				break
			}
			return nil, recvErr
		}
		out.Role = chunk.Role
		out.Content += chunk.Content
		out.ToolCalls = append(out.ToolCalls, chunk.ToolCalls...)
		out.ResponseMeta = chunk.ResponseMeta
	}
	return &out, nil
}

func (m *budgetModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.calls++
	meta := &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 25000, CompletionTokens: 10}}
	if m.calls == 1 {
		// Birinchi aylanish: tool chaqiradi → ikkinchi aylanish keladi.
		return schema.StreamReaderFromArray([]*schema.Message{{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				ID: "call_b_1", Type: "function",
				Function: schema.FunctionCall{Name: "any_tool", Arguments: `{}`},
			}},
			ResponseMeta: meta,
		}}), nil
	}
	return schema.StreamReaderFromArray([]*schema.Message{{
		Role: schema.Assistant, Content: "javob",
		ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 20000, CompletionTokens: 5}},
	}}), nil
}

// Byudjet qo'riqchisi: yig'ilgan prompt token chegaradan oshsa sikl to'xtaydi,
// error event chiqadi, tarixga yozilmaydi va done chiqmaydi.
func TestChatStream_PromptTokenBudget_StopsLoop(t *testing.T) {
	store := session.NewMemory()
	exceeded := 0
	chat, err := NewChat(context.Background(), &budgetModel{}, nil, store, Options{
		Stream:            true,
		PromptTokenBudget: 40000,
		OnBudgetExceeded:  func(string) { exceeded++ },
	})
	if err != nil {
		t.Fatalf("NewChat: %v", err)
	}

	var (
		gotError string
		gotDone  bool
	)
	for ev := range chat.Stream(context.Background(), "s1", "salom") {
		switch ev.Kind {
		case KindError:
			gotError = ev.Err
		case KindDone:
			gotDone = true
		}
	}

	if !strings.Contains(gotError, "javob byudjeti tugadi") || !strings.Contains(gotError, "45000 token") {
		t.Fatalf("error event = %q, want byudjet xabari (45000 token)", gotError)
	}
	if gotDone {
		t.Fatal("byudjet tugagach done event chiqmasligi kerak")
	}
	if exceeded != 1 {
		t.Fatalf("OnBudgetExceeded chaqiruvlari = %d, want 1", exceeded)
	}
	history, err := store.History(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("byudjet xatosi tarixga yozildi: %d xabar", len(history))
	}
}

// Byudjet 0 (o'chiq) bo'lsa yig'ilgan tokenlar cheklanmaydi — odatdagi oqim.
func TestChatStream_PromptTokenBudget_ZeroIsOff(t *testing.T) {
	store := session.NewMemory()
	chat, err := NewChat(context.Background(), &budgetModel{}, nil, store, Options{Stream: true})
	if err != nil {
		t.Fatalf("NewChat: %v", err)
	}

	var done *Usage
	for ev := range chat.Stream(context.Background(), "s1", "salom") {
		if ev.Kind == KindDone {
			done = ev.Usage
		}
		if ev.Kind == KindError {
			t.Fatalf("byudjet o'chiq bo'lsa error chiqmasligi kerak: %s", ev.Err)
		}
	}
	if done == nil || done.PromptTokens != 45000 {
		t.Fatalf("done.usage = %+v, want Prompt=45000", done)
	}
	history, err := store.History(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(history) == 0 {
		t.Fatal("oddiy navbat tarixga yozilishi kerak")
	}
}
