package golden

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// llmStep — mock modelning BITTA javobi (Chat.Stream navbatidagi model chaqiruvi).
// Yo tools bo'ladi (bitta awsatda bir nechta parallel tool_call), yo text
// (yakuniy matn). Ikkalasi ham bo'lsa emas — skript har chaqiruvni aniq biladi.
type llmStep struct {
	Tools []llmTool `yaml:"tools,omitempty"`
	Text  string    `yaml:"text,omitempty"`
}

// llmTool — bitta tool_call: nomi va argumentlari (JSON'ga marshal qilinadi).
type llmTool struct {
	Name string         `yaml:"name"`
	Args map[string]any `yaml:"args,omitempty"`
}

// mockModel — skript (llmStep ketma-ketligi) bo'yicha ishlaydigan sohta
// ToolCallingChatModel. Har Stream chaqiruvi navbatdagi skript qadamini qaytaradi;
// skript tugay qolsa xato (harness buni muvaffaqiyatsizlik deb qayd qiladi).
// chat_test.go scriptModel naqshi (tasks 1.41: "skript: user matn → tool chaqiruv/javob").
type mockModel struct {
	mu        sync.Mutex
	script    []llmStep
	next      int // navbatdagi skript qadami
	callID    int // tool_call ID'lari uchun tartib raqam
	exhausted bool
}

func (m *mockModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return nil, errors.New("stream rejimida Generate chaqirilmasligi kerak")
}

// Stream skriptdagi navbatdagi javobni qaytaradi: tools bo'lsa har biri alohida
// chunk (Index bilan — parallel tool_call'lar to'g'ri yig'ilib, tartibda chiqadi),
// text bo'lsa bo'laklarga bo'lib oqiladi (rules §4.5.4: TTFT uchun stream).
func (m *mockModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.next >= len(m.script) {
		m.exhausted = true
		return nil, errors.New("mock: llm skripti tugab qoldi — reagent tool/style davom ettirdi")
	}
	cur := m.script[m.next]
	m.next++
	if len(cur.Tools) > 0 {
		return m.toolCallChunks(cur.Tools)
	}
	return schema.StreamReaderFromArray(textChunks(cur.Text)), nil
}

func (m *mockModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func (m *mockModel) toolCallChunks(calls []llmTool) (*schema.StreamReader[*schema.Message], error) {
	msgs := make([]*schema.Message, 0, len(calls))
	for i, call := range calls {
		args := call.Args
		if args == nil {
			args = map[string]any{}
		}
		raw, err := json.Marshal(args)
		if err != nil {
			return nil, fmt.Errorf("tool argumentlarini marshal qilish: %w", err)
		}
		idx := i
		m.callID++
		msgs = append(msgs, &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				Index:    &idx,
				ID:       fmt.Sprintf("call_%02d", m.callID),
				Type:     "function",
				Function: schema.FunctionCall{Name: call.Name, Arguments: string(raw)},
			}},
		})
	}
	return schema.StreamReaderFromArray(msgs), nil
}

// textChunkRunes matn bir stream bo'lagiga nechta rune sig'adi. Katta emas —
// agent matnni JONLI stream qilayotganini isbotlash uchun 2+ bo'lak bo'lsin.
const textChunkRunes = 40

// textChunks matnni rune'larga bo'lib, oxirgi bo'lakka usage ma'lumotini qo'yadi
// (answerning tok senc — Chat.Stream uchun unchalik muhim emas, ammo real provider
// naqshi shunday: usage oqim oxirida keladi, chat.go uni har chunk'dan yig'adi).
func textChunks(s string) []*schema.Message {
	if s == "" {
		return nil
	}
	rs := []rune(s)
	parts := make([]string, 0, len(rs)/textChunkRunes+1)
	for len(rs) > 0 {
		n := min(textChunkRunes, len(rs))
		parts = append(parts, string(rs[:n]))
		rs = rs[n:]
	}
	msgs := make([]*schema.Message, 0, len(parts))
	for i, p := range parts {
		msg := &schema.Message{Role: schema.Assistant, Content: p}
		if i == len(parts)-1 {
			msg.ResponseMeta = &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 10, CompletionTokens: 5}}
		}
		msgs = append(msgs, msg)
	}
	return msgs
}
