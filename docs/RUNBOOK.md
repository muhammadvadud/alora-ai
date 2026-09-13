# Alora AI Agent — Operatsion Holatlar (Runbook)

Ushbu qollanma tizim ishlash jarayonidagi xatoliklarni topish va bartaraf etish bo'yicha ko'rsatmalarni taqdim etadi.

## 1. Operatsion Holatlar va Harakatlar

| Holat / Xato | Mumkin bo'lgan Sabablar | Tekshirish va Harakatlar | Fayl Havolasi |
| :--- | :--- | :--- | :--- |
| **Katalog Stale (> 10 daq)**<br>*(AI narxlar eski deydi)* | Watch oqimi uzilgan, Django gRPC yotgan, yoxud Django Redis muammosi. | 1. `/readyz` endpoint holatini tekshirish.<br>2. `/metrics` da `alora_catalog_age_seconds` va `alora_catalog_watch_connected` ni kuzatish.<br>3. Loglarda `catalog_resync` izlash. | `internal/backend/catalog_sync.go` |
| **Django Pricing yotgan**<br>*(Hisob vaqtincha ishlamayapti)* | Django server qulagan yoki bog'liqlik yo'q. Bu holatda AI "hisob xizmati vaqtincha ishlamayapti" deb narx aytmaydi. | 1. Django gRPC xizmati ishlayotganligini tekshirish (`ALORA_BACKEND_ADDR`).<br>2. Agent gRPC klient timeout'ini kuzatish. | `internal/backend/pricing.go` |
| `ratelimit RESOURCE_EXHAUSTED` | Bitta sessiya orqali juda ko'p so'rovlar (20 req/min) kelmoqda. | Agar hujum emas, balki real muhtojlik bo'lsa `ratelimit` cheklovini config orqali vaqtincha oshirish. | `internal/transport/grpc/server.go` |
| **Outbox Dead > 0**<br>*(Yuborilmagan hodisalar)* | Django uzoq vaqt yotgan yoki network uzilgan, 10-retry yiqilgan. | `alora_outbox_dead_total` o'sadi. SQL orqali qayta urinish: `UPDATE agent.outbox SET dead=false, attempts=0, next_at=now() WHERE dead=true;` | `internal/backend/outbox.go` |
| **order_draft confirm xatosi** | Django tomonda tasdiqlash jarayonida qandaydir validatsiya xatosi chiqyapti. | Agent Django'dan qaytgan xatoni `error` maydonida AI'ga (va sotuvchiga) to'g'ridan-to'g'ri qaytaradi, loglarni kuzating. | `internal/tools/order_draft.go` |
| **VoiceChat InvalidArgument** | Gating/metadata uzatishda xatolik. | Django gateway orqali kelayotganda `x-session-id`, `x-user-id`, `x-role` va `x-customer-mode` metadata kalitlari bormi tekshiring. | `internal/transport/grpc/voice.go` |
| **LLM Provayder xatosi / 429**<br>*(Rate limit, Timeout)* | Model API'dan javob kelmayapti yoki limit tugagan. | **Fallback provayder (4.1):** Ikkita xato bo'lsa avtomatik `fallback_provider` (config'dan) yoqiladi, va navbat qayta yuboriladi. Ikkalasi yiqilsa `error` event chiqadi. `alora_llm_fallback_total` va `llm_fallback` log'larini kuzating. | `internal/llm/llm.go` |

## 2. Ishga Tushirish

### Kalitsiz ishga tushirish (mock provider)

API kalitisiz deterministik dev sinovi uchun `llm.provider: mock` tanlanadi. Mock
provider `mehmonxona`/`hotel` so'rovida `set_context` va `product_search` chaqiradi,
o'lchamli `hisobla` so'rovida esa `curtain_calculate` chaqiradi; tool natijasida
bo'lmagan narxni o'zi ishlab chiqarmaydi. Bu rejim faqat integratsiya va SSE oqimini
tekshirish uchun.

> Eslatma: `config.Load` provider registry'siga `mock` qo'shilgan build talab qilinadi.

Agentni ishlab chiqish yoki tekshirish maqsadida ishga tushirish uchun quyidagi usullardan foydalaning:
* **Make yordamida:**
  ```bash
  make run-grpc
  ```
* **Docker Compose orqali:**
  ```bash
  docker compose up agent -d
  ```
* **Portlar:**
  * `9090` — Ichki gRPC server porti (biznes so'rovlar uchun).
  * `8081` — HTTP Health va Metrics porti (`/healthz`, `/readyz`, `/metrics`).
*(Asosiy kod: `cmd/agent/main.go`)*

## 3. Testlarni Ishga Tushirish
Qo'shimcha tekshiruvlar uchun quyidagi buyruqlar foydali:
* **Live Testlar (Real Django bilan):**
  ```bash
  ALORA_LIVE_BACKEND=127.0.0.1:50051 AGENT_SERVICE_TOKEN=secret go test -tags live ./tests/live/
  ```
* **Load Testlar (Yuklama):**
  ```bash
  go test -tags load ./tests/load/
  ```

## 3. Loglarni O'qish
Loglar JSON formatda `slog` orqali yoziladi. Har bir zaprosni diagnostika qilish uchun muhim kalitlardan foydalaniladi:
* `request_id` (so'rovni backend dan tortib agentgacha aniqlash)
* `session_id` (foydalanuvchi suhbati konteksti)
*(Asosiy kod: `internal/observability/logger.go`)*

## 4. Versiyani Bilish
Tizim qaysi build'da ekanligini bilish uchun Health gRPC chaqiruvi qilinadi. Qaytarilgan javobda `version` qiymati mavjud bo'ladi (ldflags orqali build vaqtida berilgan semver, masalan: `v0.1.0`).
*(Asosiy kod: `internal/transport/grpc/server.go` yoki `cmd/agent/main.go` Health resolver)*
