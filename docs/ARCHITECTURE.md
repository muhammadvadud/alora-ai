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

## Tool'lar Ro'yxati

| Tool nomi | Nima qiladi | Domen funksiyasi | Django'ga boradimi | State'ga nima yozadi |
| :--- | :--- | :--- | :--- | :--- |
| `product_search` | Katalogdan mato va bezaklarni qidiradi | `curtains.Search` | Yo'q (lokal cache) | `Apply("product_search", args)` |
| `set_context` | Ehtiyojni state'ga saqlaydi | `NextQuestion()` | Yo'q | `Apply("set_context", args)` |
| `curtain_calculate` | Parda va qismlari narxini hisoblaydi | `curtains.Validate`, `Calculator` | Ha (`Pricing.CalcPreview`) | `Apply("...", args)`, `LastCalc` |
| `price_quote` | Xizmatlar va chegirma bilan yakuniy narx | `Calculator`, check limits| Ha (`Pricing.CalcPreview`) | `LastQuote` |
| `build_options` | Arzon/o'rtacha/premium variantlar tuzadi | `curtains.PlanOptions`, `Calc` | Ha (3x `CalcPreview`) | `Apply("build_options", args)` |
| `order_draft` | Buyurtma qoralama ko'rish va saqlash | `Orders.SaveOrderDraft`| Ha (faqat `confirm` action'da) | `Apply("order_draft", args)` |
| `fason_lookup` | Xona/uslubga tayyor fason shablon topadi | `curtains.LookupFason` | Yo'q | Yozmaydi |
| `knowledge_lookup`| Do'kon tavsiyalari va qoidalarini topadi | `curtains.MatchKnowledge`| Yo'q | Yozmaydi |
| `calculator` | Ikki son ustida oddiy arifmetik amal | (utils) | Yo'q | Yozmaydi |
| `current_time` | Mintaqa bo'yicha joriy vaqtni qaytaradi | (utils) | Yo'q | Yozmaydi |

## Outbox Oqimi
Fayl: `internal/backend/outbox.go`, `internal/session/outbox_store.go`
* **Oqim**: `ReportUsage` va `LogMessage` kabi ma'lumotlar bevosita API ga yuborilmasdan Postgres `agent.outbox` jadvaliga tushadi. Asinxron worker ularni o'qiydi.
* **Retry jadvali**: Xatolik bo'lsa `1s, 5s, 30s, 2m, 10m` oraliqlarida takror yuboriladi. 10 marta yiqilsa `dead=true` bo'ladi.
* **Metrikalar**: Kutayotgan (`alora_outbox_pending`) va o'lgan (`alora_outbox_dead_total`) xabarlar soni Prometheus'ga uzatiladi.

## VoiceChat Oqimi
Fayl: `internal/transport/grpc/voice.go`
* **Metadata**: Ulanishda `x-session-id`, `x-user-id`, `x-role` kabilar talab qilinadi, yo'q bo'lsa `InvalidArgument` xatosi beriladi.
* **STT va Event**: Ovoz buferi yig'ilib `is_final` flagi kelganda STT bajariladi va agentga matn sifatida (`transcript` event) kiritiladi. Agar STT xato qilsa maxsus xato event qaytadi, lekin sessiya yopilmaydi.
* **Sintez va Barge-in**: Agent javobi `SentenceSplitter` bilan jumlalarga bo'linadi, `NormalizeForTTS` orqali tozalangach TTS ga yuboriladi. Agar shu vaqtda foydalanuvchidan yangi ovoz chunk'i kelsa (`barge-in`), ishlab turgan TTS uziladi (`cancel()`).


## Katalog Sinxroni
Fayl: `internal/backend/catalog_sync.go` va `internal/curtains/catalog.go`
* **Snapshot**: Agent ishga tushganda Django'dan katalog `Snapshot` orqali sahifalab (page_token) olinadi. 
* **Watch (Delta)**: So'ngra doimiy `Watch` oqimi ochiladi. O'zgarishlar (delta) kelsa qabul qilinadi. Uzilish bo'lsa exponential backoff (1s→30s) bilan qayta ulanadi. Agar versiyalar ketma-ketligi buzilsa yoki `FULL_RESYNC` signali kelsa, yana to'liq snapshot qilinadi.
* **Copy-on-write View & Stale**: Katalog xotirada `copy-on-write` mexanizmida ishlaydi (tool ishi davomida ma'lumot o'zgarmaydi). Agar oxirgi sinxronlanish vaqti belgilangan vaqtdan (10m) oshsa, katalog holati `stale = true` deb olingan holda ogohlantirish beradi.

## Sessiya Holati va Migrator
Fayl: `internal/curtains/state.go`, `internal/session/postgres.go`, `internal/session/migrations/`
* **SessionState (Stage Mashinasi)**: Suhbat qaysi bosqichda ekani `Stage` orqali (need → recommend → measure → calc → variants → choose → order) boshqariladi.
* **set_context**: Model bu tool yordamida aniqlagan ma'lumotlarni (room, style, budget) state'ga yozadi va `NextQuestion` orqali nima yetishmayotganini so'raydi.
* **session.Store**: Suhbat tarixi va State Postgres jadvalida (`agent.sessions`) yagona tranzaksiyada saqlanadi.
* **Migrator (ADR-011)**: Katta kutubxonalarsiz, `go:embed` SQL fayllari va `agent.schema_migrations` orqali ichki migrator ishlatiladi.

## System Prompt Yig'ilishi
Fayl: `internal/prompt/builder.go`
System prompt **har bir so'rovda** dinamik yig'iladi va quyidagi bloklardan iborat:
1. **Base**: Django `Catalog.Prompt` dan kelgan matn yoki `configs/prompts/system_v1.txt` dagi fallback matn.
2. **StateBlock**: Sessiyaning joriy holati (`<state>`).
3. **NextQuestion**: Keyingi aniqlanishi kerak bo'lgan ma'lumot.
4. **KnowledgeBlock**: Django'dan kelgan va filtrlangan do'kon qoidalari tavsiyalari (`<knowledge>`).
5. **CatalogMeta**: Katalog versiyasi va eskirish (stale) haqidagi ma'lumotlar.
*Izoh: Prompt Django'da tahrirlanadi, Go uni faqat o'qiydi.*

## ADR (Qarorlar Jurnali)
Qarorlar jurnali qat'iy saqlanadi (batafsil `rules.md` §19 da ko'ring: ADR-009, ADR-010, ADR-011).
**Eng Muhimi: ADR-009 — Hisob Faqat Django'da**
Buyurtmachi va jamoa qaroriga ko'ra (2026-09-13) narx hisoblash formulalari Go'da yozilmaydi. Buning o'rniga Go `curtain_calculate` va `price_quote` tool'lari ishlaganda, parametrlar `internal/backend/pricing.go` orqali Django gRPC `Pricing.CalcPreview` ga yuboriladi va tayyor hisob qabul qilinadi. Bu logikani takrorlashni va kelishmovchiliklarni oldini oladi (0 hallucination siyosati).

## Testlar Xaritasi
* **Unit Testlar**: Barcha paketlar (`go test -race ./...`).
* **Golden Dialoglar**: `tests/golden/` papkasida (20 ssenariy, mock LLM, zero hallucination).
* **Live Testlar**: Real Django bilan ishlash tekshiruvi. (`go test -tags live ./tests/live/`).
* **Load (Yuklama) Testlar**: Yuklama tag'i bilan parallel sessiyalar sadoqati (`go test -tags load ./tests/load/`).

## Konfiguratsiya va ENV Kalitlari
Fayl: `configs/config.yaml` va `.env`
Loyiha Viper orqali o'qiydi. Atrof muhit o'zgaruvchilari har doim ustun keladi (`ALORA_` prefiksi bilan, masalan `ALORA_LLM_MODEL`).

**Asosiy ENV kalitlari:**
* `DEEPSEEK_API_KEY`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY` (LLM modellari uchun)
* `ELEVENLABS_API_KEY`, `GOOGLE_API_KEY` (Ovoz API uchun)
* `AGENT_SERVICE_TOKEN` (gRPC uchun auth token)
* `DATABASE_URL` (Sessiyalar uchun Postgres URL, yo'q bo'lsa in-memory bo'ladi)
* `ALORA_BACKEND_ADDR` (Django server manzili)
