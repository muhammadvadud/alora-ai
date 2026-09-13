package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/joho/godotenv"
	"github.com/muhammadvadud/alora-ai/gen/alorav1"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/muhammadvadud/alora-ai/internal/agent"
	"github.com/muhammadvadud/alora-ai/internal/backend"
	"github.com/muhammadvadud/alora-ai/internal/config"
	"github.com/muhammadvadud/alora-ai/internal/curtains"
	"github.com/muhammadvadud/alora-ai/internal/llm"
	"github.com/muhammadvadud/alora-ai/internal/observability"
	"github.com/muhammadvadud/alora-ai/internal/prompt"
	"github.com/muhammadvadud/alora-ai/internal/session"
	"github.com/muhammadvadud/alora-ai/internal/tools"
	"github.com/muhammadvadud/alora-ai/internal/transport/cli"
	grpctransport "github.com/muhammadvadud/alora-ai/internal/transport/grpc"
	httptransport "github.com/muhammadvadud/alora-ai/internal/transport/http"
	"github.com/muhammadvadud/alora-ai/internal/voice"
)

var _ tools.StateStore = (session.Store)(nil)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "xato:", err)
		os.Exit(1)
	}
}

func run() error {
	configFile := flag.String("config", configPath(), "config fayl yo'li")
	modeFlag := flag.String("mode", "", "ishga tushirish rejimi: cli | http | grpc (config'ni ustidan yozadi)")
	flag.Parse()

	_ = godotenv.Load() // .env bo'lsa yuklaymiz (majburiy emas)

	cfg, err := config.Load(*configFile)
	if err != nil {
		return err
	}
	if *modeFlag != "" {
		cfg.Mode = *modeFlag
	}
	observability.SetupLogger(cfg.LogLevel)

	// SIGINT/SIGTERM da toza to'xtash.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Bog'liqliklarni ulaymiz (DI).
	chatModel, err := llm.New(ctx, cfg.LLM)
	if err != nil {
		return err
	}
	var fallbackModel model.ToolCallingChatModel
	if provider := cfg.LLM.FallbackProvider; provider != "" {
		fallbackCfg := cfg.LLM
		fallbackCfg.Provider = provider
		fallbackCfg.APIKey = fallbackAPIKey(provider)
		fallbackCfg.BaseURL = ""
		fallbackCfg.Thinking = ""
		fallbackCfg.ReasoningEffort = ""
		if fallbackCfg.APIKey == "" && provider != "ollama" {
			slog.Warn("llm fallback o'chirildi: API kalit topilmadi", "provider", provider)
		} else {
			fallbackModel, err = llm.New(ctx, fallbackCfg)
			if err != nil {
				slog.Warn("llm fallback o'chirildi: model yaratilmadi", "provider", provider, "error", err)
				fallbackModel = nil
			}
		}
	}

	// Tool'lar shu yerda (composition root) ochiq register qilinadi — kim nima
	// qila oladi, bir joyda ko'rinadi. Handler/DTO/konstruktor: internal/tools.
	// Alora domen tool'lari (product_search, curtain_calculate, ...) tasks.md 1.E da qo'shiladi.
	toolList := []tool.BaseTool{tools.Calculator(), tools.Clock()}
	var backendConn *grpc.ClientConn
	var syncer *backend.CatalogSyncer
	var metrics *observability.Metrics
	var metricsReg *prometheus.Registry
	catalog := curtains.NewCatalog(curtains.RealClock{}, cfg.Catalog.StaleAfter)
	var calc curtains.Calculator
	if cfg.Mode == "grpc" {
		if cfg.LLM.Provider == "mock" {
			slog.Warn("mock provider grpc rejimida faqat dev/e2e uchun; prod'da taqiqlangan")
		}
		if cfg.GRPC.AuthRequired && !cfg.GRPC.TLS.Enabled {
			slog.Warn("token TLS'siz uzatilmoqda — faqat ichki tarmoq")
		}
		if !cfg.Backend.TLS.Enabled {
			slog.Warn("backend token TLS'siz uzatilmoqda — faqat ichki tarmoq")
		}
		token := os.Getenv(cfg.GRPC.ServiceTokenEnv)
		backendCreds, credErr := backend.ClientCredentials(cfg.Backend.TLS)
		if credErr != nil {
			return credErr
		}
		backendConn, err = backend.DialTLS(ctx, cfg.Backend.Addr, token, backendCreds)
		if err != nil {
			return fmt.Errorf("backend ulanishi: %w", err)
		}
		defer backendConn.Close()
		metrics, metricsReg = observability.NewMetrics()
		syncer = backend.NewCatalogSyncer(backendConn, catalog, backend.SyncOptions{
			PageSize: int32(cfg.Catalog.PageSize), Metrics: metrics,
		})
		go func() {
			if syncErr := syncer.Run(ctx); syncErr != nil {
				slog.Error("katalog sinxronizatsiyasi to'xtadi", "error", syncErr)
			}
		}()
		calc = backend.NewPricingClient(backendConn, cfg.Backend.Timeout, "")
	}

	// Suhbat xotirasi: DATABASE_URL berilgan bo'lsa Postgres, aks holda in-memory.
	store, err := newStore(ctx, cfg.DB.URL, cfg.DB.AutoMigrate)
	if err != nil {
		return err
	}
	if archiver, ok := store.(session.Archiver); ok {
		store = session.WithArchiveOnReset(store, archiver)
		startArchiveCleanup(ctx, archiver)
	}
	var reporter grpctransport.Reporter
	var reportOutbox *backend.Outbox
	if cfg.Mode == "grpc" {
		var outboxStore session.OutboxStore
		if cfg.DB.URL == "" {
			slog.Warn("backend outbox: in-memory navbat (DATABASE_URL berilmagan)")
			outboxStore = session.NewMemoryOutbox()
		} else {
			outboxStore, err = session.NewPostgresOutbox(ctx, cfg.DB.URL)
			if err != nil {
				return fmt.Errorf("postgres outbox: %w", err)
			}
		}
		reportOutbox = backend.NewOutbox(outboxStore, alorav1.NewBackendClient(backendConn), backend.OutboxOptions{Metrics: metrics, Clock: curtains.RealClock{}, Logger: slog.Default()})
		reporter = reportOutbox
		go func() {
			if runErr := reportOutbox.Run(ctx); runErr != nil && ctx.Err() == nil {
				slog.Error("outbox worker to'xtadi", "error", runErr)
			}
		}()
	}
	deps := tools.Deps{Catalog: catalog, Calc: calc, State: store, Clock: curtains.RealClock{}}
	if backendConn != nil {
		// Buyurtma qoralamasi faqat gRPC rejimida (CLI'da foydalanuvchi mijoz emas).
		// nil client interfeysdan non-nil bo'lib qolmasligi uchun shart qo'yiladi —
		// aks holda order_draft "xizmat ulanmagan" deb ayta olmay, panic beradi.
		deps.Orders = backend.NewOrderClient(backendConn, cfg.Backend.Timeout)
	}
	for _, candidate := range tools.Alora(deps) {
		if candidate != nil {
			toolList = append(toolList, candidate)
		}
	}
	// AloraLookups (fason/knowledge) har doim qo'shiladi: knowledge_lookup ideal holatda
	// faqat len(view.Knowledge()) > cfg.Knowledge.InlineMax bo'lganda register bo'lishi
	// kerak edi (rules §4.6.2), lekin startup'da katalog hali sinxronlanmagan bo'lishi
	// mumkin — dinamik shartni keyin hal qilamiz.
	for _, candidate := range tools.AloraLookups(deps) {
		if candidate != nil {
			toolList = append(toolList, candidate)
		}
	}
	chat, err := agent.NewChat(ctx, chatModel, toolList, store, agent.Options{
		SystemPrompt:     systemPrompt(cfg),
		SystemPromptFunc: promptFunc(store, catalog, fallbackPrompt(cfg), cfg.Knowledge.InlineMax),
		MaxStep:          cfg.Agent.MaxStep,
		MaxHistory:       cfg.Agent.MaxHistory,
		Stream:           cfg.Agent.Stream,
		StateHook:        stateHook{store: store},
		Provider:         cfg.LLM.Provider,
		Model:            cfg.LLM.Model,
		Fallback:         fallbackModel,
		FallbackProvider: cfg.LLM.FallbackProvider,
	})
	if err != nil {
		return err
	}

	slog.Info("agent tayyor", "provider", cfg.LLM.Provider, "model", cfg.LLM.Model, "mode", cfg.Mode, "stream", cfg.Agent.Stream, "max_history", cfg.Agent.MaxHistory, "thinking", cfg.LLM.Thinking, "effort", cfg.LLM.ReasoningEffort)
	stt, tts := buildVoiceProviders(ctx, cfg)

	// Transport tanlash. Yangi transport (masalan gRPC/WebSocket) qo'shish uchun
	// Chat.Stream(...) <-chan Event ni render qiling va shu switch'ga yangi case qo'shing.
	switch cfg.Mode {
	case "grpc":
		service := grpctransport.New(chat, grpctransport.Options{
			ServiceToken: os.Getenv(cfg.GRPC.ServiceTokenEnv), AuthRequired: cfg.GRPC.AuthRequired,
			MaxSessions: cfg.GRPC.MaxSessions, MaxMessageChars: cfg.GRPC.MaxMessageChars, RateLimitPerMinute: cfg.GRPC.RateLimitPerMinute, Store: store, Ready: syncer, Metrics: metrics, Reporter: reporter,
			STT: stt, TTS: tts, TTSVoice: cfg.TTS.Voice, STTLang: cfg.STT.Language,
			Health: func() (string, string) {
				if reportOutbox != nil {
					pending, dead, pendingErr := reportOutbox.Pending(ctx)
					if pendingErr != nil {
						return "degraded", "outbox holatini o'qib bo'lmadi"
					}
					if pending > 1000 || dead > 0 {
						return "degraded", "outbox navbati to'lib qolgan"
					}
				}
				if catalog.View().Stale {
					return "degraded", "katalog eskirgan"
				}
				return "ok", ""
			},
		})
		serverCreds, credErr := grpctransport.ServerCredentials(cfg.GRPC.TLS)
		if credErr != nil {
			return credErr
		}
		if !cfg.GRPC.TLS.Enabled {
			serverCreds = nil
		}
		return serveGRPC(ctx, cfg, service, metrics, metricsReg, syncer, serverCreds)
	case "http":
		// HTTP transport faqat /healthz, /readyz, /metrics (rules.md §0.7).
		// Katalog/outbox sinxroni (HealthState) gRPC rejimida ulanadi; http rejimida
		// tayyorlik jarayon o'zi — ularsiz ham xizmat bera oladi.
		_, reg := observability.NewMetrics()
		return httptransport.Serve(ctx, cfg.HTTP.Addr, reg, httpReady{})
	case "cli":
		return cli.Run(ctx, chat)
	default:
		return fmt.Errorf("noma'lum mode %q (cli yoki http)", cfg.Mode)
	}
}

func buildVoiceProviders(ctx context.Context, cfg *config.Config) (voice.STT, voice.TTS) {
	var stt voice.STT
	var tts voice.TTS
	if cfg.STT.Provider != "" {
		provider, err := voice.NewSTT(ctx, voice.STTConfig{Provider: cfg.STT.Provider, Language: cfg.STT.Language, APIKey: voiceAPIKey(cfg.STT.Provider)})
		if err == nil {
			stt = provider
		} else {
			slog.Warn("stt ulanmagan", "provider", cfg.STT.Provider, "error", err)
		}
	}
	if cfg.TTS.Provider != "" {
		provider, err := voice.NewTTS(ctx, voice.TTSConfig{Provider: cfg.TTS.Provider, Voice: cfg.TTS.Voice, Speed: cfg.TTS.Speed, APIKey: voiceAPIKey(cfg.TTS.Provider)})
		if err == nil {
			tts = provider
		} else {
			slog.Warn("tts ulanmagan", "provider", cfg.TTS.Provider, "error", err)
		}
	}
	return stt, tts
}

func voiceAPIKey(provider string) string {
	keyEnv := map[string]string{"whisper": "OPENAI_API_KEY", "elevenlabs": "ELEVENLABS_API_KEY", "google": "GOOGLE_API_KEY"}[provider]
	return os.Getenv(keyEnv)
}

// httpReady — http rejimida katalog/backed sinxroni yo'q, shuning uchun doim tayyor.
type httpReady struct{}

func (httpReady) Ready() (bool, string) { return true, "" }

// newStore DB URL bo'lsa Postgres store, aks holda in-memory store qaytaradi.
func newStore(ctx context.Context, dbURL string, autoMigrate bool) (session.Store, error) {
	if dbURL == "" {
		slog.Info("suhbat xotirasi: in-memory (DATABASE_URL berilmagan)")
		return session.NewMemory(), nil
	}
	store, err := session.NewPostgres(ctx, dbURL, session.PostgresOptions{AutoMigrate: autoMigrate})
	if err != nil {
		return nil, fmt.Errorf("postgres store: %w", err)
	}
	slog.Info("suhbat xotirasi: postgres")
	return store, nil
}

func systemPrompt(cfg *config.Config) string {
	if cfg.Agent.SystemPrompt != "" {
		return cfg.Agent.SystemPrompt
	}
	if cfg.Fallback.SystemPrompt != "" {
		return cfg.Fallback.SystemPrompt
	}
	return defaultSystemPrompt
}

func fallbackPrompt(cfg *config.Config) string {
	if cfg.Agent.SystemPrompt != "" {
		return cfg.Agent.SystemPrompt
	}
	if cfg.Fallback.SystemPrompt != "" {
		return cfg.Fallback.SystemPrompt
	}
	return prompt.FallbackSystemPrompt
}

func fallbackAPIKey(provider string) string {
	var env string
	switch provider {
	case "deepseek":
		env = "DEEPSEEK_API_KEY"
	case "openai":
		env = "OPENAI_API_KEY"
	case "claude":
		env = "ANTHROPIC_API_KEY"
	default:
		return ""
	}
	return os.Getenv(env)
}

func serveGRPC(ctx context.Context, cfg *config.Config, srv *grpctransport.Server, metrics *observability.Metrics, reg *prometheus.Registry, ready observability.Readiness, creds credentials.TransportCredentials) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errs := make(chan error, 2)
	go func() { errs <- httptransport.Serve(runCtx, cfg.HTTP.Addr, reg, ready) }()
	go func() {
		errs <- grpctransport.ServeTLS(runCtx, cfg.GRPC.Addr, srv, grpctransport.ServeConfig{
			ServiceToken: os.Getenv(cfg.GRPC.ServiceTokenEnv), AuthRequired: cfg.GRPC.AuthRequired,
			MaxSessions: cfg.GRPC.MaxSessions, MaxMessageChars: cfg.GRPC.MaxMessageChars, RateLimitPerMinute: cfg.GRPC.RateLimitPerMinute, Metrics: metrics,
		}, creds)
	}()
	first := <-errs
	cancel()
	second := <-errs
	if first != nil {
		return first
	}
	return second
}

func startArchiveCleanup(ctx context.Context, archiver session.Archiver) {
	interval := 24 * time.Hour
	if raw := os.Getenv("ALORA_SESSION_CLEANUP_INTERVAL"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil {
			interval = parsed
		}
	}
	if interval <= 0 {
		return
	}
	ttl := 90 * 24 * time.Hour
	if raw := os.Getenv("ALORA_SESSION_TTL_DAYS"); raw != "" {
		if days, err := strconv.Atoi(raw); err == nil && days >= 0 {
			ttl = time.Duration(days) * 24 * time.Hour
		}
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				archived, deleted, err := archiver.Cleanup(ctx, ttl)
				if err != nil {
					slog.Error("sessiya arxivi tozalanmadi", "error", err)
					continue
				}
				slog.Info("sessiya arxivi tozalandi", "archived", archived, "deleted", deleted)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func configPath() string {
	if p := os.Getenv("CONFIG_PATH"); p != "" {
		return p
	}
	return "configs/config.yaml"
}

const defaultSystemPrompt = `Sen Rocket Park uchun zakaz qabul qiluvchi AI yordamchisan.
Faqat mavjud paketlar va narxlar bilan ishlagin, yangi menyu o'ylab topma.
Har javob bitta qisqa xabar bo'lsin, ortiqcha qayta takrorlama.
Kerak bo'lsa emoji ishlat, lekin matnni ixcham yoz.
Mijozdan kerakli ma'lumotlarni bosqichma-bosqich yig':
paket nomi, bolalar soni, sana, vaqt, buyurtmachi ismi, telefon va izoh.
Paket nomi noaniq bo'lsa katalog tool'idan foydalan.
Bolalar soni paket sig'imidan oshsa, mosroq variantni taklif qil.
Zakaz to'liq bo'lsa tool orqali tasdiqlashni yakunla va admin chatga yuborilishi uchun tayyorla.
Faqat bitta savol yoki bitta keyingi qadamni ber.`
