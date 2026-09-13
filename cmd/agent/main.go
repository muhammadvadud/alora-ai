package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudwego/eino/components/tool"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"

	"github.com/muhammadvadud/alora-agent/internal/agent"
	"github.com/muhammadvadud/alora-agent/internal/backend"
	"github.com/muhammadvadud/alora-agent/internal/config"
	"github.com/muhammadvadud/alora-agent/internal/curtains"
	"github.com/muhammadvadud/alora-agent/internal/llm"
	"github.com/muhammadvadud/alora-agent/internal/observability"
	"github.com/muhammadvadud/alora-agent/internal/session"
	"github.com/muhammadvadud/alora-agent/internal/tools"
	"github.com/muhammadvadud/alora-agent/internal/transport/cli"
	grpctransport "github.com/muhammadvadud/alora-agent/internal/transport/grpc"
	httptransport "github.com/muhammadvadud/alora-agent/internal/transport/http"
)

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

	// Tool'lar shu yerda (composition root) ochiq register qilinadi — kim nima
	// qila oladi, bir joyda ko'rinadi. Handler/DTO/konstruktor: internal/tools.
	// Alora domen tool'lari (product_search, curtain_calculate, ...) tasks.md 1.E da qo'shiladi.
	toolList := []tool.BaseTool{tools.Calculator(), tools.Clock()}
	var backendConn *grpc.ClientConn
	var syncer *backend.CatalogSyncer
	var metrics *observability.Metrics
	var metricsReg *prometheus.Registry
	var catalog *curtains.Catalog
	if cfg.Mode == "grpc" {
		token := os.Getenv(cfg.GRPC.ServiceTokenEnv)
		backendConn, err = backend.Dial(ctx, cfg.Backend.Addr, token)
		if err != nil {
			return fmt.Errorf("backend ulanishi: %w", err)
		}
		defer backendConn.Close()
		catalog = curtains.NewCatalog(curtains.RealClock{}, cfg.Catalog.StaleAfter)
		metrics, metricsReg = observability.NewMetrics()
		syncer = backend.NewCatalogSyncer(backendConn, catalog, backend.SyncOptions{
			PageSize: int32(cfg.Catalog.PageSize), Metrics: metrics,
		})
		go func() {
			if syncErr := syncer.Run(ctx); syncErr != nil {
				slog.Error("katalog sinxronizatsiyasi to'xtadi", "error", syncErr)
			}
		}()
		pricing := backend.NewPricingClient(backendConn, cfg.Backend.Timeout, "")
		// Session state Store API 1.30/1.31 da ulanadi; hozircha nil ataylab beriladi.
		for _, candidate := range tools.Alora(tools.Deps{
			Catalog: catalog, Calc: pricing, State: nil, Clock: curtains.RealClock{},
		}) {
			if candidate != nil {
				toolList = append(toolList, candidate)
			}
		}
	}

	// Suhbat xotirasi: DATABASE_URL berilgan bo'lsa Postgres, aks holda in-memory.
	store, err := newStore(ctx, cfg.DB.URL)
	if err != nil {
		return err
	}
	chat, err := agent.NewChat(ctx, chatModel, toolList, store, agent.Options{
		SystemPrompt: systemPrompt(cfg),
		MaxStep:      cfg.Agent.MaxStep,
		MaxHistory:   cfg.Agent.MaxHistory,
		Stream:       cfg.Agent.Stream,
	})
	if err != nil {
		return err
	}

	slog.Info("agent tayyor", "provider", cfg.LLM.Provider, "model", cfg.LLM.Model, "mode", cfg.Mode, "stream", cfg.Agent.Stream, "max_history", cfg.Agent.MaxHistory, "thinking", cfg.LLM.Thinking, "effort", cfg.LLM.ReasoningEffort)

	// Transport tanlash. Yangi transport (masalan gRPC/WebSocket) qo'shish uchun
	// Chat.Stream(...) <-chan Event ni render qiling va shu switch'ga yangi case qo'shing.
	switch cfg.Mode {
	case "grpc":
		service := grpctransport.New(chat, grpctransport.Options{
			ServiceToken: os.Getenv(cfg.GRPC.ServiceTokenEnv), AuthRequired: cfg.GRPC.AuthRequired,
			MaxSessions: cfg.GRPC.MaxSessions, Ready: syncer, Metrics: metrics,
			Health: func() (string, string) {
				if catalog.View().Stale {
					return "degraded", "katalog eskirgan"
				}
				return "ok", ""
			},
		})
		return serveGRPC(ctx, cfg, service, metrics, metricsReg, syncer)
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

// httpReady — http rejimida katalog/backed sinxroni yo'q, shuning uchun doim tayyor.
type httpReady struct{}

func (httpReady) Ready() (bool, string) { return true, "" }

// newStore DB URL bo'lsa Postgres store, aks holda in-memory store qaytaradi.
func newStore(ctx context.Context, dbURL string) (session.Store, error) {
	if dbURL == "" {
		slog.Info("suhbat xotirasi: in-memory (DATABASE_URL berilmagan)")
		return session.NewMemory(), nil
	}
	store, err := session.NewPostgres(ctx, dbURL)
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

func serveGRPC(ctx context.Context, cfg *config.Config, srv *grpctransport.Server, metrics *observability.Metrics, reg *prometheus.Registry, ready observability.Readiness) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	errs := make(chan error, 2)
	go func() { errs <- httptransport.Serve(runCtx, cfg.HTTP.Addr, reg, ready) }()
	go func() {
		errs <- grpctransport.Serve(runCtx, cfg.GRPC.Addr, srv, grpctransport.ServeConfig{
			ServiceToken: os.Getenv(cfg.GRPC.ServiceTokenEnv), AuthRequired: cfg.GRPC.AuthRequired,
			MaxSessions: cfg.GRPC.MaxSessions, Metrics: metrics,
		})
	}()
	first := <-errs
	cancel()
	second := <-errs
	if first != nil {
		return first
	}
	return second
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
