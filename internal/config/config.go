// Package config Viper orqali YAML + muhit o'zgaruvchilaridan sozlamalarni yuklaydi.
//
// ENV ustuni: barcha kalitlar ALORA_ prefiksi bilan (ALORA_LLM_MODEL → llm.model).
// API kalitlar alohida ENV'dan (DEEPSEEK_API_KEY va h.k.) — prefikssiz,
// faqat resolveAPIKey orqali ingl. config.yaml da hech qachon kalit bo'lmaydi.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Mode      string          `mapstructure:"mode"` // grpc | cli | http
	LogLevel  string          `mapstructure:"log_level"`
	LLM       LLMConfig       `mapstructure:"llm"`
	Agent     AgentConfig     `mapstructure:"agent"`
	Server    ServerConfig    `mapstructure:"server"` // legacy: http transportidagi eski port
	GRPC      GRPCConfig      `mapstructure:"grpc"`
	HTTP      HTTPConfig      `mapstructure:"http"`
	Backend   BackendConfig   `mapstructure:"backend"`
	Catalog   CatalogConfig   `mapstructure:"catalog"`
	Knowledge KnowledgeConfig `mapstructure:"knowledge"`
	STT       STTConfig       `mapstructure:"stt"`
	TTS       TTSConfig       `mapstructure:"tts"`
	DB        DBConfig        `mapstructure:"db"`
	Fallback  FallbackConfig  `mapstructure:"fallback"`
}

// DBConfig — suhbat tarixi uchun PostgreSQL. URL bo'sh bo'lsa in-memory ishlatiladi.
// URL faqat ENV'dan: DATABASE_URL
type DBConfig struct {
	URL         string `mapstructure:"url"`
	AutoMigrate bool   `mapstructure:"auto_migrate"` // dev'da ishga tushganda migratsiya
}

type LLMConfig struct {
	Provider    string        `mapstructure:"provider"` // deepseek | openai | claude | ollama
	APIKey      string        `mapstructure:"api_key"`
	Model       string        `mapstructure:"model"`
	BaseURL     string        `mapstructure:"base_url"`
	MaxTokens   int           `mapstructure:"max_tokens"`
	Temperature float32       `mapstructure:"temperature"`
	Timeout     time.Duration `mapstructure:"timeout"`
	// Thinking — model "o'ylash" (reasoning) rejimi: adaptive | enabled | disabled.
	// Bo'sh — provider standarti. ENV: LLM_THINKING (ALORA_ prefiksi bilan).
	//
	// Hozircha faqat deepseek provideri qo'llaydi.
	Thinking string `mapstructure:"thinking"`
	// ReasoningEffort — o'ylash chuqurligi: low | medium | high | xhigh | max.
	// Bo'sh — provider standarti. ENV: LLM_REASONING_EFFORT (ALORA_ prefiksi bilan).
	// Faqat thinking o'chirilmagan bo'lsa ma'noga ega.
	ReasoningEffort string `mapstructure:"reasoning_effort"`
	// FallbackProvider — asosiy provider ketma-ket 2 marta xato bersa, o'sha navbat
	// shu provider bilan qayta so'raladi (rules.md §6.5). ENV: ALORA_LLM_FALLBACK_PROVIDER
	FallbackProvider string `mapstructure:"fallback_provider"`
	// TraceFile — LLM'ga ketgan so'rov va kelgan javob yoziladigan JSONL fayl
	// (debug trace, buyurtmachi ko'radi). Bo'sh — trace o'chiq. Kalit yozilmaydi.
	// ENV: ALORA_LLM_TRACE_FILE
	TraceFile string `mapstructure:"trace_file"`
}

// Thinking rejimlari (DeepSeek API: thinking.type).
const (
	ThinkingAdaptive = "adaptive" // model o'zi qaror qiladi
	ThinkingEnabled  = "enabled"
	ThinkingDisabled = "disabled"
)

// O'ylash chuqurligi darajalari (DeepSeek API: reasoning_effort), pastdan yuqoriga.
const (
	EffortLow    = "low"
	EffortMedium = "medium"
	EffortHigh   = "high"
	EffortXHigh  = "xhigh"
	EffortMax    = "max"
)

type AgentConfig struct {
	// MaxStep — ReAct siklining maksimal qadami (tool -> model -> tool ...).
	MaxStep int `mapstructure:"max_step"`
	// MaxHistory — modelga yuboriladigan eski xabarlar chegarasi. Har navbat 4+ xabar
	// qo'shadi, ya'ni ~40 ≈ 8-10 navbat. Uzun suhbat oqimida oshiring.
	// ENV: ALORA_AGENT_MAX_HISTORY
	MaxHistory int `mapstructure:"max_history"`
	// MaxToolResultChars — eski tool natijasining modelga yuboriladigan nusxasi
	// uchun rune chegarasi. ENV: ALORA_AGENT_MAX_TOOL_RESULT_CHARS
	MaxToolResultChars int `mapstructure:"max_tool_result_chars"`
	// SystemPrompt — fallback prompt (Django sinxronlanmaganda). O'rniga
	// $fallback.system_prompt dan foydalaniladi; eski joylashuv uchun saqlanadi.
	SystemPrompt string `mapstructure:"system_prompt"`
	// Stream — javobni jonli (bo'lak-bo'lak) yuborish. false bo'lsa javob yaxlit keladi.
	// Standart: true. ENV: ALORA_AGENT_STREAM=false
	Stream bool `mapstructure:"stream"`
}

type ServerConfig struct {
	Addr string `mapstructure:"addr"`
}

// GRPCConfig — asosiy transport (rules.md §8). Token nomi ENV'dan o'qiladi;
// qiymatning o'zi config'da bo'lmaydi.
type GRPCConfig struct {
	Addr               string        `mapstructure:"addr"`
	ServiceTokenEnv    string        `mapstructure:"service_token_env"`
	AuthRequired       bool          `mapstructure:"auth_required"`
	MaxSessions        int           `mapstructure:"max_sessions"`
	RateLimitPerMinute int           `mapstructure:"rate_limit_per_minute"`
	MaxMessageChars    int           `mapstructure:"max_message_chars"`
	Keepalive          time.Duration `mapstructure:"keepalive"`
	TLS                TLSConfig     `mapstructure:"tls"`
}

// HTTPConfig — faqat /healthz, /readyz, /metrics (rules.md §0.7); biznes endpoint yo'q.
type HTTPConfig struct {
	Addr string `mapstructure:"addr"`
}

// BackendConfig — Go → Django gRPC klient (rules.md §9).
type BackendConfig struct {
	Addr    string        `mapstructure:"addr"`
	Timeout time.Duration `mapstructure:"timeout"`
	Retry   int           `mapstructure:"retry"`
	TLS     TLSConfig     `mapstructure:"tls"`
}

// TLSConfig faqat sertifikat ENV nomlarini saqlaydi; maxfiy qiymatlar config'da bo'lmaydi.
type TLSConfig struct {
	Enabled     bool   `mapstructure:"enabled"`
	CertFileEnv string `mapstructure:"cert_file_env"`
	KeyFileEnv  string `mapstructure:"key_file_env"`
	CAFileEnv   string `mapstructure:"ca_file_env"`
}

// CatalogConfig — katalog cache sinxroni (rules.md §4.1).
type CatalogConfig struct {
	SnapshotOnStart bool          `mapstructure:"snapshot_on_start"`
	StaleAfter      time.Duration `mapstructure:"stale_after"`
	PageSize        int           `mapstructure:"page_size"`
}

// KnowledgeConfig — bilim qoidalarini prompt'ga joylash chegarasi (rules.md §4.6.2).
type KnowledgeConfig struct {
	InlineMax int `mapstructure:"inline_max"`
}

// STTConfig — nutqni matnga aylantirish (rules.md §11).
type STTConfig struct {
	Provider string `mapstructure:"provider"`
	Language string `mapstructure:"language"`
}

// TTSConfig — matnni nutqqa aylantirish (rules.md §11).
type TTSConfig struct {
	Provider string  `mapstructure:"provider"`
	Voice    string  `mapstructure:"voice"`
	Speed    float64 `mapstructure:"speed"`
}

// FallbackConfig — faqat Django ulanmaganda ishlatiladigan qiymatlar (rules.md §12.3).
type FallbackConfig struct {
	SystemPrompt string `mapstructure:"system_prompt"`
}

// providerKeyEnv har provider uchun API-kalit muhit o'zgaruvchisi (faqat ENV).
// Bo'sh qiymat — kalit talab qilinmaydi (masalan Ollama lokal).
// stt/tts providerlari ham shu xaritada — bitta ENV nomi manbai bo'lsin.
var providerKeyEnv = map[string]string{
	"mock":       "",
	"deepseek":   "DEEPSEEK_API_KEY",
	"openai":     "OPENAI_API_KEY",
	"claude":     "ANTHROPIC_API_KEY",
	"ollama":     "",
	"whisper":    "OPENAI_API_KEY",
	"elevenlabs": "ELEVENLABS_API_KEY",
	"google":     "GOOGLE_API_KEY",
}

// Load config faylini o'qiydi, ALORA_ muhit o'zgaruvchilari bilan ustidan yozadi va
// tanlangan provider'ga qarab API kalitni tegishli ENV'dan yechadi. So'ng Validate().
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// Standart qiymatlar (config.yaml yetmaydigan/bo'sh bo'lsa).
	v.SetDefault("llm.provider", "deepseek")
	v.SetDefault("llm.model", "deepseek-v4-pro")
	v.SetDefault("llm.base_url", "https://api.deepseek.com/v1")
	v.SetDefault("llm.max_tokens", 2048)
	v.SetDefault("llm.temperature", 0.7)
	v.SetDefault("llm.timeout", "60s")
	v.SetDefault("llm.thinking", "")         // bo'sh default — env override (ALORA_LLM_THINKING) ishlashi uchun
	v.SetDefault("llm.reasoning_effort", "") // bir xil sabab: bo'sh default, env override zarur
	v.SetDefault("llm.fallback_provider", "claude")
	v.SetDefault("llm.trace_file", "")
	v.SetDefault("agent.max_step", 20)
	v.SetDefault("agent.max_history", 40)
	v.SetDefault("agent.max_tool_result_chars", 3000)
	v.SetDefault("agent.stream", true)
	v.SetDefault("agent.system_prompt", "")
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("grpc.addr", ":9090")
	v.SetDefault("grpc.service_token_env", "AGENT_SERVICE_TOKEN")
	v.SetDefault("grpc.auth_required", true)
	v.SetDefault("grpc.max_sessions", 100)
	v.SetDefault("grpc.rate_limit_per_minute", 20)
	v.SetDefault("grpc.max_message_chars", 4000)
	v.SetDefault("grpc.keepalive", "30s")
	v.SetDefault("grpc.tls.enabled", false)
	v.SetDefault("grpc.tls.cert_file_env", "ALORA_GRPC_TLS_CERT_FILE")
	v.SetDefault("grpc.tls.key_file_env", "ALORA_GRPC_TLS_KEY_FILE")
	v.SetDefault("grpc.tls.ca_file_env", "ALORA_GRPC_TLS_CA_FILE")
	v.SetDefault("http.addr", ":8081")
	v.SetDefault("backend.addr", "django:9091")
	v.SetDefault("backend.timeout", "5s")
	v.SetDefault("backend.retry", 3)
	v.SetDefault("backend.tls.enabled", false)
	v.SetDefault("backend.tls.cert_file_env", "ALORA_BACKEND_TLS_CERT_FILE")
	v.SetDefault("backend.tls.key_file_env", "ALORA_BACKEND_TLS_KEY_FILE")
	v.SetDefault("backend.tls.ca_file_env", "ALORA_BACKEND_TLS_CA_FILE")
	v.SetDefault("catalog.snapshot_on_start", true)
	v.SetDefault("catalog.stale_after", "10m")
	v.SetDefault("catalog.page_size", 500)
	v.SetDefault("knowledge.inline_max", 50)
	v.SetDefault("stt.provider", "whisper")
	v.SetDefault("stt.language", "uz")
	v.SetDefault("tts.provider", "elevenlabs")
	v.SetDefault("tts.voice", "")
	v.SetDefault("tts.speed", 1.0)
	v.SetDefault("db.auto_migrate", false)
	v.SetDefault("db.url", "")
	v.SetDefault("fallback.system_prompt", "")
	v.SetDefault("mode", "cli")
	v.SetDefault("log_level", "info")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("config faylini o'qish: %w", err)
	}

	// ENV ustuni: ALORA_ prefiks, `.` → `_` (ALORA_LLM_MODEL → llm.model).
	v.SetEnvPrefix("ALORA")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config'ni tahlil qilish: %w", err)
	}

	// Prefiksga bo'ysinmaydigan maxsus ENV'lar (oyna yoki shartnoma bilan o'rnatilgan).
	if cfg.DB.URL == "" {
		cfg.DB.URL = os.Getenv("DATABASE_URL")
	}

	if err := cfg.resolveAPIKey(); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// resolveAPIKey provider'ga mos ENV'dan kalitni oladi va zarurligini tekshiradi.
func (c *Config) resolveAPIKey() error {
	envName, known := providerKeyEnv[c.LLM.Provider]
	if !known {
		return fmt.Errorf("noma'lum provider %q (deepseek, openai, claude, ollama, mock)", c.LLM.Provider)
	}
	if envName == "" {
		return nil // kalit talab qilinmaydi
	}
	if c.LLM.APIKey == "" {
		c.LLM.APIKey = os.Getenv(envName)
	}
	if c.LLM.APIKey == "" {
		return fmt.Errorf("%s o'rnatilmagan (%s provideri uchun zarur)", envName, c.LLM.Provider)
	}
	return nil
}

// Validate konfiguratsiya qiymatlarini tekshiradi — noto'g'ri bo'lsa ishga tushmaydi
// (rules.md §12.1). Xato matni o'zbekcha, maydon nomi bilan.
func (c *Config) Validate() error {
	switch c.Mode {
	case "grpc", "cli", "http":
	default:
		return fmt.Errorf("mode %q noto'g'ri (grpc, cli yoki http)", c.Mode)
	}
	// gRPC — katalog va backend bilan ishlaydigan asosiy rejim: Django manzili zarur.
	if c.Mode == "grpc" && strings.TrimSpace(c.Backend.Addr) == "" {
		return fmt.Errorf("grpc rejimida backend.addr bo'sh bo'lishi mumkin emas")
	}
	if c.Catalog.StaleAfter <= 0 {
		return fmt.Errorf("catalog.stale_after %v noto'g'ri — 0 dan katta bo'lishi kerak", c.Catalog.StaleAfter)
	}
	if c.Catalog.PageSize < 1 || c.Catalog.PageSize > 1000 {
		return fmt.Errorf("catalog.page_size %d noto'g'ri — 1..1000 oralig'ida bo'lishi kerak", c.Catalog.PageSize)
	}
	if c.LLM.Model == "" {
		return fmt.Errorf("llm.model bo'sh bo'lishi mumkin emas")
	}
	if c.GRPC.RateLimitPerMinute < 0 {
		return fmt.Errorf("grpc.rate_limit_per_minute noto'g'ri")
	}
	if c.GRPC.MaxMessageChars < 0 {
		return fmt.Errorf("grpc.max_message_chars noto'g'ri")
	}
	switch c.LLM.Thinking {
	case "", ThinkingAdaptive, ThinkingEnabled, ThinkingDisabled:
	default:
		return fmt.Errorf("llm.thinking %q noto'g'ri (%s, %s yoki %s)",
			c.LLM.Thinking, ThinkingAdaptive, ThinkingEnabled, ThinkingDisabled)
	}
	switch c.LLM.ReasoningEffort {
	case "", EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax:
	default:
		return fmt.Errorf("llm.reasoning_effort %q noto'g'ri (%s, %s, %s, %s yoki %s)",
			c.LLM.ReasoningEffort, EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax)
	}
	return nil
}
