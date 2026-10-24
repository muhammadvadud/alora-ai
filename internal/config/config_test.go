package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// minimalYaml — testlar uchun asos bo'ladi; kerakli joyidan tahrirlanadi.
const minimalYaml = `mode: cli
llm:
  provider: deepseek
`

// writeConfig vaqtinchalik katalogda minimal config yozadi.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("config yozish: %v", err)
	}
	return path
}

// loadForTest DEEPSEEK_API_KEY ni qo'yib Load chaqiradi (default provider deepseek).
func loadForTest(t *testing.T, path string) (*Config, error) {
	t.Helper()
	t.Setenv("DEEPSEEK_API_KEY", "test-key")
	return Load(path)
}

func TestLoad_ShippedConfig(t *testing.T) {
	// Repodagi configs/config.yaml doim to'g'ri yuklanishi shart — bu sinov
	// konfiguratsiyaning "kalitsiz, yaroqli holatda" saqlanishini qo'riqlaydi.
	t.Setenv("DEEPSEEK_API_KEY", "test-key")
	cfg, err := Load("../../configs/config.yaml")
	if err != nil {
		t.Fatalf("yuklangan configs/config.yaml xato berdi: %v", err)
	}
	if cfg.Mode != "grpc" {
		t.Errorf("mode = %q, kutilgan %q", cfg.Mode, "grpc")
	}
	if cfg.Backend.Addr != "django:9091" {
		t.Errorf("backend.addr = %q, kutilgan %q", cfg.Backend.Addr, "django:9091")
	}
	if cfg.LLM.APIKey != "test-key" {
		t.Errorf("llm.api_key ENV'dan yechilmadi: %q", cfg.LLM.APIKey)
	}
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := loadForTest(t, writeConfig(t, minimalYaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	tests := []struct {
		got  any
		want any
	}{
		{cfg.Mode, "cli"},
		{cfg.LLM.Provider, "deepseek"},
		{cfg.LLM.Model, "deepseek-v4-pro"},
		{cfg.LLM.FallbackProvider, "claude"},
		{cfg.LLM.Timeout, 60 * time.Second},
		{cfg.Agent.MaxStep, 20},
		{cfg.Agent.Stream, true},
		{cfg.GRPC.Addr, ":9090"},
		{cfg.GRPC.ServiceTokenEnv, "AGENT_SERVICE_TOKEN"},
		{cfg.GRPC.AuthRequired, true},
		{cfg.GRPC.MaxSessions, 100},
		{cfg.GRPC.RateLimitPerMinute, 20},
		{cfg.GRPC.MaxMessageChars, 4000},
		{cfg.GRPC.Keepalive, 30 * time.Second},
		{cfg.HTTP.Addr, ":8081"},
		{cfg.Backend.Timeout, 5 * time.Second},
		{cfg.Backend.Retry, 3},
		{cfg.Catalog.SnapshotOnStart, true},
		{cfg.Catalog.StaleAfter, 10 * time.Minute},
		{cfg.Catalog.PageSize, 500},
		{cfg.Knowledge.InlineMax, 8},
		{cfg.STT.Provider, "whisper"},
		{cfg.STT.Language, "uz"},
		{cfg.TTS.Provider, "elevenlabs"},
		{cfg.TTS.Speed, 1.0},
		{cfg.DB.AutoMigrate, false},
		{cfg.Fallback.SystemPrompt, ""},
		{cfg.LogLevel, "info"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("got %v, want %v", tt.got, tt.want)
		}
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	t.Setenv("ALORA_LLM_PROVIDER", "claude")
	t.Setenv("ALORA_LLM_MODEL", "claude-sonnet-4")
	t.Setenv("ALORA_LLM_THINKING", "disabled")
	t.Setenv("ALORA_LLM_FALLBACK_PROVIDER", "openai")
	t.Setenv("ALORA_AGENT_MAX_HISTORY", "120")
	t.Setenv("ALORA_AGENT_STREAM", "false")
	t.Setenv("ALORA_GRPC_ADDR", ":9443")
	t.Setenv("ALORA_GRPC_AUTH_REQUIRED", "false")
	t.Setenv("ALORA_GRPC_MAX_SESSIONS", "250")
	t.Setenv("ALORA_BACKEND_ADDR", "localhost:9091")
	t.Setenv("ALORA_BACKEND_TIMEOUT", "3s")
	t.Setenv("ALORA_CATALOG_STALE_AFTER", "20m")
	t.Setenv("ALORA_CATALOG_PAGE_SIZE", "200")
	t.Setenv("ALORA_KNOWLEDGE_INLINE_MAX", "15")
	t.Setenv("ALORA_STT_PROVIDER", "google")
	t.Setenv("ALORA_TTS_VOICE", "ru_female_1")
	t.Setenv("ALORA_TTS_SPEED", "0.9")
	t.Setenv("ALORA_DB_AUTO_MIGRATE", "true")
	t.Setenv("ALORA_LOG_LEVEL", "debug")
	t.Setenv("ALORA_MODE", "grpc")

	t.Setenv("DEEPSEEK_API_KEY", "test-key") // provider claude uchun kalit
	t.Setenv("ANTHROPIC_API_KEY", "claude-key")
	cfg, err := Load(writeConfig(t, minimalYaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Mode != "grpc" {
		t.Errorf("mode = %q, want grpc", cfg.Mode)
	}
	if cfg.LLM.Provider != "claude" || cfg.LLM.Model != "claude-sonnet-4" {
		t.Errorf("llm env override ishlamadi: %+v", cfg.LLM)
	}
	if cfg.LLM.APIKey != "claude-key" {
		t.Errorf("llm.api_key claude ENV'dan yechilmadi: %q", cfg.LLM.APIKey)
	}
	if cfg.LLM.FallbackProvider != "openai" || cfg.LLM.Thinking != "disabled" {
		t.Errorf("llm fallback/thinking env override ishlamadi: %+v", cfg.LLM)
	}
	if cfg.Agent.MaxHistory != 120 || cfg.Agent.Stream {
		t.Errorf("agent env override ishlamadi: %+v", cfg.Agent)
	}
	if cfg.GRPC.Addr != ":9443" || cfg.GRPC.AuthRequired || cfg.GRPC.MaxSessions != 250 {
		t.Errorf("grpc env override ishlamadi: %+v", cfg.GRPC)
	}
	if cfg.Backend.Addr != "localhost:9091" || cfg.Backend.Timeout != 3*time.Second {
		t.Errorf("backend env override ishlamadi: %+v", cfg.Backend)
	}
	if cfg.Catalog.StaleAfter != 20*time.Minute || cfg.Catalog.PageSize != 200 {
		t.Errorf("catalog env override ishlamadi: %+v", cfg.Catalog)
	}
	if cfg.Knowledge.InlineMax != 15 {
		t.Errorf("knowledge env override ishlamadi: %d", cfg.Knowledge.InlineMax)
	}
	if cfg.STT.Provider != "google" {
		t.Errorf("stt env override ishlamadi: %+v", cfg.STT)
	}
	if cfg.TTS.Voice != "ru_female_1" || cfg.TTS.Speed != 0.9 {
		t.Errorf("tts env override ishlamadi: %+v", cfg.TTS)
	}
	if !cfg.DB.AutoMigrate {
		t.Errorf("db.auto_migrate env override ishlamadi: %+v", cfg.DB)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("log_level = %q, want debug", cfg.LogLevel)
	}
}

func TestLoad_DBURL_FromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/agent")
	cfg, err := loadForTest(t, writeConfig(t, minimalYaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.DB.URL != "postgres://localhost/agent" {
		t.Errorf("db.url ENV'dan olindi: %q", cfg.DB.URL)
	}
}

func TestLoad_MissingAPIKey(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")
	_, err := Load(writeConfig(t, minimalYaml))
	if err == nil || !strings.Contains(err.Error(), "DEEPSEEK_API_KEY") {
		t.Fatalf("kalit yo'q bo'lsa xato qaytishi kerak, oldik: %v", err)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		env   map[string]string // env override'lari (yaml'dan ustun turadi)
		isErr bool
		want  string // xato matnida bo'lishi kerak bo'lgan so'z
	}{
		{
			name:  "noto'g'ri mode",
			env:   map[string]string{"ALORA_MODE": "zoom"},
			isErr: true,
			want:  "mode",
		},
		{
			name:  "grpc rejimida bo'sh backend.addr",
			body:  "mode: grpc\nbackend:\n  addr: \"\"\n",
			isErr: true,
			want:  "backend.addr",
		},
		{
			name:  "grpc rejimida to'g'ri backend.addr",
			body:  "mode: grpc\nbackend:\n  addr: django:9091\n",
			isErr: false,
		},
		{
			name:  "stale_after nolga teng",
			body:  "mode: cli\ncatalog:\n  stale_after: 0\n",
			isErr: true,
			want:  "stale_after",
		},
		{
			name:  "stale_after manfiy",
			env:   map[string]string{"ALORA_CATALOG_STALE_AFTER": "-5s"},
			isErr: true,
			want:  "stale_after",
		},
		{
			name:  "page_size noldan kichik",
			body:  "mode: cli\ncatalog:\n  page_size: 0\n",
			isErr: true,
			want:  "page_size",
		},
		{
			name:  "page_size chegaradan oshdi",
			body:  "mode: cli\ncatalog:\n  page_size: 1001\n",
			isErr: true,
			want:  "page_size",
		},
		{
			name:  "page_size 1000 — chegarada, o'tadi",
			body:  "mode: cli\ncatalog:\n  page_size: 1000\n",
			isErr: false,
		},
		{
			name:  "llm.model bo'sh",
			body:  "mode: cli\nllm:\n  model: \"\"\n",
			isErr: true,
			want:  "llm.model",
		},
		{
			name:  "thinking noto'g'ri",
			env:   map[string]string{"ALORA_LLM_THINKING": "maybe"},
			isErr: true,
			want:  "thinking",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DEEPSEEK_API_KEY", "test-key")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			body := tt.body
			if body == "" {
				body = minimalYaml
			}
			_, err := Load(writeConfig(t, body))
			if tt.isErr {
				if err == nil {
					t.Fatalf("xato kutilgan edi, Load o'tdi")
				}
				if !strings.Contains(err.Error(), tt.want) {
					t.Errorf("xato matnida %q topilmadi: %v", tt.want, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("xato kutilmagan edi: %v", err)
			}
		})
	}
}

func TestValidate_Direct(t *testing.T) {
	// Validate'ni bevosita tekshiramiz — Load'ga bog'liqsiz (kalitsiz provider).
	base := &Config{}
	*base = Config{
		Mode:    "cli",
		LLM:     LLMConfig{Provider: "ollama", Model: "qwen"},
		Agent:   AgentConfig{MaxStep: 20, MaxHistory: 40, Stream: true},
		Catalog: CatalogConfig{StaleAfter: 10 * time.Minute, PageSize: 500},
		Backend: BackendConfig{Timeout: 5 * time.Second, Retry: 3},
	}

	if err := base.Validate(); err != nil {
		t.Fatalf("to'g'ri config xato berdi: %v", err)
	}

	base.Mode = "bogus"
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "mode") {
		t.Fatalf("noto'g'ri mode xato berishi kerak: %v", err)
	}
	base.Mode = "grpc"
	base.Backend.Addr = ""
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "backend.addr") {
		t.Fatalf("bo'sh backend.addr (grpc) xato berishi kerak: %v", err)
	}
	base.Mode = "cli"
	base.Backend.Addr = ""
	base.Catalog.StaleAfter = 0
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "stale_after") {
		t.Fatalf("stale_after=0 xato berishi kerak: %v", err)
	}
	base.Catalog.StaleAfter = 10 * time.Minute
	base.Catalog.PageSize = 5000
	if err := base.Validate(); err == nil || !strings.Contains(err.Error(), "page_size") {
		t.Fatalf("page_size=5000 xato berishi kerak: %v", err)
	}
}

func TestLoad_MockProvider(t *testing.T) {
	path := writeConfig(t, "mode: cli\nllm:\n  provider: mock\n")
	cfg, err := Load(path)
	if err != nil || cfg.LLM.Provider != "mock" {
		t.Fatalf("mock provider: cfg=%#v err=%v", cfg, err)
	}
}
