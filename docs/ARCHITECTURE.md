# Alora AI Agent — Arxitektura va Tuzilma

## Loyiha Maqsadi
Alora loyihasining AI agenti — Go tilida yozilgan, gRPC orqali Django backend bilan bog'lanuvchi sun'iy intellekt xizmati. U parda va aksessuarlar savdosi bo'yicha sotuvchilarga yordamchi bo'lib, mijozlar bilan muloqot qilish, katalogdan mahsulotlarni qidirish, narxlarni hisoblash, va buyurtma loyihalarini shakllantirish jarayonlarini avtomatlashtiradi. Loyiha nol hallusinatsiya (0 hallucination) bilan ishlashga qaratilgan bo'lib, biznes mantiqi va narxlash to'liq Django backend'da boshqariladi.

## Paket Xaritasi
Loyiha `internal/` papkasida modulli tuzilishga ega:
* **`agent`**: ReAct (Reasoning and Acting) sikli (`chat.go`) va chat eventlari (`event.go`) boshqaruvi. Domen mantiqidan xoli, faqat LLM va tool'lar o'rtasida vositachi.
* **`backend`**: Go agentidan Django backend'ga gRPC so'rovlarni jo'natuvchi klient (`pricing.go`, `catalog_sync.go`, `outbox.go`). Asinxron xabarlar yuborish mexanizmini o'z ichiga oladi.
* **`config`**: `configs/config.yaml` va atrof-muhit o'zgaruvchilaridan (`ENV`) loyiha sozlamalarini o'qiydi va tekshiradi.
* **`curtains`**: Asosiy biznes-domen (katalog, variantlar, hisob interfeysi, state mashinasi, bilimlar). LLM yoki gRPC importlarisiz ishlaydigan mustaqil qatlam.
* **`llm`**: LLM provayderlari (DeepSeek, Claude, OpenAI) bilan ishlash adapterlari va token/usage hisobi.
* **`observability`**: Loglash (`slog`), metrikalar (Prometheus) va tracing vositalari.
* **`prompt`**: System prompt'ni dinamik ravishda turli bloklardan yig'ish (Django qismi, holat, bilimlar, meta).
* **`session`**: Foydalanuvchi suhbat tarixi va sessiya holati (`StateStore`), Postgres'da `sessions` jadvalida saqlanadi.
* **`tools`**: LLM tool'larining yupqa adapterlari (`product_search.go`, `curtain_calculate.go`, `price_quote.go`). Domen va DTO validatsiyasini birlashtiradi.
* **`transport`**: Tashqi dunyo bilan aloqa. Asosan `grpc` (`server.go`), shuningdek `http` (metrics/health) va `cli` (terminalda sinash).

**Bog'liqlik yo'nalishi:**
```text
transport ──► agent ──► tools ──► curtains
    │            │                   ▲
    │            └──► session        │
    ├──► backend ────────────────────┘ (faqat types)
    └──► voice
prompt ──► curtains (state, knowledge)
```

## Bitta Chat So'rovining Oqimi
Chat so'rovi (misol: gRPC `Chat.Stream`) quyidagi ketma-ketlikda o'tadi:
1. **Django SSE → gRPC Agent.Chat**: Foydalanuvchi so'rovi Django backend orqali gRPC ga yetib keladi.
2. **Interceptorlar**: Request-id beriladi, `x-service-token` bilan avtorizatsiya qilinadi, log va metrikalar boshlanadi, rate limit tekshiriladi (`transport/grpc/server.go`).
3. **Chat.Stream (ReAct)**: `agent/chat.go` LLM bilan muloqotni boshlaydi.
4. **Tool Chaqiruvlari**: 
   * Agar qidiruv bo'lsa: `product_search` lokal `curtains.Catalog` cache'iga murojaat qiladi.
   * Agar hisob bo'lsa: `curtain_calculate` / `price_quote` → `backend.PricingClient` orqali Django `Pricing.CalcPreview` ga murojaat qiladi.
5. **Event'lar Generatsiyasi**: ReAct sikli jarayonida asinxron eventlar ketma-ketligi chiqadi: `start` → `tool_call` → `tool_args` → `tool_result` → `products` (agar mahsulot topilsa) → `content` (LLM javobi matni) → `stage` (suhbat bosqichi) → `done` (yakun, usage bilan).
6. **pb Mapping → Django**: `transport/grpc/mapping.go` da Go eventlari `pb.ChatEvent` ga o'tkazilib Django orqali mijozga uzatiladi.

## Katalog Sinxroni
Fayl: `internal/backend/catalog_sync.go` va `internal/curtains/catalog.go`
* **Snapshot**: Agent ishga tushganda Django'dan katalog `Snapshot` orqali sahifalab (page_token) olinadi. 
* **Watch (Delta)**: So'ngra doimiy `Watch` oqimi ochiladi. O'zgarishlar (delta) kelsa qabul qilinadi. Uzilish bo'lsa exponential backoff (1s→30s) bilan qayta ulanadi. Agar versiyalar ketma-ketligi buzilsa yoki `FULL_RESYNC` signali kelsa, yana to'liq snapshot qilinadi.
* **Copy-on-write View & Stale**: Katalog xotirada `copy-on-write` mexanizmida ishlaydi (tool ishi davomida ma'lumot o'zgarmaydi). Agar oxirgi sinxronlanish vaqti belgilangan vaqtdan (10m) oshsa, katalog holati `stale = true` deb olingan holda ogohlantirish beradi.

## Sessiya Holati
Fayl: `internal/curtains/state.go` va `internal/session/postgres.go`
* **SessionState (Stage Mashinasi)**: Suhbat qaysi bosqichda ekani `Stage` orqali (need → recommend → measure → calc → variants → choose → order) boshqariladi.
* **set_context**: Model bu tool yordamida aniqlagan ma'lumotlarni (room, style, budget) state'ga yozadi va `NextQuestion` orqali nima yetishmayotganini so'raydi.
* **session.Store**: Suhbat tarixi va State Postgres jadvalida (`sessions`) yagona tranzaksiyada saqlanadi.

## System Prompt Yig'ilishi
Fayl: `internal/prompt/builder.go`
System prompt dinamik yig'iladi va quyidagi bloklardan iborat:
1. **Base**: Django `Catalog.Prompt` dan kelgan matn yoki `configs/prompts/system_v1.txt` dagi fallback matn.
2. **StateBlock**: Sessiyaning joriy holati (`<state>`).
3. **NextQuestion**: Keyingi aniqlanishi kerak bo'lgan ma'lumot.
4. **KnowledgeBlock**: Django'dan kelgan va filtrlangan do'kon qoidalari tavsiyalari (`<knowledge>`).
5. **CatalogMeta**: Katalog versiyasi va eskirish (stale) haqidagi ma'lumotlar.
*Izoh: Prompt Django'da tahrirlanadi, Go uni faqat o'qiydi.*

## ADR (Qarorlar Jurnali)
Qarorlar jurnali qat'iy saqlanadi (batafsil `rules.md` §19 da ko'ring).
**Eng Muhimi: ADR-009 — Hisob Faqat Django'da**
Buyurtmachi va jamoa qaroriga ko'ra (2026-09-13) narx hisoblash formulalari Go'da yozilmaydi. Buning o'rniga Go `curtain_calculate` va `price_quote` tool'lari ishlaganda, parametrlar `internal/backend/pricing.go` orqali Django gRPC `Pricing.CalcPreview` ga yuboriladi va tayyor hisob qabul qilinadi. Bu logikani takrorlashni va kelishmovchiliklarni oldini oladi (0 hallucination siyosati).

## Konfiguratsiya va ENV Kalitlari
Fayl: `configs/config.yaml` va `.env`
Loyiha Viper orqali o'qiydi. Atrof muhit o'zgaruvchilari har doim ustun keladi (`ALORA_` prefiksi bilan, masalan `ALORA_LLM_MODEL`).

**Asosiy ENV kalitlari:**
* `DEEPSEEK_API_KEY`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY` (LLM modellari uchun)
* `ELEVENLABS_API_KEY`, `GOOGLE_API_KEY` (Ovoz API uchun)
* `AGENT_SERVICE_TOKEN` (gRPC uchun auth token)
* `DATABASE_URL` (Sessiyalar uchun Postgres URL, yo'q bo'lsa in-memory bo'ladi)
* `ALORA_BACKEND_ADDR` (Django server manzili)
