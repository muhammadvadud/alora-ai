package llm

import "github.com/cloudwego/eino/schema"

// TokenUsage — model javobidagi token hisobining yassi nusxasi (agent`ga shu
// ko'rinishda uzatiladi). Provider'ga xos bo'laklar (cached/reasoning) shu
// strukturaning o'z maydonlarida — provider shaklini agent bilmaydi (rules §6.3).
type TokenUsage struct {
	Prompt     int
	Completion int
	Cached     int
	Reasoning  int
}

// Usage model javobidagi token hisobini qaytaradi. ResponseMeta/Usage yo'q
// bo'lsa (ba'zi providerlar yubormaydi) ok=false va zero qiymat keladi.
// Reasoning() naqshi bilan bir xil uslub: agent emas, llm provider shaklini biladi.
func Usage(msg *schema.Message) (TokenUsage, bool) {
	if msg == nil || msg.ResponseMeta == nil || msg.ResponseMeta.Usage == nil {
		return TokenUsage{}, false
	}
	u := msg.ResponseMeta.Usage
	return TokenUsage{
		Prompt:     u.PromptTokens,
		Completion: u.CompletionTokens,
		// OpenAI'da cached/resoning alohida me'yorlarda (masalan deepseek-chat
		// cached prompt) — eino v0.9.12 ularni PromptTokenDetails/CompletionTokensDetails
		// da beradi; provider yubormasa 0 qoladi (eino schema komentida aytilgan).
		Cached:    u.PromptTokenDetails.CachedTokens,
		Reasoning: u.CompletionTokensDetails.ReasoningTokens,
	}, true
}
