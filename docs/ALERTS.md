# Alora AI Agent — Alert Qoidalari (ALERTS.md)

Bu hujjat Grafana/Prometheus orqali kuzatiladigan alertlar ro'yxatini va ularni hal qilish bo'yicha birlamchi reaksiyalarni o'z ichiga oladi.

## Alert Shartlari

| Metrika | Shart | Jiddiylik | Birinchi Harakat | Fayl Havolasi |
| :--- | :--- | :--- | :--- | :--- |
| `alora_catalog_stale` | stale > 10m | Ogohlantirish (Warning) | RUNBOOK: Katalog Stale bo'limiga qarang (Watch oqimi va Django holatini tekshirish). | `internal/curtains/catalog.go` |
| `alora_outbox_dead_total` | outbox dead > 0 | Tanqidiy (Critical) | RUNBOOK: Django serveri ishlayotganligini tekshiring, asinxron yozuvlar failed/dead holatga o'tmoqda. | `internal/backend/outbox.go` |
| `alora_ttft_seconds` | TTFT p95 > 2s | Ogohlantirish (Warning) | LLM provider sekinlashganini yoki tarmoq kutilishlarini kuzating. | `internal/observability/metrics.go` |
| `alora_llm_errors_total` | error rate > 5% | Yuqori (High) | RUNBOOK: LLM provayder xatosi/429 bo'limiga qarang. API kalit limitlarini tekshiring. | `internal/llm/llm.go` |
| `alora_catalog_watch_connected` | uzilish > 1m | Yuqori (High) | RUNBOOK: Katalog Stale qismida yozilganidek `/readyz` va Django gRPC dostupini kuzating. | `internal/backend/catalog_sync.go` |
| `alora_llm_fallback_total` | o'sishi | Ogohlantirish (Warning) | Asosiy provayder muammosi, fallback ishga tushyapti. Limitlarni tekshiring. | `internal/llm/llm.go` |
| STT xatolari (log) | stt error rate | Yuqori (High) | Whisper/Google STT api ulanishini tekshiring, xatolar tufayli matn yozilmayapti. | `internal/transport/grpc/voice.go` |

*Batafsil ma'lumot va yechimlar uchun [RUNBOOK.md](./RUNBOOK.md) hujjatiga murojaat qiling.*
