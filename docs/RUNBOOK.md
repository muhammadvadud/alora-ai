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

## 5. Live Sinov

Agentni real provayder (LLM) bilan lokal muhitda ko'tarish:
1. Asosiy config fayldan nusxa oling va zarur LLM kalitlarini qo'ying. Portlarni 9095 (gRPC) va 8082 (HTTP) qilib sozlang.
2. Django gRPC backend portini ko'rsating (masalan 9091).
3. Serverni ko'taring.
4. `tools/livechat` orqali gRPC so'rov yuborib agent bilan suhbatlashing.

**Nimaga e'tibor berish kerak?**
* **Tool tartibi:** LLM modelining to'g'ri tool'larni ketma-ket chaqirishi.
* **TTFT (Time To First Token):** Birinchi tokengacha bo'lgan vaqt 1.5 - 2 sekunddan oshmasligi kerak.
* **Token sarfi:** Agar 2-xabardayoq 34k gacha token sarflanayotgan bo'lsa, tarixni qisqartirish uchun konfiguratsiyada `agent.max_tool_result_chars` sozlamasidan foydalanishni inobatga oling.

## 6. Xarajat (Token Kuchaytirish)

Agent token sarfini uch sathda cheklaydi/kuzatadi: **bosqichga qarab tool**
**to'plami**, **kesh kuzatuvi** va **prompt byudjeti**.

### 6.1 Metrikalar

| Metrika | Tavsif |
| :--- | :--- |
| `alora_llm_tokens_total{provider,model,kind}` | kind = `input`/`output`/`cached` — sarf va kesh urlgan tokenlar |
| `alora_llm_cached_tokens_total{provider}` | Provider prefix-keshiga urlgan prompt tokenlar (DeepSeek `prompt_cache_hit_tokens`) |
| `alora_llm_budget_exceeded_total{provider}` | Byudjet tugagan so'rovlar soni |

### 6.2 Trace (JSONL)

`llm.trace_file` ko'rsatilgan bo'lsa har so'rov/javob qator qilib yoziladi; `usage`
maydonida `cached` qiymati bor — kesh samaradorligi uchun `prompt`/`cached` nisbatini
kuzating. Diagnostika misoli:

```bash
grep '"usage"' trace.jsonl | jq -r '[.usage.prompt,.usage.cached,.usage.completion] | @tsv'
```

### 6.3 Sozlash

| Sozlama | ENV | Qiymat | Tavsif |
| :--- | :--- | :--- | :--- |
| `agent.prompt_token_budget` | `ALORA_AGENT_PROMPT_TOKEN_BUDGET` | `40000` | Bir Stream ichida yig'ilgan prompt tokenlar chegarasi. **0 = o'chiq.** Oshsa: `error` event "javob byudjeti tugadi", `alora_llm_budget_exceeded_total` o'sadi, **tarixga yozilmaydi**. |
| `agent.max_history` | `ALORA_AGENT_MAX_HISTORY` | `40` | Tarixdagi xabarlar soni |
| `agent.max_tool_result_chars` | `ALORA_AGENT_MAX_TOOL_RESULT_CHARS` | `3000` | Tool natijasining model kontekstidagi nusxasi chegarasi |

### 6.4 Bosqichga qarab tool to'plami

Har bir sessiya Stage'iga mos tool'lar modelga beriladi (`cmd/agent/wiring.go` —
`stageTools`). *Tool syemasi qancha kichik bo'lsa, prompt token shuncha kam va
DeepSeek prefix-keshi shuncha aniq urladi.* Biznes qoidasi: hisob/order bosqichida
buyurtmadan tashqari narsani tool'larga kiritmaydi, need bosqichida esa narx
hisobi ishlamaydi. Model tanlovdan tashqari tool chaqirsa `noma'lum tool` natijasi
qaytadi va agent o'zini tuzatadi (xato sifatida o'lchanmaydi).

**Eslatma (DeepSeek):** prefix-kesh `DeepSeekReasoner`/`DeepSeekV3V2` rejimida
avtomatik; tez-tez takrorlanadigan prompt prefixi (system prompt + tarix) keshga
tushadi. Kesh xotira yoki key-limit bilan pasaysa, `alora_llm_cached_tokens_total`
o'sishi to'xtaydi — birinchi navbatda `agent.max_history`/`max_tool_result_chars`ni
kamaytirib profilni tekshiring.

*(Asosiy kod: `internal/agent/chat.go` — byudjet qo'riqchisi, `cmd/agent/wiring.go` —
tool selectori, `cmd/agent/main.go` — metrika ulanishi, `internal/llm/trace.go`)*
