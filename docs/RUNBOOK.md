# Alora AI Agent — Operatsion Holatlar (Runbook)

Ushbu qollanma tizim ishlash jarayonidagi xatoliklarni topish va bartaraf etish bo'yicha ko'rsatmalarni taqdim etadi.

## 1. Operatsion Holatlar va Harakatlar

| Holat / Xato | Mumkin bo'lgan Sabablar | Tekshirish va Harakatlar | Fayl Havolasi |
| :--- | :--- | :--- | :--- |
| **Katalog Stale (> 10 daq)**<br>*(AI narxlar eski deydi)* | Watch oqimi uzilgan, Django gRPC yotgan, yoxud Django Redis muammosi. | 1. `/readyz` endpoint holatini tekshirish.<br>2. `/metrics` da `alora_catalog_age_seconds` va `alora_catalog_watch_connected` ni kuzatish.<br>3. Loglarda `catalog_resync` izlash. | `internal/backend/catalog_sync.go` |
| **Django Pricing yotgan**<br>*(Hisob vaqtincha ishlamayapti)* | Django server qulagan yoki bog'liqlik yo'q. Bu holatda AI "hisob xizmati vaqtincha ishlamayapti" deb narx aytmaydi. | 1. Django gRPC xizmati ishlayotganligini tekshirish (`ALORA_BACKEND_ADDR`).<br>2. Agent gRPC klient timeout'ini kuzatish. | `internal/backend/pricing.go` |
| **LLM Provayder xatosi / 429**<br>*(Rate limit, Timeout)* | Model API'dan javob kelmayapti yoki limit tugagan. | **Reja:** (Hozircha yo'q, 4.1 task'da rejalashtirilgan). Agent `fallback_provider` ni (masalan, claude) avtomatik yoqadi. Loglarda error'larni kuzating. | `internal/llm/llm.go` |
| **gRPC UNAUTHENTICATED**<br>*(Avtorizatsiya rad etildi)* | Metadata `x-service-token` va config'dagi `AGENT_SERVICE_TOKEN` mos emas. | `AGENT_SERVICE_TOKEN` (ENV) qiymatini agent va backend mijozlarida solishtirish va to'g'rilash. | `internal/transport/grpc/server.go` |
| **ratelimit RESOURCE_EXHAUSTED** | Bitta sessiya orqali juda ko'p so'rovlar (20 req/min) kelmoqda. | Agar hujum emas, balki real muhtojlik bo'lsa `ratelimit` cheklovini config orqali vaqtincha oshirish. | `internal/transport/grpc/server.go` |

## 2. Ishga Tushirish
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

## 3. Loglarni O'qish
Loglar JSON formatda `slog` orqali yoziladi. Har bir zaprosni diagnostika qilish uchun muhim kalitlardan foydalaniladi:
* `request_id` (so'rovni backend dan tortib agentgacha aniqlash)
* `session_id` (foydalanuvchi suhbati konteksti)
*(Asosiy kod: `internal/observability/logger.go`)*

## 4. Versiyani Bilish
Tizim qaysi build'da ekanligini bilish uchun Health gRPC chaqiruvi qilinadi. Qaytarilgan javobda `version` qiymati mavjud bo'ladi (ldflags orqali build vaqtida berilgan semver, masalan: `v0.1.0`).
*(Asosiy kod: `internal/transport/grpc/server.go` yoki `cmd/agent/main.go` Health resolver)*
