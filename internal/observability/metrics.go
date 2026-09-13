package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Migratsiya: metrikalar nomlari rules.md §13.1 ro'yxatining o'zi — Django/Flutter
// tomondagi dashboard shu nomlarga tayanadi, o'zgartirilsa alert'lar buziladi.

// TTFT byudjeti p95 ≤ 1.5s, alert chegarasi 2s (rules §15, §13.3).
var ttftBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 1.5, 2, 3}

// To'liq javob 2 tool bilan ≤ 6s (rules §15).
var responseBuckets = []float64{0.25, 0.5, 1, 2, 3, 5, 8, 12}

// Tool byudjeti: lokal cache ≤ 5ms, CalcPreview ≤ 200ms (rules §15).
var toolBuckets = []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.2, 0.5, 1, 2}

var sttBuckets = []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 4, 8}

// Metrics — qayd etuvchi metrikalar. Har maydon alohida goroutine tomonidan
// ishlatiladi; Prometheus kollektorlari konkurrensiyaga chidamli. Global holat
// yo'q (rules §3.4) — main.go NewMetrics orqali DI qiladi.
type Metrics struct {
	ChatRequests          *prometheus.CounterVec   // {status}
	TTFTSeconds           prometheus.Histogram     // birinchi content tokenigacha
	ResponseSeconds       prometheus.Histogram     // to'liq javob
	ToolSeconds           *prometheus.HistogramVec // {tool}
	ToolErrors            *prometheus.CounterVec   // {tool}
	LLMTokens             *prometheus.CounterVec   // {provider,model,kind}
	LLMErrors             *prometheus.CounterVec   // {provider}
	LLMFallback           prometheus.Counter
	CatalogVersion        prometheus.Gauge
	CatalogStale          prometheus.Gauge // 1 = eskirgan (rules §4.1.5)
	CatalogAgeSeconds     prometheus.Gauge
	CatalogWatchConnected prometheus.Gauge // 1 = Watch oqimi ulangan
	OutboxPending         prometheus.Gauge
	OutboxDead            prometheus.Counter
	ActiveSessions        prometheus.Gauge
	VoiceSTTSeconds       prometheus.Histogram
	VoiceTTSChars         prometheus.Counter
}

// NewMetrics alohida registry bilan metrikalar to'plamini yaratadi. Alohida
// registry — default global registry'ga bog'liqlik yo'q (testlar va boshqa
// komponent hisoblagichlariga aralashmaydi, rules §3.4).
func NewMetrics() (*Metrics, *prometheus.Registry) {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		ChatRequests: newCounterVec("alora_chat_requests_total",
			"Suhbat so'rovlari soni, javob holati bo'yicha", "status"),
		TTFTSeconds: newHist("alora_ttft_seconds",
			"Birinchi content tokenigacha o'tgan vaqt (sekund)", ttftBuckets),
		ResponseSeconds: newHist("alora_response_seconds",
			"To'liq javob vaqti (sekund)", responseBuckets),
		ToolSeconds: newHistVec("alora_tool_seconds",
			"Tool bajarilish vaqti (sekund)", toolBuckets, "tool"),
		ToolErrors: newCounterVec("alora_tool_errors_total",
			"Tool xatoliklari soni (go error — foydalanuvchi xatosi emas)", "tool"),
		LLMTokens: newCounterVec("alora_llm_tokens_total",
			"LLM tokenlari: input/output/… (provider va model bo'yicha)",
			"provider", "model", "kind"),
		LLMErrors: newCounterVec("alora_llm_errors_total",
			"LLM chaqiruv xatolari soni", "provider"),
		LLMFallback: newCounter("alora_llm_fallback_total",
			"Zaxira provider'ga o'tishlar soni (rules §6.5)"),
		CatalogVersion: newGauge("alora_catalog_version",
			"Katalog versiyasi (monoton o'suvchi)"),
		CatalogStale: newGauge("alora_catalog_stale",
			"Katalog eskirganmi: 1 — ha, 0 — yo'q (rules §4.1.5)"),
		CatalogAgeSeconds: newGauge("alora_catalog_age_seconds",
			"Oxirgi sinxronlashdan beri o'tgan vaqt (sekund)"),
		CatalogWatchConnected: newGauge("alora_catalog_watch_connected",
			"Catalog.Watch oqimi ulanganmi: 1 — ha, 0 — uzilgan"),
		OutboxPending: newGauge("alora_outbox_pending",
			"Yuborilmagan yozuvlar navbatda qancha (outbox)"),
		OutboxDead: newCounter("alora_outbox_dead_total",
			"Retry tugagach tashlab yuborilgan yozuvlar soni"),
		ActiveSessions: newGauge("alora_active_sessions",
			"Hozirgi faol suhbat sessiyalari soni"),
		VoiceSTTSeconds: newHist("alora_voice_stt_seconds",
			"STT bajarilish vaqti (sekund)", sttBuckets),
		VoiceTTSChars: newCounter("alora_voice_tts_chars",
			"TTS orqali o'qilgan belgilar soni"),
	}

	reg.MustRegister(
		m.ChatRequests,
		m.TTFTSeconds,
		m.ResponseSeconds,
		m.ToolSeconds,
		m.ToolErrors,
		m.LLMTokens,
		m.LLMErrors,
		m.LLMFallback,
		m.CatalogVersion,
		m.CatalogStale,
		m.CatalogAgeSeconds,
		m.CatalogWatchConnected,
		m.OutboxPending,
		m.OutboxDead,
		m.ActiveSessions,
		m.VoiceSTTSeconds,
		m.VoiceTTSChars,
	)
	return m, reg
}

func newCounter(name, help string) prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{Name: name, Help: help})
}

func newCounterVec(name, help string, labels ...string) *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{Name: name, Help: help}, labels)
}

func newGauge(name, help string) prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{Name: name, Help: help})
}

func newHist(name, help string, buckets []float64) prometheus.Histogram {
	return prometheus.NewHistogram(prometheus.HistogramOpts{Name: name, Help: help, Buckets: buckets})
}

func newHistVec(name, help string, buckets []float64, labels ...string) *prometheus.HistogramVec {
	return prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: name, Help: help, Buckets: buckets}, labels)
}
