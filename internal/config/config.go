// Package config Viper orqali YAML + muhit o'zgaruvchilaridan sozlamalarni yuklaydi.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	LLM      LLMConfig      `mapstructure:"llm"`
	Agent    AgentConfig    `mapstructure:"agent"`
	Server   ServerConfig   `mapstructure:"server"`
	Telegram TelegramConfig `mapstructure:"telegram"`
	DB       DBConfig       `mapstructure:"db"`
	Mode     string         `mapstructure:"mode"` // cli | http | telegram
	LogLevel string         `mapstructure:"log_level"`
}

// DBConfig — suhbat tarixi uchun PostgreSQL. URL bo'sh bo'lsa in-memory ishlatiladi.
// URL faqat ENV'dan: DATABASE_URL
type DBConfig struct {
	URL string `mapstructure:"url"`
}

// TelegramConfig — telegram bot transporti. Token faqat ENV'dan: TELEGRAM_BOT_TOKEN
type TelegramConfig struct {
	Token string `mapstructure:"token"`
	// EditInterval — stream paytida xabarni yangilash oralig'i (Telegram rate-limit).
	// ENV: TELEGRAM_EDIT_INTERVAL
	EditInterval time.Duration `mapstructure:"edit_interval"`
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
	// Bo'sh — provider standarti. ENV: LLM_THINKING
	//
	// Hozircha faqat deepseek provideri qo'llaydi.
	Thinking string `mapstructure:"thinking"`
	// ReasoningEffort — o'ylash chuqurligi: low | medium | high | xhigh | max.
	// Bo'sh — provider standarti. ENV: LLM_REASONING_EFFORT
	// Faqat thinking o'chirilmagan bo'lsa ma'noga ega.
	ReasoningEffort string `mapstructure:"reasoning_effort"`
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
	// ENV: AGENT_MAX_HISTORY
	MaxHistory   int    `mapstructure:"max_history"`
	SystemPrompt string `mapstructure:"system_prompt"`
	// Stream — javobni jonli (bo'lak-bo'lak) yuborish. false bo'lsa javob yaxlit keladi.
	// Standart: true. ENV: AGENT_STREAM=false
	Stream bool `mapstructure:"stream"`
}

type ServerConfig struct {
	Addr string `mapstructure:"addr"`
}

// providerKeyEnv har provider uchun API-kalit muhit o'zgaruvchisi.
// Bo'sh qiymat — kalit talab qilinmaydi (masalan Ollama lokal).
var providerKeyEnv = map[string]string{
	"deepseek": "DEEPSEEK_API_KEY",
	"openai":   "OPENAI_API_KEY",
	"claude":   "ANTHROPIC_API_KEY",
	"ollama":   "",
}

// Load config faylini o'qiydi, muhit o'zgaruvchilari bilan ustidan yozadi va
// tanlangan provider'ga qarab API kalitni tegishli ENV'dan yechadi.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	v.SetDefault("llm.provider", "deepseek")
	v.SetDefault("llm.model", "deepseek-v4-pro")
	v.SetDefault("llm.base_url", "https://api.deepseek.com/v1")
	v.SetDefault("llm.max_tokens", 2048)
	v.SetDefault("llm.temperature", 0.7)
	v.SetDefault("llm.timeout", "60s")
	v.SetDefault("agent.max_step", 20)
	v.SetDefault("agent.max_history", 40)
	v.SetDefault("agent.stream", true)
	v.SetDefault("telegram.edit_interval", "800ms")
	v.SetDefault("server.addr", ":8080")
	v.SetDefault("mode", "cli")
	v.SetDefault("log_level", "info")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("config faylini o'qish: %w", err)
	}

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("config'ni tahlil qilish: %w", err)
	}

	if err := cfg.resolveAPIKey(); err != nil {
		return nil, err
	}
	if cfg.LLM.Model == "" {
		return nil, fmt.Errorf("llm.model bo'sh bo'lishi mumkin emas")
	}
	switch cfg.LLM.Thinking {
	case "", ThinkingAdaptive, ThinkingEnabled, ThinkingDisabled:
	default:
		return nil, fmt.Errorf("llm.thinking %q noto'g'ri (%s, %s yoki %s)",
			cfg.LLM.Thinking, ThinkingAdaptive, ThinkingEnabled, ThinkingDisabled)
	}
	switch cfg.LLM.ReasoningEffort {
	case "", EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax:
	default:
		return nil, fmt.Errorf("llm.reasoning_effort %q noto'g'ri (%s, %s, %s, %s yoki %s)",
			cfg.LLM.ReasoningEffort, EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax)
	}
	// Telegram tokeni ENV'dan (faqat telegram rejimida zarur — main.go tekshiradi).
	if cfg.Telegram.Token == "" {
		cfg.Telegram.Token = os.Getenv("TELEGRAM_BOT_TOKEN")
	}
	// DB URL ENV'dan (ixtiyoriy — bo'sh bo'lsa in-memory).
	if cfg.DB.URL == "" {
		cfg.DB.URL = os.Getenv("DATABASE_URL")
	}
	return &cfg, nil
}

// resolveAPIKey provider'ga mos ENV'dan kalitni oladi va zarurligini tekshiradi.
func (c *Config) resolveAPIKey() error {
	envName, known := providerKeyEnv[c.LLM.Provider]
	if !known {
		return fmt.Errorf("noma'lum provider %q (deepseek, openai, claude, ollama)", c.LLM.Provider)
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
