package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestSetupLogger_InvalidLevelFallsBackToInfo(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	logger := SetupLogger("not-a-level")
	if logger == nil {
		t.Fatal("SetupLogger nil logger qaytardi")
	}
	// Tanib bo'lmasa info — debug qatori ham chiqishi (ko'proq) emas, kamroq emas:
	// noaniq holatda standart daraja eng xavfsizi (rules §3.9).
	if !logger.Enabled(context.Background(), slog.LevelInfo) {
		t.Fatal("fallback info: info daraja o'chirilgan")
	}
	if logger.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("fallback info: debug daraja ochiq qolgan")
	}
	if !logger.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("fallback info: error daraja o'chirilgan")
	}
}

func TestLogger_WithRequestAddsFields(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	// request_id, session_id, user_id — rules §3.9 majburiy maydonlari (tool,
	// latency_ms biznes qatorlarida, transport bu yerda emas).
	ctx := WithRequest(context.Background(), "req-1", "sess-2", "user-3")
	Logger(ctx).Info("sinov")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("log JSON emas: %v; matn: %q", err, buf.String())
	}
	if got["request_id"] != "req-1" {
		t.Fatalf("request_id = %v, \"req-1\" bo'lishi kerak", got["request_id"])
	}
	if got["session_id"] != "sess-2" {
		t.Fatalf("session_id = %v, \"sess-2\" bo'lishi kerak", got["session_id"])
	}
	if got["user_id"] != "user-3" {
		t.Fatalf("user_id = %v, \"user-3\" bo'lishi kerak", got["user_id"])
	}
}

func TestLogger_WithoutContextUsesDefault(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	// Kontekstda maydon bo'lmasa shovqin qo'shilmagan logger — default.
	Logger(context.Background()).Info("ishga tushish")
	if strings.Contains(buf.String(), "request_id") {
		t.Fatalf("kontekstsiz logga maydon tushib qoldi: %q", buf.String())
	}
}
