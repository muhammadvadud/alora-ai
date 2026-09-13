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
	KindDone       Kind = "done"        // javob tugadi
	KindError      Kind = "error"       // xato yuz berdi
)

// Event agent ishlashi davomida yuzaga keladigan hodisa.
// Transport qatlami (CLI/HTTP) uni o'ziga mos ko'rinishda render qiladi.
type Event struct {
	Kind Kind `json:"kind"`
	// Content — KindContent uchun javob matni, KindThinking uchun o'ylash matni.
	Content string `json:"content,omitempty"`
	Tool    string `json:"tool,omitempty"`
	Args    string `json:"args,omitempty"`
	Result  string `json:"result,omitempty"`
	Err     string `json:"error,omitempty"`
}
