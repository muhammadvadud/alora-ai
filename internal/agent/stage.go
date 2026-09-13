package agent

import "context"

// StageReporter — suhbat bosqichini transportga ko'rsatadigan hook.
// Domen (SessionState) agent'da EMAS: main.go shu interfeysni curtains bilan
// bog'laydi (rules §2 — agent domenni bilmaydi). nil bo'lsa agent bosqich
// haqida hech narsa chiqarmaydi.
type StageReporter interface {
	// OnToolCall — har tool bajarilgach chaqiriladi (state.Apply uchun).
	OnToolCall(ctx context.Context, sessionID, tool, argsJSON string)
	// Stage — joriy bosqich nomi va qisqa JSON xulosani qaytaradi. Ular
	// KindStage event'ining Content/Result maydonlariga tushadi (rules §4.5.5).
	Stage(ctx context.Context, sessionID string) (stage, summaryJSON string)
}
