// Package http faqat sog'liq va metrika endpointlarini beradi (rules.md §0.7):
// /healthz (liveness), /readyz (readiness), /metrics (Prometheus). Biznes endpoint
// yo'q — u yerda agent yoki chat soni ko'rinishi arxitekturani buzadi.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/muhammadvadud/alora-ai/internal/observability"
)

// Serve HTTP serverini ishga tushiradi va ctx bekor bo'lganda 5 soniya ichida
// toza yopadi (graceful shutdown — boshlangan so'rovlar tugaydi).
// Timeout'lar goxavfsizlik uchun: uzoq/bo'sh ulanish serverni yopib qo'ymasligi kerak.
func Serve(ctx context.Context, addr string, reg *prometheus.Registry, ready observability.Readiness) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           newHandler(reg, ready),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}

	// Shutdown'ni alohida goroutine'da — Shutdown barcha faol ulanishlarni to'xtatadi
	// va shu yerda kutish jarayonni osib qo'ymaydi (contextcheck: ctx bu yerda ushlanadi).
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("HTTP server tinglayapti", "addr", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

// newHandler mux'ni yig'adi — test'da httptest bilan to'g'ridan-to'g'ri ishlatiladi,
// real port ochish shart emas.
func newHandler(reg *prometheus.Registry, ready observability.Readiness) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		readyz(w, ready)
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	return mux
}

// healthz liveness: jarayon tirikmi. Tashqi bog'liqliklarni tekshirmaydi — agar
// shu erda tekshirilsa, Django yotganda orchestrator podni o'ldirishi mumkin (rules §8.7).
func healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readyz readiness: servis xizmat ko'rsatishga tayyormi. Katalog snapshoti
// tugamaguncha 503 (rules §4.1.3) — K8s/HEALTHCHECK podni aylanishdan chiqaradi.
func readyz(w http.ResponseWriter, ready observability.Readiness) {
	ok, reason := ready.Ready()
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not_ready",
			"reason": reason,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
