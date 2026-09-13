package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/muhammadvadud/alora-ai/internal/observability"
)

// fakeReadiness — /readyz testi uchun boshqariladigan tayyorlik.
type fakeReadiness struct {
	ready  bool
	reason string
}

func (f fakeReadiness) Ready() (bool, string) { return f.ready, f.reason }

func newTestServer(reg *prometheus.Registry, ready observability.Readiness) *httptest.Server {
	return httptest.NewServer(newHandler(reg, ready))
}

// get — noctx'ga qarshi: so'rov har doim context bilan (rules §3.3).
func get(t *testing.T, srv *httptest.Server, path string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("so'rov yasash: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func TestHealthz_AlwaysOK(t *testing.T) {
	_, reg := observability.NewMetrics()
	srv := newTestServer(reg, fakeReadiness{ready: false}) // liveness readyga bog'liq emas
	defer srv.Close()

	resp := get(t, srv, "/healthz")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, 200 bo'lishi kerak", resp.StatusCode)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(readBody(t, resp)), &body); err != nil {
		t.Fatalf("javob JSON emas: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("healthz.status = %q, \"ok\" bo'lishi kerak", body.Status)
	}
}

func TestReadyz_NotReadyThenReady(t *testing.T) {
	_, reg := observability.NewMetrics()

	// Katalog hali yuklanmadi — 503 not_ready.
	srv := newTestServer(reg, fakeReadiness{ready: false, reason: "katalog hali yuklanmadi"})
	resp := get(t, srv, "/readyz")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("not_ready holatida status = %d, 503 bo'lishi kerak", resp.StatusCode)
	}
	var body struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal([]byte(readBody(t, resp)), &body); err != nil {
		t.Fatalf("javob JSON emas: %v", err)
	}
	resp.Body.Close()
	srv.Close()
	if body.Status != "not_ready" {
		t.Fatalf("readyz.status = %q, \"not_ready\" bo'lishi kerak", body.Status)
	}
	if body.Reason != "katalog hali yuklanmadi" {
		t.Fatalf("readyz.reason = %q, sabab kutilganidan farq qiladi", body.Reason)
	}

	// Katalog yuklandi — 200 ok.
	srv = newTestServer(reg, fakeReadiness{ready: true})
	resp = get(t, srv, "/readyz")
	defer resp.Body.Close()
	srv.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tayyor holatida status = %d, 200 bo'lishi kerak", resp.StatusCode)
	}
}

func TestMetrics_ExposesAlora(t *testing.T) {
	_, reg := observability.NewMetrics()
	srv := newTestServer(reg, fakeReadiness{ready: true})
	defer srv.Close()

	resp := get(t, srv, "/metrics")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics status = %d, 200 bo'lishi kerak", resp.StatusCode)
	}
	body := readBody(t, resp)
	// Prometheus matn formatida har metrika HELP/TYPE qatori bilan chiqadi —
	// katalog versiyasining bor-yo'qligi registry to'g'ri ulanganini isbotlaydi.
	if !strings.Contains(body, "alora_catalog_version") {
		t.Fatalf("/metrics da alora_catalog_version yo'q; birinchi 512 bayt: %q", truncate(body, 512))
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("javobni o'qish: %v", err)
	}
	return string(b)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
