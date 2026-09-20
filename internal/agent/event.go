package agent

// Kind Event turini bildiradi.
type Kind string

const (
	KindStart      Kind = "start"       // agent so'rov ustida ishlashni boshladi
	KindThinking   Kind = "thinking"    // model "o'ylashi" (reasoning) bo'lagi — javob emas
	KindContent    Kind = "content"     // javob matni bo'lagi
	KindToolCall   Kind = "tool_call"   // tool nomi aniqlandi (model oqimida)
	KindToolArgs   Kind = "tool_args"   // tool argumentlari to'ldi
	KindToolResult Kind = "tool_result" // tool natija qaytardi
	KindProducts   Kind = "products"    // tool katalogdan mahsulotlar topdi — ilova kartochka chizadi
	KindStage      Kind = "stage"       // javob tugashidan oldin joriy suhbat bosqichi
	KindTitle      Kind = "title"       // birinchi navbatdan keyin AI qo'ygan suhbat sarlavhasi (Content)
	KindDone       Kind = "done"        // javob tugadi (Usage bilan birga)
	KindError      Kind = "error"       // xato yuz berdi
)

// Event agent ishlashi davomida yuzaga keladigan hodisa.
// Transport qatlami (CLI/HTTP) uni o'ziga mos ko'rinishda render qiladi.
type Event struct {
	Kind Kind `json:"kind"`
	// Content — KindContent uchun javob matni, KindThinking uchun o'ylash matni,
	// KindStage uchun bosqich nomi, KindTitle uchun suhbat sarlavhasi.
	Content string `json:"content,omitempty"`
	Tool    string `json:"tool,omitempty"`
	Args    string `json:"args,omitempty"`
	Result  string `json:"result,omitempty"`
	// Products — KindProducts uchun: tool topgan mahsulotlar (Flutter kartochka chizadi).
	Products []ProductRef `json:"products,omitempty"`
	// Usage — KindDone bilan birga keladi (rules §6.2).
	Usage *Usage `json:"usage,omitempty"`
	Err   string `json:"error,omitempty"`
}

// Usage — bitta so'rov (navbat) davomida sarflangan tokenlar va kechikish.
// Narx Go'da hisoblanmaydi (ADR-005): uni Django ModelPricing o'zi chiqaradi —
// Go faqat token sonlari va latency yuboradi (rules §6.3).
type Usage struct {
	Provider         string   `json:"provider"`
	Model            string   `json:"model"`
	PromptTokens     int      `json:"prompt_tokens"`
	CompletionTokens int      `json:"completion_tokens"`
	CachedTokens     int      `json:"cached_tokens,omitempty"`
	ReasoningTokens  int      `json:"reasoning_tokens,omitempty"`
	ToolCalls        []string `json:"tool_calls,omitempty"`
	LatencyMs        int64    `json:"latency_ms"`
}
