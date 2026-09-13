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

	"github.com/muhammadvadud/alora-agent/internal/agent"
	"github.com/muhammadvadud/alora-agent/internal/config"
	"github.com/muhammadvadud/alora-agent/internal/llm"
	"github.com/muhammadvadud/alora-agent/internal/session"
	"github.com/muhammadvadud/alora-agent/internal/tools"
	"github.com/muhammadvadud/alora-agent/internal/transport/cli"
	httptransport "github.com/muhammadvadud/alora-agent/internal/transport/http"
	"github.com/muhammadvadud/alora-agent/internal/transport/telegram"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "xato:", err)
		os.Exit(1)
	}
}

func run() error {
	configFile := flag.String("config", configPath(), "config fayl yo'li")
	modeFlag := flag.String("mode", "", "ishga tushirish rejimi: cli | http (config'ni ustidan yozadi)")
	flag.Parse()

	_ = godotenv.Load() // .env bo'lsa yuklaymiz (majburiy emas)

	cfg, err := config.Load(*configFile)
	if err != nil {
		return err
	}
	if *modeFlag != "" {
		cfg.Mode = *modeFlag
	}
	setupLogger(cfg.LogLevel)

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
	case "http":
		return httptransport.Serve(ctx, chat, cfg.Server.Addr)
	case "cli":
		return cli.Run(ctx, chat)
	case "telegram":
		if cfg.Telegram.Token == "" {
			return fmt.Errorf("TELEGRAM_BOT_TOKEN o'rnatilmagan (telegram rejimi uchun zarur)")
		}
		return telegram.Serve(ctx, chat, telegram.Options{
			Token:        cfg.Telegram.Token,
			AdminChatID:  1769851684,
			EditInterval: cfg.Telegram.EditInterval,
		})
	default:
		return fmt.Errorf("noma'lum mode %q (cli, http yoki telegram)", cfg.Mode)
	}
}

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
	return defaultSystemPrompt
}

func setupLogger(level string) {
	var l slog.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		l = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l})))
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
