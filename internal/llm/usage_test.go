package llm

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestUsage_WithResponseMeta(t *testing.T) {
	msg := &schema.Message{
		Role:    schema.Assistant,
		Content: "javob",
		ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 25,
			TotalTokens:      125,
			PromptTokenDetails: schema.PromptTokenDetails{
				CachedTokens: 40,
			},
			CompletionTokensDetails: schema.CompletionTokensDetails{
				ReasoningTokens: 12,
			},
		}},
	}

	u, ok := Usage(msg)
	if !ok {
		t.Fatal("Usage mavjud bo'lsa ham ok=false qaytdi")
	}
	if u.Prompt != 100 || u.Completion != 25 || u.Cached != 40 || u.Reasoning != 12 {
		t.Errorf("noto'g'ri hisob: %+v, want {100 25 40 12}", u)
	}
}

// Provider bo'laklarni yubormasa (cached/reasoning yo'q) — 0 qoladi, ok=true.
func TestUsage_WithoutProviderDetails(t *testing.T) {
	msg := &schema.Message{
		Role: schema.Assistant,
		ResponseMeta: &schema.ResponseMeta{Usage: &schema.TokenUsage{
			PromptTokens:     7,
			CompletionTokens: 3,
		}},
	}

	u, ok := Usage(msg)
	if !ok {
		t.Fatal("Usage mavjud bo'lsa ham ok=false qaytdi")
	}
	if u.Prompt != 7 || u.Completion != 3 || u.Cached != 0 || u.Reasoning != 0 {
		t.Errorf("noto'g'ri hisob: %+v, want {7 3 0 0}", u)
	}
}

// Ba'zi providerlar ResponseMeta/Usage umuman yubormaydi — ok=false, zero qiymat.
func TestUsage_WithoutResponseMeta(t *testing.T) {
	if u, ok := Usage(&schema.Message{Role: schema.Assistant, Content: "javob"}); ok {
		t.Errorf("ResponseMeta yo'q bo'lsa ok=true qaytdi: %+v", u)
	}
	if u, ok := Usage(&schema.Message{
		Role:         schema.Assistant,
		ResponseMeta: &schema.ResponseMeta{}, // Usage nil
	}); ok {
		t.Errorf("Usage nil bo'lsa ok=true qaytdi: %+v", u)
	}
	if u, ok := Usage(nil); ok {
		t.Errorf("nil message uchun ok=true qaytdi: %+v", u)
	}
}
