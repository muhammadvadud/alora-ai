//go:build load

package load

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/shopspring/decimal"

	"github.com/muhammadvadud/alora-agent/internal/curtains"
)

type loadMockModel struct{}

func (m *loadMockModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return nil, fmt.Errorf("Generate chaqirilmasligi kerak")
}

func (m *loadMockModel) Stream(_ context.Context, msgs []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	last := msgs[len(msgs)-1]

	if last.Role == schema.User {
		idx := 0
		msg := &schema.Message{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				Index:    &idx,
				ID:       "call_123",
				Type:     "function",
				Function: schema.FunctionCall{Name: "product_search", Arguments: `{"room":"bedroom"}`},
			}},
		}
		return schema.StreamReaderFromArray([]*schema.Message{msg}), nil
	}

	msg1 := &schema.Message{Role: schema.Assistant, Content: "Mana natijalar: "}
	msg2 := &schema.Message{
		Role:         schema.Assistant,
		Content:      "Tugadi.",
		ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{PromptTokens: 10, CompletionTokens: 5}},
	}
	return schema.StreamReaderFromArray([]*schema.Message{msg1, msg2}), nil
}

func (m *loadMockModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

type fakeCalc struct{}

func (f fakeCalc) Calculate(_ context.Context, _ curtains.CalcInput) (curtains.CalcResult, error) {
	return curtains.CalcResult{}, nil
}

type fakeOrders struct{}

func (f fakeOrders) SaveOrderDraft(_ context.Context, _ curtains.OrderDraft) (curtains.OrderDraftResult, error) {
	return curtains.OrderDraftResult{OrderID: "123", OrderNumber: "ORD-123"}, nil
}

func setupCatalog() *curtains.Catalog {
	cat := curtains.NewCatalog(nil, 10*time.Minute)
	cat.ApplySnapshot(curtains.Snapshot{
		Version: 1,
		Products: []curtains.Product{{
			ID: "p1", Name: "Parda",
			VariantIDs: []string{"v1"},
		}},
		Variants: []curtains.Variant{{
			ID: "v1", ProductID: "p1", Price: decimal.NewFromInt(1000), PriceUnit: curtains.PerSqm, IsActive: true,
		}},
	})
	return cat
}
