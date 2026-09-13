package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/muhammadvadud/alora-agent/internal/config"
)

func init() { register("mock", buildMock) }

type mockModel struct {
	model string

	mu     sync.Mutex
	callID int
}

func buildMock(_ context.Context, cfg config.LLMConfig) (model.ToolCallingChatModel, error) {
	name := cfg.Model
	if name == "" {
		name = "alora-mock"
	}
	return &mockModel{model: name}, nil
}

func (m *mockModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	// Mock tool sxemasini ishlatmaydi, ammo agent kutgan model wrapper kontraktini
	// saqlaydi: har bir WithTools chaqiruvi shu deterministik modelning nusxasini qaytaradi.
	return &mockModel{model: m.model}, nil
}

func (m *mockModel) Generate(ctx context.Context, messages []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	stream, err := m.Stream(ctx, messages)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	var out schema.Message
	for {
		chunk, recvErr := stream.Recv()
		if recvErr != nil {
			if errors.Is(recvErr, io.EOF) {
				break
			}
			return nil, recvErr
		}
		out.Role = chunk.Role
		out.Content += chunk.Content
		out.ToolCalls = append(out.ToolCalls, chunk.ToolCalls...)
		if chunk.ResponseMeta != nil {
			out.ResponseMeta = chunk.ResponseMeta
		}
	}
	return &out, nil
}

func (m *mockModel) Stream(_ context.Context, messages []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	lastUser := -1
	for i, message := range messages {
		if message != nil && message.Role == schema.User {
			lastUser = i
		}
	}
	if lastUser < 0 {
		return nil, fmt.Errorf("mock: foydalanuvchi xabari topilmadi")
	}

	input := messages[lastUser].Content
	turn := messages[lastUser+1:]
	if isCalculationRequest(input) {
		return schema.StreamReaderFromArray([]*schema.Message{m.calculationCall(input, messages)}), nil
	}
	if isHotelRequest(input) {
		if !hasTool(turn, "set_context") {
			return schema.StreamReaderFromArray([]*schema.Message{m.toolCall("set_context", map[string]any{
				"room": "living", "style": "hi_tech",
			})}), nil
		}
		if !hasTool(turn, "product_search") {
			return schema.StreamReaderFromArray([]*schema.Message{m.toolCall("product_search", map[string]any{
				"room": "living", "style": "hi_tech", "limit": 3,
			})}), nil
		}
		return schema.StreamReaderFromArray(textChunks(productSummary(messages))), nil
	}
	return schema.StreamReaderFromArray(textChunks("Savolingizni aniqroq yozing, iltimos.")), nil
}

func (m *mockModel) toolCall(name string, args map[string]any) *schema.Message {
	raw, _ := json.Marshal(args)
	m.mu.Lock()
	m.callID++
	id := fmt.Sprintf("mock_call_%02d", m.callID)
	m.mu.Unlock()
	return &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{{
			ID: id, Type: "function", Function: schema.FunctionCall{Name: name, Arguments: string(raw)},
		}},
		ResponseMeta: mockUsage(),
	}
}

func (m *mockModel) calculationCall(input string, messages []*schema.Message) *schema.Message {
	numbers := numberRE.FindAllString(input, -1)
	length, height := "", ""
	if len(numbers) >= 2 {
		length, height = normalizeNumber(numbers[0]), normalizeNumber(numbers[1])
	}
	variantID := firstVariantID(messages)
	args := map[string]any{
		"cornice_length_m": parseNumber(length),
		"height_m":         parseNumber(height),
		"room_name":        "living",
		"parts": []map[string]any{{
			"variant_id": variantID, "role": "portyera", "coefficient": 2.5,
		}},
	}
	return m.toolCall("curtain_calculate", args)
}

func mockUsage() *schema.ResponseMeta {
	return &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 24, CompletionTokens: 8, TotalTokens: 32}}
}

var numberRE = regexp.MustCompile(`\d+(?:[.,]\d+)?`)

func isHotelRequest(input string) bool {
	lower := strings.ToLower(input)
	return strings.Contains(lower, "mehmonxona") || strings.Contains(lower, "hotel")
}

func isCalculationRequest(input string) bool {
	lower := strings.ToLower(input)
	return (strings.Contains(lower, "hisobla") || strings.Contains(lower, "o'lcham") || strings.Contains(lower, "o‘lcham")) && len(numberRE.FindAllString(input, -1)) >= 2
}

func hasTool(messages []*schema.Message, name string) bool {
	for _, message := range messages {
		if message != nil && message.Role == schema.Tool && message.ToolName == name {
			return true
		}
	}
	return false
}

func textChunks(text string) []*schema.Message {
	if text == "" {
		return []*schema.Message{{Role: schema.Assistant, ResponseMeta: mockUsage()}}
	}
	parts := make([]*schema.Message, 0, 3)
	runes := []rune(text)
	for i := 0; i < 3 && len(runes) > 0; i++ {
		remaining := 3 - i
		n := (len(runes) + remaining - 1) / remaining
		parts = append(parts, &schema.Message{Role: schema.Assistant, Content: string(runes[:n])})
		runes = runes[n:]
	}
	parts[len(parts)-1].ResponseMeta = mockUsage()
	return parts
}

func productSummary(messages []*schema.Message) string {
	names := make([]string, 0, 3)
	for i := len(messages) - 1; i >= 0 && len(names) < 3; i-- {
		message := messages[i]
		if message == nil || message.Role != schema.Tool || message.ToolName != "product_search" {
			continue
		}
		var result struct {
			Products []struct {
				Name string `json:"name"`
			} `json:"products"`
		}
		if json.Unmarshal([]byte(message.Content), &result) != nil {
			continue
		}
		for _, product := range result.Products {
			if product.Name != "" {
				names = append(names, product.Name)
				if len(names) == 3 {
					break
				}
			}
		}
	}
	if len(names) == 0 {
		return "Katalogda mos mato topilmadi."
	}
	return "Mana mos matolar: " + strings.Join(names, ", ")
}

func firstVariantID(messages []*schema.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		message := messages[i]
		if message == nil || message.Role != schema.Tool || message.ToolName != "product_search" {
			continue
		}
		var result struct {
			Products []struct {
				VariantID string `json:"variant_id"`
			} `json:"products"`
		}
		if json.Unmarshal([]byte(message.Content), &result) == nil && len(result.Products) > 0 && result.Products[0].VariantID != "" {
			return result.Products[0].VariantID
		}
	}
	return "mock-variant"
}

func normalizeNumber(raw string) string { return strings.ReplaceAll(raw, ",", ".") }

func parseNumber(raw string) float64 {
	var value float64
	_, _ = fmt.Sscanf(raw, "%f", &value)
	return value
}
