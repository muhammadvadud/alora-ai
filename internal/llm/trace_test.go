package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// tracePath testlar uchun yangi JSONL fayl yo'lini beradi.
func tracePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "trace.jsonl")
}

// readTrace fayldagi har qatorni alohida JSON'ga bo'lib qaytaradi.
func readTrace(t *testing.T, path string) []map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("trace o'qish: %v", err)
	}
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("trace qatori JSON emas (%q): %v", line, err)
		}
		out = append(out, obj)
	}
	return out
}

func kindOf(t *testing.T, obj map[string]any) string {
	t.Helper()
	kind, _ := obj["kind"].(string)
	return kind
}

func TestWithTrace_Generate_RequestAndResponse(t *testing.T) {
	// Generate: bitta so'rov = 2 qator (request + response), content va
	// usage to'g'ri yoziladi. Bo'sh fayldan boshlaymiz (append ishlashini ham
	// ko'radigan bo'lamiz).
	path := tracePath(t)
	base := &mockModel{model: "trace-mock"}
	traced, err := WithTrace(base, path)
	if err != nil {
		t.Fatalf("WithTrace: %v", err)
	}

	in := []*schema.Message{
		schema.SystemMessage("sen yordamchisan"),
		schema.UserMessage("Salom, qandaysan?"),
	}
	msg, err := traced.Generate(context.Background(), in)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if msg == nil || msg.Content == "" {
		t.Fatalf("javob bo'sh: %+v", msg)
	}

	lines := readTrace(t, path)
	if len(lines) != 2 {
		t.Fatalf("qatorlar soni = %d, kutilgan 2", len(lines))
	}
	if kindOf(t, lines[0]) != "request" || kindOf(t, lines[1]) != "response" {
		t.Fatalf("kind tartibi noto'g'ri: %s, %s", lines[0]["kind"], lines[1]["kind"])
	}
	if lines[1]["content"] != msg.Content {
		t.Errorf("response content = %v, kutilgan %q", lines[1]["content"], msg.Content)
	}
	usage, ok := lines[1]["usage"].(map[string]any)
	if !ok || usage["prompt"] == nil || usage["completion"] == nil {
		t.Errorf("response usage yozilmadi: %v", lines[1]["usage"])
	}
	if _, ok := lines[1]["latency_ms"].(float64); !ok {
		t.Errorf("latency_ms yozilmadi: %v", lines[1]["latency_ms"])
	}

	msgs, ok := lines[0]["messages"].([]any)
	if !ok || len(msgs) != 2 {
		t.Fatalf("request messages yozilmadi: %v", lines[0]["messages"])
	}
	user := msgs[1].(map[string]any)
	if user["role"] != "user" || user["content"] != "Salom, qandaysan?" {
		t.Errorf("user xabari noto'g'ri: %v", user)
	}
}

func TestWithTrace_Generate_ToolCalls(t *testing.T) {
	// Tool chaqiruvchi javobda response qatoriga tool_calls (nom + argumentlar)
	// yozilishi kerak — kalitga o'xshash hech narsa yozilmaydi, faqat shu.
	path := tracePath(t)
	traced, err := WithTrace(&mockModel{model: "trace-mock"}, path)
	if err != nil {
		t.Fatalf("WithTrace: %v", err)
	}

	// mock "hisobla ... 3 va 4" ni curtain_calculate tool calliga aylantiradi.
	in := []*schema.Message{schema.UserMessage("xona o'lchamini hisobla 3 va 4")}
	if _, err := traced.Generate(context.Background(), in); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	lines := readTrace(t, path)
	resp := lines[1]
	calls, ok := resp["tool_calls"].([]any)
	if !ok || len(calls) != 1 {
		t.Fatalf("response tool_calls yozilmadi: %v", resp["tool_calls"])
	}
	call := calls[0].(map[string]any)
	if call["name"] != "curtain_calculate" {
		t.Errorf("tool nomi = %v, kutilgan curtain_calculate", call["name"])
	}
	args, ok := call["arguments"].(string)
	if !ok || !strings.Contains(args, "cornice_length_m") {
		t.Errorf("tool argumentlar JSON string bo'lishi kerak: %v", call["arguments"])
	}
}

func TestWithTrace_Stream_AggregatedChunks(t *testing.T) {
	// Streaming javob bo'laklarga bo'linib keladi, lekin faylga BITTА
	// response qatori yoziladi — biz o'qigan bo'laklar yig'indisiga teng.
	path := tracePath(t)
	traced, err := WithTrace(&mockModel{model: "trace-mock"}, path)
	if err != nil {
		t.Fatalf("WithTrace: %v", err)
	}

	in := []*schema.Message{schema.UserMessage("Salom, narx bormi?")}
	sr, err := traced.Stream(context.Background(), in)
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	defer sr.Close()
	var got strings.Builder
	for {
		chunk, recvErr := sr.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			t.Fatalf("Recv: %v", recvErr)
		}
		got.WriteString(chunk.Content)
	}
	if got.Len() == 0 {
		t.Fatal("bo'laklar o'tmadi")
	}

	lines := readTrace(t, path)
	if len(lines) != 2 {
		t.Fatalf("qatorlar soni = %d, kutilgan 2 (chunks yig'ilib 1 response)", len(lines))
	}
	if lines[1]["content"] != got.String() {
		t.Errorf("response content = %q, bo'laklar yig'indisi = %q", lines[1]["content"], got.String())
	}
	usage, ok := lines[1]["usage"].(map[string]any)
	if !ok || usage["prompt"] == nil {
		t.Errorf("stream usage yozilmadi: %v", lines[1]["usage"])
	}
}

func TestWithTrace_WithTools_WrapsAgain(t *testing.T) {
	// WithTools o'ramni SAQLAB yangi trace model qaytaradi (rules §6) —
	// aks holda agent tool'lar bog'lagach trace yo'qolardi.
	path := tracePath(t)
	traced, err := WithTrace(&mockModel{model: "trace-mock"}, path)
	if err != nil {
		t.Fatalf("WithTrace: %v", err)
	}
	bound, err := traced.WithTools([]*schema.ToolInfo{{Name: "product_search"}})
	if err != nil {
		t.Fatalf("WithTools: %v", err)
	}

	next, ok := bound.(*traceModel)
	if !ok {
		t.Fatalf("WithTools trace o'ramni saqlamadi: %T", bound)
	}
	if len(next.tools) != 1 || next.tools[0] != "product_search" {
		t.Errorf("tool nomlari o'tmadi: %v", next.tools)
	}

	// O'ralgan model ishlaydi va request'da tool nomi ko'rinadi.
	if _, err := next.Generate(context.Background(), []*schema.Message{schema.UserMessage("Salom")}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	lines := readTrace(t, path)
	tools, ok := lines[0]["tools"].([]any)
	if !ok || len(tools) != 1 || tools[0] != "product_search" {
		t.Errorf("request tools yozilmadi: %v", lines[0]["tools"])
	}
}

func TestWithTrace_InvalidPath_ReturnsError(t *testing.T) {
	// Fayl ochilolmasa startup'da aniq xato — "yomon trace bilan jonly ishlash" yo'q.
	_, err := WithTrace(&mockModel{model: "trace-mock"}, filepath.Join(t.TempDir(), "yo'q", "trace.jsonl"))
	if err == nil || !strings.Contains(err.Error(), "trace faylini ochish") {
		t.Fatalf("ochilmaydigan yo'l xato berishi kerak, oldik: %v", err)
	}
}

func TestWithTrace_ParallelCalls_LinesNotInterleaved(t *testing.T) {
	// Parallel so'rovlar qatorlarni bir-biriga aralashtirmasligi kerak: har bir
	// qator to'liq JSON bo'lib qoladi (mutex + bitta Write). Har chaqiruv
	// request+response juftligi bilan tugaydi.
	const calls = 8
	path := tracePath(t)
	traced, err := WithTrace(&mockModel{model: "trace-mock"}, path)
	if err != nil {
		t.Fatalf("WithTrace: %v", err)
	}
	bound, err := traced.WithTools([]*schema.ToolInfo{{Name: "test_tool"}})
	if err != nil {
		t.Fatalf("WithTools: %v", err)
	}

	var wg sync.WaitGroup
	errs := make(chan error, calls)
	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			in := []*schema.Message{schema.UserMessage(fmt.Sprintf("so'rov %d", n))}
			msg, genErr := bound.Generate(context.Background(), in)
			if genErr != nil {
				errs <- genErr
			}
			if msg == nil {
				errs <- fmt.Errorf("javob nil")
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatalf("Generate: %v", e)
	}

	lines := readTrace(t, path)
	if len(lines) != calls*2 {
		t.Fatalf("qatorlar soni = %d, kutilgan %d", len(lines), calls*2)
	}
	var req, resp int
	seen := make(map[string]bool)
	for _, line := range lines {
		switch kindOf(t, line) {
		case "request":
			req++
			// Har so'rovning user xabari butun holda saqlanib qolishi
			// kerak — bir so'rovning matni boshqasiga aralashmaydi.
			var content string
			if msgs, ok := line["messages"].([]any); ok {
				for _, m := range msgs {
					obj, _ := m.(map[string]any)
					if obj["role"] == "user" {
						content, _ = obj["content"].(string)
					}
				}
			}
			if !strings.HasPrefix(content, "so'rov ") || seen[content] {
				t.Errorf("request content aralashib ketgan: %q", content)
			}
			seen[content] = true
			if tools, _ := line["tools"].([]any); len(tools) != 1 || tools[0] != "test_tool" {
				t.Errorf("tools yozilmadi: %v", line["tools"])
			}
		case "response":
			resp++
			if _, ok := line["latency_ms"].(float64); !ok {
				t.Errorf("response latency yozilmadi: %v", line["latency_ms"])
			}
		default:
			t.Errorf("noma'lum kind: %v", line["kind"])
		}
	}
	if req != calls || resp != calls {
		t.Fatalf("request=%d response=%d, kutilgan %d/%d", req, resp, calls, calls)
	}
}

func TestWithTrace_NilModel(t *testing.T) {
	if _, err := WithTrace(nil, tracePath(t)); err == nil {
		t.Fatal("nil model xato berishi kerak")
	}
}

var _ model.ToolCallingChatModel = (*traceModel)(nil)
