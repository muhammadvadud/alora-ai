package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// WithTrace modelni debug trace fayliga ulab qaytaradi: har Generate/Stream
// chaqiruvida faylga bitta request va bitta response qatori (JSONL) qo'shiladi.
// Buyurtmachi debug uchun LLM'ga aniq nima ketganini va nima kelganini ko'radi.
//
// path bo'sh bo'lmasa chaqiriladi (main.go); fayl ochilolmasa darhol xato
// qaytadi — trace noto'g'ri sozlangan bo'lsa ishga tushmaslik "yomon trace
// bilan bejirim ishlashdan" aniqroq.
func WithTrace(m model.ToolCallingChatModel, path string) (model.ToolCallingChatModel, error) {
	if m == nil {
		return nil, errors.New("trace: model nil")
	}
	tf, err := newTraceFile(path)
	if err != nil {
		return nil, err
	}
	return &traceModel{inner: m, tf: tf}, nil
}

// traceFile — debug trace fayli yozuvchisi. Bitta WithTrace o'ramidan
// WithTools natijasida bir nechta model nusxasi paydo bo'ladi, lekin ular
// YAGONA yozuvchini ulashadi — aks holda qatorlar bir-birini ustiga yozardi.
type traceFile struct {
	mu   sync.Mutex // parallel sessiyalarda qatorlar ajralmasin
	file *os.File
}

func newTraceFile(path string) (*traceFile, error) {
	// O_APPEND: qayta ishga tushirishda eski trace yo'qolmasin (log naqshi).
	// 0600: trace'da suhbat matni bor — boshqa foydalanuvchi o'qiy olmasin.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("trace faylini ochish: %w", err)
	}
	return &traceFile{file: f}, nil
}

// write bitta to'liq JSONL qatorini qo'shadi. Mutex ichida bitta Write —
// qator hech qachon yarim kesilib qolmaydi (parallel chaqiruvlar testi).
func (t *traceFile) write(v any) error {
	line, err := json.Marshal(v)
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	// json.Marshal har chaqiruvda yangi buffer qaytaradi — append xavfsiz.
	_, err = t.file.Write(append(line, '\n'))
	return err
}

// traceModel — model o'rami. Kalit/Authorization hech qachon yozilmaydi:
// biz HTTP emas, eino xabarlarining tanlangan maydonlarini (role/content/
// tool_calls) yozamiz — provider shaklini bilmaymiz, demak maxfiy narsa ham
// ulamaymiz.
type traceModel struct {
	inner model.ToolCallingChatModel
	tf    *traceFile
	tools []string // WithTools da olingan nomlar; request qatoridagi "tools" uchun
}

// WithTools o'ramni saqlaydi (rules §6 nozikligi): ichki modelga tool'lar
// bog'lanadi va YANGI trace o'rami qaytariladi — aks holda agent tool'lar
// bog'lagach trace butunlay yonlab qoladi. Fayl yozuvchi (`tf`) saqlanadi.
func (t *traceModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	inner, err := t.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(tools))
	for _, info := range tools {
		names = append(names, info.Name)
	}
	return &traceModel{inner: inner, tf: t.tf, tools: names}, nil
}

func (t *traceModel) Generate(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	start := time.Now()
	t.writeRequest(in)
	msg, err := t.inner.Generate(ctx, in, opts...)
	resp := traceResponse{TS: nowIso(), Kind: "response", LatencyMs: sinceMs(start)}
	if msg != nil {
		resp.Content = msg.Content
		resp.ToolCalls = traceToolCalls(msg.ToolCalls)
		if msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
			resp.Usage.Prompt = msg.ResponseMeta.Usage.PromptTokens
			resp.Usage.Completion = msg.ResponseMeta.Usage.CompletionTokens
			resp.Usage.Cached = msg.ResponseMeta.Usage.PromptTokenDetails.CachedTokens
		}
	}
	if err != nil {
		// Xato ham yoziladi — debugging uchun "nimaga javob yo'q" muhim.
		resp.Error = err.Error()
	}
	t.writeBestEffort(resp)
	return msg, err
}

// Stream so'rovni yozib, ichki model oqimini o'rab qaytaradi. Chunki javob
// bo'laklari iste'molchiga O'ZGARMASDAN uzatilishi kerak (streaming yo'qolmasin),
// yig'ish alohida goroutineda, iste'molchi bilan parallel boradi.
func (t *traceModel) Stream(ctx context.Context, in []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	start := time.Now()
	t.writeRequest(in)
	inner, err := t.inner.Stream(ctx, in, opts...)
	if err != nil {
		t.writeBestEffort(traceResponse{
			TS: nowIso(), Kind: "response", LatencyMs: sinceMs(start), Error: err.Error(),
		})
		return nil, err
	}
	out, writer := schema.Pipe[*schema.Message](8)
	go t.drainStream(inner, writer, start)
	return out, nil
}

// drainStream ichki oqimni o'qib, har bo'lakni iste'molchiga uzatadi va bir
// vaqtning o'zida yagona response qatoriga jamlab yozadi. Bo'laklarning o'tish
// sur'ati o'zgarmaydi — chunki bu yerda hech qanday buferlash yig'ilishi yo'q,
// faqat uzatilgan narsa nusxalanadi.
func (t *traceModel) drainStream(inner *schema.StreamReader[*schema.Message], writer *schema.StreamWriter[*schema.Message], start time.Time) {
	defer writer.Close() // iste'molchi qaytmasa ham oqim bir marta yopiladi
	defer inner.Close()

	var content strings.Builder
	// Stream'dagi tool_call bo'lak bo'lib keladi va Index orqali juftlanadi
	// (chat.go readStream'dagi naqsh) — argumentlar EOF'da to'liq bo'ladi.
	calls := map[int]*traceCallAcc{}
	var order []int
	prompt, completion, cached := 0, 0, 0

	for {
		chunk, err := inner.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			// Ichki oqim xatosini iste'molchiga uzatamiz va uning o'zini
			// response qatoriga yozamiz — keyingi navbat trace'da ko'rinadi.
			writer.Send(nil, err)
			t.writeBestEffort(traceResponse{
				TS: nowIso(), Kind: "response", LatencyMs: sinceMs(start),
				Content: content.String(), ToolCalls: finalTraceCalls(calls, order),
				Error: err.Error(),
			})
			return
		}
		if chunk != nil {
			content.WriteString(chunk.Content)
			if chunk.ResponseMeta != nil && chunk.ResponseMeta.Usage != nil {
				prompt += chunk.ResponseMeta.Usage.PromptTokens
				completion += chunk.ResponseMeta.Usage.CompletionTokens
				cached += chunk.ResponseMeta.Usage.PromptTokenDetails.CachedTokens
			}
			for _, tc := range chunk.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}
				a, ok := calls[idx]
				if !ok {
					a = &traceCallAcc{}
					calls[idx] = a
					order = append(order, idx)
				}
				if a.name == "" && tc.Function.Name != "" {
					a.name = tc.Function.Name
				}
				a.args.WriteString(tc.Function.Arguments)
			}
		}
		// Iste'molchi oqimni yopgan bo'lsa (masalan ctx bekor) Send false
		// qaytadi — shunda bloklanmasdan chiqamiz, qancha yig'ilgan bo'lsa
		// shunchasini response qatoriga yozamiz.
		if closed := writer.Send(chunk, nil); closed {
			break
		}
	}

	t.writeBestEffort(traceResponse{
		TS: nowIso(), Kind: "response", LatencyMs: sinceMs(start),
		Content: content.String(), ToolCalls: finalTraceCalls(calls, order),
		Usage: traceUsage{Prompt: prompt, Completion: completion, Cached: cached},
	})
}

// writeBestEffort trace yozuvi yiqilsa suhbatni buzmaydi — model chaqiruvi
// trace fayli to'lgani uchun yiqilmasligi kerak. Shovqin qilmaydi (jim
// yutilmaydi), faqat warn log chiqadi.
func (t *traceModel) writeBestEffort(v any) {
	if err := t.tf.write(v); err != nil {
		slog.Warn("llm trace yozilmadi", "err", err)
	}
}

// writeRequest so'rovni yozadi — tavsifga qaraganda iste'molchi faylida
// "modelga nima kirdi" qatori keladi.
func (t *traceModel) writeRequest(in []*schema.Message) {
	t.writeBestEffort(traceRequest{
		TS: nowIso(), Kind: "request",
		Messages: serializeMessages(in),
		Tools:    t.toolNames(),
	})
}

func (t *traceModel) toolNames() []string {
	out := make([]string, len(t.tools))
	copy(out, t.tools)
	return out
}

// traceRequest — modelga ketayotgan so'rovning bo'sh (xavfsiz) nusxasi.
// Eino xabarlaridan FAQAT shular olinadi: role/content/tool_calls — bizga
// keraksiz (va ba'zan maxfiy) ResponseMeta/Extra maydonlari tashlab qo'yiladi.
type traceRequest struct {
	TS       string         `json:"ts"`
	Kind     string         `json:"kind"`
	Messages []traceMessage `json:"messages"`
	Tools    []string       `json:"tools"`
}

type traceMessage struct {
	Role       string          `json:"role"`
	Content    string          `json:"content,omitempty"`
	ToolCalls  []traceToolCall `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"` // tool xabari uchun (assistant emas)
}

// traceResponse — model javobining yagona qatori. Stream'da bo'laklar
// yig'ilib shu bitta qator bo'lib yoziladi.
type traceResponse struct {
	TS        string          `json:"ts"`
	Kind      string          `json:"kind"`
	Content   string          `json:"content,omitempty"`
	ToolCalls []traceToolCall `json:"tool_calls,omitempty"`
	Usage     traceUsage      `json:"usage"`
	LatencyMs int64           `json:"latency_ms"`
	Error     string          `json:"error,omitempty"`
}

type traceUsage struct {
	Prompt     int `json:"prompt"`
	Completion int `json:"completion"`
	// Cached — provider prefix-keshiga urilgan prompt tokenlar (DeepSeek
	// prompt_cache_hit_tokens). Xarajat diagnostikasi uchun: buyurtmachi kesh
	// urishini trace'dan ham ko'radi (metrikadan tashqari).
	Cached int `json:"cached,omitempty"`
}

type traceToolCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// serializeMessages xabarlarni trace formati uchun chizadi (keraksiz maydonlarsiz).
func serializeMessages(in []*schema.Message) []traceMessage {
	if len(in) == 0 {
		return nil
	}
	out := make([]traceMessage, 0, len(in))
	for _, m := range in {
		if m == nil {
			continue
		}
		out = append(out, traceMessage{
			Role:       string(m.Role),
			Content:    m.Content,
			ToolCalls:  traceToolCalls(m.ToolCalls),
			ToolCallID: m.ToolCallID,
			Name:       m.ToolName,
		})
	}
	return out
}

func traceToolCalls(calls []schema.ToolCall) []traceToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]traceToolCall, 0, len(calls))
	for _, tc := range calls {
		out = append(out, traceToolCall{Name: tc.Function.Name, Arguments: tc.Function.Arguments})
	}
	return out
}

// finalTraceCalls stream'dagi yig'ilgan tool_call'larni tartibli ro'yxatga aylantiradi.
func finalTraceCalls(calls map[int]*traceCallAcc, order []int) []traceToolCall {
	if len(order) == 0 {
		return nil
	}
	out := make([]traceToolCall, 0, len(order))
	for _, idx := range order {
		a := calls[idx]
		out = append(out, traceToolCall{Name: a.name, Arguments: a.args.String()})
	}
	return out
}

// traceCallAcc — stream bo'laklaridagi bitta tool_call yig'indisi.
// (id trace uchun kerak emas — faqat nom va argumentlar yoziladi.)
type traceCallAcc struct {
	name string
	args strings.Builder
}

func nowIso() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func sinceMs(start time.Time) int64 { return time.Since(start).Milliseconds() }
