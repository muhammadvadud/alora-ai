package observability

import (
	"sort"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// rules.md §13.1 ro'yxati — alert'lar/dashboard shu nomlarga tayanadi, o'zgarishi
// monitoringni buzadi. Test nomlarni qo'riqlaydi (sarf qiymatlarini emas).
var expectedMetricNames = []string{
	"alora_chat_requests_total",
	"alora_ttft_seconds",
	"alora_response_seconds",
	"alora_tool_seconds",
	"alora_tool_errors_total",
	"alora_llm_tokens_total",
	"alora_llm_errors_total",
	"alora_llm_fallback_total",
	"alora_catalog_version",
	"alora_catalog_stale",
	"alora_catalog_age_seconds",
	"alora_catalog_watch_connected",
	"alora_outbox_pending",
	"alora_outbox_dead_total",
	"alora_active_sessions",
	"alora_voice_stt_seconds",
	"alora_voice_tts_chars",
}

func TestMetrics_AllNamesRegistered(t *testing.T) {
	m, reg := NewMetrics()

	// Vector'lar label qiymati olmaguncha Gather'da chiqmaydi — barcha 17 nomni
	// ko'rish uchun har vector'ga bitta misol qiymat beramiz.
	m.ChatRequests.WithLabelValues("ok").Inc()
	m.ToolSeconds.WithLabelValues("test_tool").Observe(0.01)
	m.ToolErrors.WithLabelValues("test_tool").Inc()
	m.LLMTokens.WithLabelValues("deepseek", "deepseek-v4-pro", "output").Add(1)
	m.LLMErrors.WithLabelValues("deepseek").Inc()

	// Gather — registrda RO'YXATDAN OTGAN metrikalarni qaytaradi; nomlar ro'yxati
	// rules §13.1 bilan bir xil bo'lishi shart.
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	got := make([]string, 0, len(families))
	for _, f := range families {
		got = append(got, f.GetName())
	}
	sort.Strings(got)

	want := append([]string(nil), expectedMetricNames...)
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("metrikalar soni = %d, %d bo'lishi kerak; qo'shimcha/yetishmayotgan: got=%v want=%v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("metrika nomi = %q, %q bo'lishi kerak (to'liq: %v)", got[i], want[i], got)
		}
	}
}

func TestMetrics_LabeledFamilies(t *testing.T) {
	m, reg := NewMetrics()

	// Label'li metrikalar label'larini Prometheus formatida ko'rsatishi kerak —
	// quruq ro'yxatdan o'tish o'zi emas, ishlatiladigan label'larni ham tekshiramiz.
	m.ToolSeconds.WithLabelValues("product_search").Observe(0.02)
	m.ToolErrors.WithLabelValues("product_search").Inc()
	m.LLMTokens.WithLabelValues("deepseek", "deepseek-v4-pro", "output").Add(150)
	m.ChatRequests.WithLabelValues("ok").Inc()
	m.CatalogVersion.Set(7)

	cases := []struct {
		name   string
		labels []string
	}{
		{"alora_tool_seconds", []string{"tool"}},
		{"alora_tool_errors_total", []string{"tool"}},
		{"alora_llm_tokens_total", []string{"provider", "model", "kind"}},
		{"alora_chat_requests_total", []string{"status"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if n := testutil.CollectAndCount(reg, tc.name); n != 1 {
				t.Fatalf("%s metrikalari soni = %d, 1 bo'lishi kerak", tc.name, n)
			}
			assertLabelNames(t, reg, tc.name, tc.labels)
		})
	}
}

// assertLabelNames — berilgan metric family'ning label NOMLARI kutilganlar bilan
// bir xil ekanini tekshiradi (label yo'qotilishi monitoringni buzardi).
func assertLabelNames(t *testing.T, reg *prometheus.Registry, familyName string, want []string) {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, f := range families {
		if f.GetName() != familyName {
			continue
		}
		for _, metric := range f.GetMetric() {
			got := make([]string, len(metric.GetLabel()))
			for i, l := range metric.GetLabel() {
				got[i] = l.GetName()
			}
			sort.Strings(got)
			sortedWant := append([]string(nil), want...)
			sort.Strings(sortedWant)
			if len(got) != len(sortedWant) {
				t.Fatalf("%s label'lari = %v, %v bo'lishi kerak", familyName, got, sortedWant)
			}
			for i := range sortedWant {
				if got[i] != sortedWant[i] {
					t.Fatalf("%s label'lari = %v, %v bo'lishi kerak", familyName, got, sortedWant)
				}
			}
			return
		}
	}
	t.Fatalf("%s family topilmadi", familyName)
}
