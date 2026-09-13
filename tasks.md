# Alora AI Agent (Go) — Ish rejasi (tasks.md)

> Manba: `plan/02-AI-TZ.md` + `plan/00-ALORA-UMUMIY-TZ.md` · Qoidalar: `rules.md`
> Backend shartnoma: `projects/backend/protos/alora.proto`, `docs/GO-CONTRACT.md`, `tests/fixtures/calc_cases.json`
> Belgilar: `[ ]` boshlanmagan · `[~]` jarayonda · `[x]` tugallangan (test o'tgan) · `[!]` backend bilan kelishuv kutilmoqda
> Har bir ish tugagach shu fayl **o'sha ishning commit'i ichida** yangilanadi.
> Har task uchun "Qabul" — nima o'tsa bajarilgan hisoblanadi (rules.md §17.5 DoD).

**Umumiy holat:** 91 / 112 · Senior audit 2026-09-13 (docs/REVIEW-2026-09-13.md): 3 HIGH + 7 MEDIUM + 2 LOW tuzatildi, CI'da Postgres · backend tasks 100% · API kalit kutayotganlar: 1.43, 1.44 (mock e2e tayyor), 1.46, 2.21, 2.23, 3.2/3.3, 3.7/3.8, 3.10 · kontrakt 1.5 taklifi: docs/CONTRACT-PROPOSAL-1.5.md

**Bosqich xaritasi:** 0 Poydevor (1 hafta) → 1 MVP (3–4 hafta) → 2 To'liq sotuv sikli (3 hafta) → 3 Ovoz (2–3 hafta) → 4 Sayqal (davomiy)

---

## Bosqich 0 — Poydevor (shablon → Alora skeleti, shartnoma, CI)

### 0.A Repo va shablon

- [x] 0.1 `UzStack/agent` shablonini `alora-agent` sifatida ko'chirish; `go.mod` moduli
      `github.com/muhammadvadud/alora-ai`; barcha importlar yangilanadi.
      *Qabul:* `go build ./...` toza.
- [x] 0.2 Keraksizni (telegram ham, foydalanuvchi qarori) o'chirish: `internal/rocketpark`, `internal/weather`, `internal/transport/telegram`,
      `k8s/rocketpark-bot.yaml`, `tools/rocketpark.go`, `tools/weather.go`, mos testlar; `go mod tidy`.
      *Qabul:* `go build && go test ./...` toza, `main.go` da faqat `Calculator`, `Clock` qoladi.
- [x] 0.3 `AGENTS.md` ni Alora'ga moslash (rules.md §0 va §2 ga havola, shablon "oltin qoidalar" saqlanadi);
      `README.md` (ishga tushirish, ENV, make buyruqlari); `LICENSE`, `.editorconfig`, `.gitignore`
      (`bin/`, `tmp/`, `.env`, `coverage.out`).
- [x] 0.4 `Makefile`: `build test vet lint fmt tidy proto check-fixtures sync-fixtures bench docker run-cli run-grpc`.
- [x] 0.5 `.golangci.yml` — rules.md §3.1 linterlari + `depguard` qoidalari (§2 bog'liqlik yo'nalishi:
      `curtains` da `eino/grpc/gen` taqiq, `tools` da `gen` taqiq).
      *Qabul:* `golangci-lint run` toza, ataylab noto'g'ri import qo'shilganda fail bo'ladi (test).
- [x] 0.6 Pre-commit: `gofmt`, `goimports`, `golangci-lint --fast`, `gitleaks`.
- [x] 0.7 `.env.example` (`DEEPSEEK_API_KEY`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `ELEVENLABS_API_KEY`,
      `GOOGLE_API_KEY`, `AGENT_SERVICE_TOKEN`, `DATABASE_URL`, `ALORA_BACKEND_ADDR`) — qiymatsiz.

### 0.B Proto va generatsiya

- [x] 0.8 `buf.yaml`, `buf.gen.yaml` (`protoc-gen-go`, `protoc-gen-go-grpc`, versiya pin); `gen/alorav1/`.
- [x] 0.9 `scripts/import_proto.sh` — backend `protos/alora.proto` → `proto/alora.proto` (diff ko'rsatadi,
      backend `scripts/export_proto.sh` bilan juft). *Qabul:* `make proto` → `gen/` yangilanadi, build toza.
- [x] 0.10 **Backend bilan (P6):** `go_package` = `github.com/muhammadvadud/alora-ai/gen/alorav1;alorav1`.
      Backend tomoni: `feat/proto-agent-contract-ext` branch'ida bajarildi (kontrakt 1.1). Go: `import_proto` + `buf generate` (0.9 bilan).
- [x] 0.11 **Backend bilan (P1):** `CatalogSnapshot` 8–11 (`service_prices`, `discount_tiers`, `discount_policy`,
      `order_defaults`) + delta entity'lari — backend'da bajarildi (kontrakt 1.1, decimal matn). Go: mapping (2.17).
- [x] 0.12 **Backend bilan (P2):** `CatalogFason` (12) + delta `fason` (bo'lak o'zgarsa butun fason) — backend'da bajarildi. Go: mapping (2.17).
- [x] 0.13 **Backend bilan (P3):** `CatalogPrompt` (13) + delta `prompt` — backend'da bajarildi. Go: mapping (2.18).
- [x] 0.14 **Backend bilan (P4, P5, P7, P10):** `GO-CONTRACT.md` 1.1 da yozildi va Django'da amalga oshirildi:
      `kind="stage"` → `Conversation.stage` (Celery); `context` = `role`, `customer_mode`, `conversation_id`;
      `Usage.cost_*` faqat Django (Go `0`); `action="FULL_RESYNC"` (katta harf, entity `catalog`, stream yopiladi).
      **Diqqat:** `x-service-token`/`x-request-id` Django hali yubormaydi (backend 6.18) — Go auth `grpc.auth_required=false` bilan boshlanadi.
- [x] 0.15 CI'da `buf lint` + `buf breaking --against 'https://github.com/muhammadvadud/alora-backend.git#branch=main,subdir=protos'`.

### 0.C Etalon va CI

- [x] 0.16 `scripts/sync_fixtures.sh` — backend `tests/fixtures/calc_cases.json` → `tests/fixtures/`;
      `make check-fixtures` sha256 solishtiradi. *Qabul:* fixture farq qilsa CI qizil.
- [x] 0.17 GitHub Actions: `buf lint → build → vet → golangci-lint → test -race -cover → coverage gate
      (curtains ≥ 90 / tools ≥ 85 / umumiy ≥ 75) → govulncheck → check-fixtures → docker build`.
- [x] 0.18 `docker/Dockerfile` (multi-stage, distroless, non-root, `HEALTHCHECK /readyz`);
      `docker-compose.yml` — `agent` servisi backend compose tarmog'i `alora_net` ga ulanadi
      (`postgres`, `django:9091` ko'rinadi). *Qabul:* `docker compose up agent` → `/healthz` 200.
- [x] 0.19 `internal/observability`: `slog` JSON (`request_id`, `session_id`, `user_id` maydonlari),
      Prometheus registry, `/metrics`, `/healthz`, `/readyz` (`transport/http` faqat shu).
      *Qabul:* `curl :8081/metrics` da `alora_*` metrikalar ko'rinadi.
- [x] 0.20 `internal/config` kengaytmasi (TZ §9): `grpc`, `backend`, `catalog`, `stt`, `tts`, `knowledge`, `db`,
      `fallback` bo'limlari; `ALORA_` ENV prefiks; `Validate()`; `providerKeyEnv` ga `elevenlabs`, `google`.
      *Qabul:* `config_test.go` — noto'g'ri `stale_after`, bo'sh `backend.addr` (grpc mode) → xato.
- [x] 0.21 `clock` interfeysi (`internal/curtains/clock.go` yoki `internal/common`), testlarda `FakeClock`.

---

## Bosqich 1 — MVP (matnli chat: qidiruv + hisob + narx, gRPC, snapshot sync)

### 1.A Domen turlari va katalog cache

- [x] 1.1 `curtains/types.go` — `Product`, `Variant` (`PriceUnit` typed), `Category`, `CalcRule`, `Service`,
      `DiscountTier`, `DiscountPolicy`, `OrderDefaults`, `Fason{Parts, Accessories}`, `KnowledgeRule`, `PromptVersion`;
      barcha pul/o'lcham `decimal.Decimal`; `PartRole`, `Segment`, `Room`, `Style`, `Pattern`, `Transparency` const'lari
      (`product_search` enum'lari bilan bir xil).
- [x] 1.2 `curtains/catalog.go` — `Catalog` + `View` (copy-on-write, `atomic.Pointer`), `ApplySnapshot(page)`,
      `ApplyDelta(d)` (idempotent, version ketma-ketlik tekshiruvi → `ErrVersionGap`), `View().Stale`,
      `Version()`, `SyncedAt()`. *Qabul:* `catalog_test.go` — snapshot sahifalab yig'iladi; delta create/update/delete;
      takroriy version e'tiborsiz; gap → xato; stale `FakeClock` bilan; `-race` bilan parallel o'qish/yozish.
- [x] 1.3 `curtains/curtainstest/fixture.go` (`//go:build test` emas — `internal/curtains/testutil`) — `NewCatalogFromFixture(calc_cases.json)`:
      `catalog_seed` + `pricing_seed` dan `View`. *Qabul:* fixture'dagi barcha variant ID lar topiladi.
- [x] 1.4 `catalog_bench_test.go` — 10k mahsulot × 3 variant generatsiya; xotira ≤ 200 MB, `View()` ≤ 1 µs.

### 1.B Hisob — Django CalcPreview orqali (ADR-009)

- [x] 1.5 `curtains/calc_types.go` — `CalcInput`/`CalcResult` DTO, Django payload/preview JSON bilan bir xil kalitlar (decimal string).
- [x] 1.6 `curtains/validate.go` — rules §4.2.3 chegaralari (Σwidth qoidasi yo'q, ADR-010); `ValidationError`.
      *Qabul:* `validate_test.go` — har chegara +/− (L=0.4, 15.1; H; 11 bo'lak; K=0.9/4.1; qty=0; nofaol variant).
- [!] 1.7 **Backend bilan:** proto `service Pricing { rpc CalcPreview(CalcPreviewRequest) returns (CalcPreviewResponse) }`
      (`payload_json`, `user_role` → `result_json`, `error`, `error_fields_json`); Django servicer `calculate_preview` ni chaqiradi
      (backend tasks 6.19). Go: `import_proto` + `buf generate`.
- [x] 1.8 `curtains/calc.go` — `Calculator` interfeysi; `backend/pricing.go` — gRPC klient (timeout 3s, `UNAVAILABLE` da 1 retry,
      `ValidationError` mapping). **`calc_test.go` — 20 etalon:** fake Pricing server `expected` qaytaradi; request `payload_json`
      Django `input` bilan bir xil (JSON tenglik), javob `CalcResult` ↔ `expected` Decimal tengligi. *Qabul:* 20/20.
- [ ] 1.9 `curtains/explain.go` — `explanation`/`warnings`: chiqindi (Django `warnings` + `waste_percent`), koeffitsient −0.5 tejami
      (ikkinchi CalcPreview, parallel), arzonroq analog (katalog, ≤ 80% narx/m²). *Qabul:* fake server bilan 3 ssenariy.
- [x] 1.10 Live test (`//go:build live`): real Django (`docker compose`) bilan 20/20 CalcPreview.
- [ ] 1.11 Etalonda ≥ 6 ta ko'p matoli holat borligini tekshiruvchi test (`TestFixture_HasMultiPartCases`) — hozir 4 ta (2, 3, 3, 10 bo'lak) → backend bilan 2 ta qo'shiladi [!].

### 1.C Qidiruv

- [x] 1.12 `curtains/search.go` — `SearchQuery` (TZ §3.1 maydonlari), filtr (`kind` qoidasi: `per_sqm` ↔ fabric,
      `per_piece/per_meter` + `accessories` kategoriya ↔ accessory; `is_active`; `max_price`; `segment`),
      skorlash (xona +3, uslub +3, bezak +2, shaffoflik +2, segment +2, rang +1), deterministik tartib, `why` yig'ish.
      *Qabul:* `search_test.go` — tartib, teng ball hal qilinishi, `kind` ajratish, `limit`, bo'sh natija.
- [x] 1.13 `curtains/search_colors.go` — rang sinonimlari (uz lotin/kirill/ru/en) + hex ΔE. 100% test.
- [x] 1.14 `search_bench_test.go` — 10k mahsulotda ≤ 5 ms.

### 1.D Narx (price_quote)

- [x] 1.15 `tools/price_quote.go` (quote.go o'rniga — hisob Django'da) — `price_quote` = CalcPreview (`services`, `delivery`, `discount_percent`, `role`) + Go'da faqat:
      `prepayment`/`balance` (`OrderDefaults.prepayment_percent` yoki kiritilgan), qo'lda chegirma rol chegarasidan oshsa
      **oldindan** `{"error":"chegirma chegarasi 5% — 12% berib bo'lmaydi"}` (Django jim kesadi — AI aniq aytishi kerak),
      karniz `cornice` xizmati katalogda bo'lmasa `warnings`. *Qabul:* fake server bilan test — chegara xabari, avans/qoldiq.
- [x] 1.16 `LastCalc`/`LastQuote` `SessionState` da saqlanadi (1.25 ga bog'liq) — `price_quote` `LastCalc` siz xato.

### 1.E Tool adapterlari

- [x] 1.17 `tools/tools.go` — `Deps{Catalog *curtains.Catalog, State curtains.StateStore, Backend backend.Client, Clock}`;
      `must()`; `displayMoney()` (`5 382 000 so'm`), `truncate(8KB)`.
- [x] 1.18 `tools/dto.go` — `productSearchArgs/Result`, `calcArgs` (parts ≤ 10, accessories), `quoteArgs`, `ProductCard`;
      barcha `jsonschema` teglari, `Error` maydonlari. *Qabul:* `dto_test.go` — `InferTool` sxemasida enum/description bor.
- [x] 1.19 `tools/product_search.go` — adapter; natija ≤ 10; `stale` bayrog'i; katalog bo'sh → `error`;
      `KindProducts` uchun `ProductRef` ro'yxati `ctx` orqali agentga uzatiladi (`agent.AttachProducts(ctx, refs)`).
      *Qabul:* `product_search_test.go`.
- [x] 1.20 `tools/curtain_calculate.go` — float64 → decimal, validatsiya, `Calculate`, state'ga `LastCalc`, `explanation/warnings`.
      *Qabul:* test — 1 bo'lak, 10 bo'lak, 11 bo'lak (xato), noto'g'ri variant, faol emas variant, stale.
- [x] 1.21 `tools/price_quote.go` — adapter; rol `ctx` dan; `LastCalc` yo'q → xato. *Qabul:* test.
- [x] 1.22 `main.go` registratsiya: `Calculator`, `Clock`, `ProductSearch`, `CurtainCalculate`, `PriceQuote` — ochiq.

### 1.F Agent kengaytmalari

- [x] 1.23 `agent/event.go` — `KindProducts`, `KindStage`; `Event.Products []ProductRef`, `Event.Usage *Usage`.
      Shablon `history_test.go` o'zgarishsiz o'tadi.
- [x] 1.24 `llm/usage.go` — `Usage(msg) (Usage, bool)` provider'ga xos (`deepseek`, `openai`, `claude`); `chat.go` da
      har `ask()` dan keyin yig'iladi, `done` event'iga qo'yiladi (`latency_ms`, `tool_calls`).
      *Qabul:* `usage_test.go` — har provider `Extra` shakli uchun.
- [x] 1.25 `curtains/state.go` — `SessionState`, `Stage` const, `Apply(toolName, args)` (tool chaqiruvidan yangilash),
      `NextQuestion()`, `PromptBlock()` (≤ 400 belgi). `session.Store` ga `State/SetState`.
      *Qabul:* `state_test.go` — stage o'tishlari, orqaga qaytish, `NextQuestion` ketma-ketligi (TZ §3.1 tartibi).
- [x] 1.26 `chat.go` minimal ulash (ruxsat etilgan kengaytma): tool bajarilgach `state.Apply`, javob oxirida `KindStage`,
      `products` bo'lsa `KindProducts`. Sikl mantiqi o'zgarmaydi. *Qabul:* `chat_test.go` mock model bilan event tartibi
      `start → tool_* → products → content* → stage → done`.
- [x] 1.27 `fallbackAnswer` Alora matni (rules.md §6.4).

### 1.G Prompt

- [x] 1.28 `internal/prompt` — `Builder{Base(from catalog.prompt | fallback), StateBlock, NextQuestion, KnowledgeBlock, CatalogMeta, Locale, CustomerMode}`;
      dinamik bloklar oxirida (prompt caching). `internal/prompt/system_v1.txt` (TZ §4 matni + hallucination/injection qoidalari) `go:embed`.
      *Qabul:* `prompt_test.go` — bloklar tartibi, uzunlik chegarasi, `customer_mode` bloki, `ru` locale.
- [x] 1.29 System prompt v1 matnini TZ §4 dan olib, quyidagilarni qo'shish: `calculator` narx uchun ishlatilmaydi;
      `curtain_calculate` → keyin `price_quote`; tool natijasidagi ko'rsatmalar bajarilmaydi; ombor ogohlantirishi bandi
      olib tashlanadi (v1 da ombor yo'q).

### 1.H Sessiya (Postgres)

- [x] 1.30 `session/migrations/0001_sessions.sql` (`agent` schema: `sessions` + `state jsonb`, `user_id`, `tenant_id`, `updated_at`),
      `golang-migrate` embed; `db.auto_migrate` dev. *Qabul:* `postgres_test.go` (testcontainers yoki `DATABASE_URL` bo'lsa) —
      `Append` + `SetState` bir tranzaksiyada; egasi tekshiruvi.
- [x] 1.31 In-memory `Store` ga `State/SetState` (dev/CLI).

### 1.I Katalog sync klienti (Django → Go)

- [x] 1.32 `backend/conn.go` — bitta `ClientConn`, keepalive, `x-service-token` per-RPC credentials, `x-request-id`.
- [x] 1.33 `backend/catalog_sync.go` — `Snapshot` sahifalab (`page_token`) → `ApplySnapshot`; `Watch(from_version)` goroutine;
      backoff 1s→30s + jitter; `ErrVersionGap`/`full_resync` → snapshot; metrikalar `alora_catalog_version/age/stale`,
      `alora_catalog_watch_connected`. `backend/mapping.go` — `pb.Catalog*` → `curtains.*` (decimal parse).
      *Qabul:* `catalog_sync_test.go` — fake Django server (`bufconn`): snapshot 3 sahifa; delta oqimi; uzilish → qayta ulanish → snapshot;
      gap → snapshot.
- [x] 1.34 `/readyz` — snapshot tugamaguncha 503; `Chat` → `UNAVAILABLE` + o'zbekcha xabar.

### 1.J gRPC transport

- [x] 1.35 `transport/grpc/server.go` — `Agent` server: `Chat` (ctx: session, user, tenant, role, locale, customer_mode),
      `ResetSession`, `Health`; `VoiceChat` → `UNIMPLEMENTED` (3-bosqichgacha).
- [x] 1.36 `transport/grpc/mapping.go` — `agent.Event` → `pb.ChatEvent` (kind string, products, usage); `thinking` filtrlanadi.
      *Qabul:* `mapping_test.go` — har kind uchun.
- [x] 1.37 Interceptorlar: recovery, request-id, auth (`ConstantTimeCompare`), logging, metrics, ratelimit (20/min/sessiya).
      *Qabul:* `interceptors_test.go` — token yo'q → `UNAUTHENTICATED`; `Health` tokensiz o'tadi; limit → `RESOURCE_EXHAUSTED`.
- [x] 1.38 Server sozlamalari: keepalive 30s/`MinTime 10s`, max msg 4 MB, `MaxConcurrentStreams`, graceful stop (SIGTERM → `GracefulStop` 15s).
- [x] 1.39 `main.go`: `mode: grpc` → HTTP health (`:8081`) + gRPC (`:9090`) parallel, `errgroup`; `mode: cli` — dev.
- [~] 1.40 `tests/e2e/chat_test.go` — `bufconn`: fake Streamer bilan `Chat` stream tartibi; fake Django + mock LLM 1.41 da.
      `ResetSession`, `Health.status` (`ok`/`degraded` stale bilan).

### 1.K MVP yakuni

- [x] 1.41 `tests/golden/` karkasi + mock `ToolCallingChatModel` (skript: user matn → tool chaqiruv/javob) — 5 ta MVP ssenariy:
      mehmonxona hi-tech to'liq oqim (need→calc→quote), noto'g'ri o'lcham (L=50), katalogda yo'q mahsulot, stale katalog,
      chegirma chegarasi. *Qabul:* `go test ./tests/golden/...` o'tadi; `forbid_content_regex` narx uchun ishlaydi.
- [x] 1.42 `hallucination_test.go` (rules.md §14.5). *Qabul:* CI'da `-run Halluc` alohida job.
- [x] 1.43 (2026-09-14: deepseek-v4-pro, 3 xabar, set_context→product_search→curtain_calculate→price_quote, TTFT 2.3–4.2 s, narxlar tool'dan) Live sinov (`//go:build live`, DeepSeek): 5 ssenariy, TTFT/tool soni `docs/LIVE-RUNS.md` ga.
      *Qabul:* TTFT p95 ≤ 1.5 s, 2-tool javob ≤ 6 s (bo'lmasa `max_tokens`/prompt hajmi sozlanadi, natija yoziladi).
- [x] 1.44 Backend bilan integratsiya sinovi (`tests/e2e/django_sse_test.go`, real DeepSeek): Django SSE → Go → tool'lar → Django Pricing → SSE; `products`, `done.usage`, tool natijasidan kelgan summa, xabarlar va usage summary tekshiriladi. Tarixdagi eski tool natijalari model nusxasida 3000 rune bilan cheklanadi.
- [x] 1.45 `docs/ARCHITECTURE.md` (paket xaritasi, oqim diagrammasi), `docs/RUNBOOK.md` (stale, outbox dead, Watch uzilishi nima qilish).
- [ ] 1.46 Teg `v0.1.0`.

---

## Bosqich 2 — To'liq sotuv sikli (variantlar, buyurtma, bilimlar, fason, usage)

### 2.A Backend'ga yozuv (outbox)

- [x] 2.1 `session/migrations/0002_outbox.sql` (`outbox(id, kind, payload jsonb, attempts, next_at, created_at)`), in-memory variant.
- [x] 2.2 `backend/outbox.go` — `Enqueue(kind, payload)`, worker (batch 50, retry jadvali `1s,5s,30s,2m,10m`, 10 → `dead`),
      metrikalar `alora_outbox_pending/dead_total`. *Qabul:* `outbox_test.go` — fake Django yotganda yig'iladi, tirilganda bo'shaydi; dead.
- [x] 2.3 `backend/usage.go` — `done.usage` → `UsageRequest` (`message_id`, `conversation_id` `context` dan), outbox orqali.
- [x] 2.4 `backend/log.go` — `LogMessage` (user, assistant, tool) — `pii.Mask()` bilan; `seq` idempotent.
- [x] 2.5 `internal/pii` — `Mask(phone)`, `MaskName`, `ScrubToolResult` — 100% test.
- [x] 2.6 `Health.status=degraded` outbox > 1000 yoki dead > 0 bo'lsa.

### 2.B Fason va bilimlar

- [x] 2.7 `curtains/fason.go` — `Fason` → `[]calcPart` (share × L/H, koeffitsient, rol), `LookupFason(room, style)`;
      `tools/fason_lookup.go`. *Qabul:* `fason_test.go` — ulushlar yig'indisi, qatlam/bo'lak ajratish, mos fason yo'q.
- [x] 2.8 `curtains/knowledge.go` — `Match(state)`, `priority` tartib, `PromptBlock(limit)`; `tools/knowledge_lookup.go`
      (faqat `inline_max` dan oshsa ro'yxatga qo'shiladi — `main.go` shart bilan). *Qabul:* `knowledge_test.go`.
- [x] 2.9 Prompt'ga `<knowledge>` bloki + injection sarlavhasi; `description`/`recommendation` sanitizatsiya (500 belgi, HTML).

### 2.C Variantlar (build_options)

- [x] 2.10 `curtains/variants.go` — 3 paket (`economy/standard/premium`), `coef_*` kalitlari (fallback `coef_default`),
      qatlam tarkibi, invariant `economy ≤ standard ≤ premium` (qayta urinish ≤ 3), bo'sh segment → `null` + warning,
      qisqa taqqoslash matni (kodda, LLM emas: "Premium o'rtachadan 2.1 mln qimmat: astar + lambrekan + yangi karniz").
      *Qabul:* `variants_test.go` — tartib invarianti 50 random katalogda (property test), bo'sh segment.
- [x] 2.11 `tools/build_options.go` — adapter; `KindProducts` ga 3 paket mahsulotlari; state `Stage=variants`, `ChosenVariants`.
- [x] 2.12 Golden: "3 variant → 'arzonroq' e'tirozi → o'rtacha paketda K 2.5→2.0" ssenariysi.

### 2.D Buyurtma qoralamasi

- [x] 2.13 `tools/order_draft.go` — `action=quote|draft|confirm`; `missing[]`; `confirm` → `backend.SaveOrderDraft` sinxron 5s,
      `items_json` = `LastQuote` (Django `OrderMeasurement/Part/Accessory` shakli — backend serializer bilan kelishiladi [!]);
      `customer_ref` bilan PII maskalash; natija `order_id`, `order_number`. *Qabul:* test — missing ketma-ketligi, Django xatosi,
      PII model kontekstiga tushmagani (confirm natijasida telefon/ism yo'q, faqat `customer_ref`).
- [x] 2.14 **Backend bilan:** `OrderDraftRequest.items_json` sxemasi (`calc_cases.json` `input` formati bilan bir xil deb taklif) —
      Go tomoni: `items_json` = `CalcInput.MarshalJSON()` (Django payload sxemasi, `GO-CONTRACT.md` ga yoziladi).
- [x] 2.15 `set_context` tool (ADR-003) — `tools/set_context.go`, `dto`; `NextQuestion` qaytaradi. *Qabul:* test + golden'da ishlatiladi.
- [x] 2.16 `main.go` ro'yxat: + `BuildOptions`, `OrderDraft`, `FasonLookup`, `KnowledgeLookup?`, `SetContext`.

### 2.E Sinxron kengaytmalar (proto P1–P3 kelgach)

- [x] 2.17 `backend/mapping.go` — `service_prices`, `discount_tiers`, `discount_policy`, `order_defaults`, `fasons`, `prompt` →
      `Catalog`; delta entity'lari. 1.15 dagi stub'lar olib tashlanadi. *Qabul:* fake server bilan sync testi kengaytirildi.
- [x] 2.18 Prompt Django versiyasidan (`catalog.prompt`), `prompt` delta → keyingi so'rovdan yangi prompt; log `info` versiya.
- [ ] 2.19 Katalog "stale" → `KindStage`/`done` da `warnings` + Django'ga `LogMessage` `kind=warning`? (kelishiladi) — hozircha metrika + log.

### 2.F Sifat

- [x] 2.20 Golden ssenariylar 20 taga yetkaziladi (TZ §12): xona turlari ×7, yorug'lik, balkon eshigi, ko'p matoli (5 bo'lak),
      10 bo'lak, chiqindi ogohlantirishi, e'tirozlar ("qimmat", "boshqa rang"), buyurtma to'liq, orqaga qaytish (o'lcham o'zgardi),
      rus tili, mijoz rejimi (tannarx aytilmaydi).
- [~] 2.21 Live sinov 20 ssenariy (DeepSeek/Claude endpointlari A/B uchun qayta ishga tushiriladi) → `tests/live/README.md`;
      real gRPC LLM oqimi, tool/stage/error tekshiruvi va narx hallucination regex audit qo'shildi. Xarajat limiti sabab 40 suhbat
      (20 + 20) ishlatiladi; 100 suhbatlik TZ auditi bajarilmadi.
- [x] 2.22 Yuklama testi: 50 parallel sessiya (`ghz` yoki Go bench, mock LLM) — xotira, goroutine sizishi (`goleak`).
- [ ] 2.23 Teg `v0.2.0`.

---

## Bosqich 3 — Ovoz (STT/TTS, VoiceChat, barge-in)

- [x] 3.1 `voice/voice.go` — `STT`, `TTS` interfeyslari, `Usage`, registry (`init()/register`), config'dan `New`.
- [ ] 3.2 `voice/whisper.go` (OpenAI Whisper API, `uz`/`ru`, auto), `voice/gemini_stt.go` (zaxira).
      *Qabul:* HTTP mock bilan test; kalit ENV.
- [ ] 3.3 `voice/elevenlabs.go` (stream chunk), `voice/google_tts.go` (zaxira, uz ovozi). *Qabul:* mock test.
- [x] 3.4 `voice/normalize.go` — raqam→so'z (uz lotin/kirill, ru), `m²`, `K=2.5`, telefon o'qilmaydi. 100% test, 40+ holat.
- [x] 3.5 `voice/sentence.go` — jumla chegarasi detektori (`. ! ?` + bo'sh joy, 120 belgi, qisqartmalar `sm.`, `mln.`). Test.
- [x] 3.6 `transport/grpc/voice.go` — `VoiceChat` bidirectional: chunk bufer → `is_final` → STT → `transcript` event →
      `Chat.Stream` → `text` event'lar + jumla-jumla TTS goroutine → `audio` event; barge-in (`cancel()` yangi chunk'da);
      `stt_seconds/tts_chars` `done.usage` ga; STT xatosi → `error kind=stt`, sessiya tirik.
      *Qabul:* `voice_test.go` — fake STT/TTS bilan event tartibi; barge-in TTS to'xtatadi; ctx bekor → goroutine sizmaydi (`goleak`).
- [~] metrikalar bor, live o'lchov kalit kutmoqda
- [ ] 3.8 Django WebSocket gateway bilan integratsiya sinovi (backend `apps/chat/consumers.py`) — audio format kelishuvi
      (`pcm_s16le 16k` kirish; chiqish `mp3`) `GO-CONTRACT.md` ga [!].
- [x] 3.9 `voice_test.go` da 3 ssenariy (transcript → tool → audio).
- [ ] 3.10 Teg `v0.3.0`.

---

## Bosqich 4 — Sayqal (davomiy)

- [x] 4.1 Fallback provider (`fallback_provider: claude`) — 2 ketma-ket xato → o'sha navbat qayta; `usage.provider` to'g'ri;
      `alora_llm_fallback_total`. *Qabul:* mock provider xato → fallback ishladi testi.
- [ ] 4.2 Prompt A/B: `context["prompt_version"]` bilan Django tanlagan versiyani ishlatish (bir nechta versiya cache'da).
- [x] 4.3 mTLS (`grpc.tls` bo'limi, sertifikat ENV yo'llari), `docs/DEPLOY.md`.
- [ ] 4.4 Semantik qidiruv: pgvector + embedding (`product_search` skoriga `+2` semantik), faqat 10k+ da yoki sifat talab qilsa (ADR kerak).
- [ ] 4.5 Rasm bo'yicha tavsiya: `ChatRequest.context["image_url"]` → vision model → rang/uslub → `set_context` (proto/kontrakt [!]).
- [ ] 4.6 Katalog indekslari (agar `search_bench` 5 ms dan oshsa) — ADR.
- [x] 4.7 Kubernetes manifestlar (`k8s/agent.yaml`: Deployment, Service ClusterIP, HPA 2–5, PDB), Grafana dashboard JSON (`docs/grafana/`).
- [x] 4.8 Tarix arxivlash (`archived_messages`), 90 kunlik tozalash cron (`session.Cleanup`).
- [ ] 4.9 `ResetSession` bilan birga `ArchiveSession` (manager suhbatni yopganda) — kontrakt [!].
- [x] 4.10 (tekshirildi 2026-09-13: `order_draft(action=quote)` LastQuote/LastCalc + display qaytaradi — PDF Django'da) PDF taklif — Django tomonida; Go faqat `LastQuote` ni `order_draft(quote)` bilan beradi (o'zgarish yo'q, tekshiruv).
- [x] 4.11 (tekshirildi 2026-09-13: Go'da o'zgarish yo'q; katalog snapshot faqat Django→Go) Oflayn/keshlangan katalog — mobil tomonda; Go'da o'zgarish yo'q (tekshiruv).
- [ ] 4.12 `ombor qoldig'i` (v2): `stock_check` tool, skorga `+2`, ogohlantirish — faqat buyurtmachi tasdiqlasa.

---

## Backend bilan ochiq savollar (kelishilmaguncha `[!]`)

| # | Savol | Kimga | Holat |
|---|---|---|---|
| Q1 | P1–P3 proto kengaytmalari (service_prices, tiers, policy, defaults, fason, prompt) — backend `catalog_service.py` ham to'ldiradi | backend | **hal** (kontrakt 1.1, 2026-09-13) |
| Q2 | `go_package` tuzatish (P6) | backend | **hal** |
| Q3 | `ChatEvent.kind="stage"` → `Conversation.stage`; `context["role"]`, `context["customer_mode"]` | backend gateway | **hal**; `x-service-token`/`x-request-id` — backend 6.18 ochiq |
| Q4 | `Usage.cost_*` faqat Django hisoblaydi | backend usage | **hal** (GO-CONTRACT §1.1) |
| Q5 | `FULL_RESYNC` — `from_version < current` bo'lsa darhol, stream yopiladi; Go Snapshot + Watch qayta ochadi | backend | **hal** (GO-CONTRACT §1.2) |
| Q6 | `OrderDraftRequest.items_json` = `calc_cases.json` `input` formati | backend orders | **hal** sxema; `SaveOrderDraft` hali stub (backend 6.17) |
| Q7 | Chiqindi ta'rifi (zapas vs rulon-eni) — buyurtmachi javobi (TZ §12 "aniqlanmagan-1") | buyurtmachi | ochiq |
| Q8 | Tikuv narxi asosi (`per_sqm` hozir) — TZ §12 "aniqlanmagan-3" | buyurtmachi | ochiq |
| Q9 | `calc_cases.json` da ko'p matoli (3–10 bo'lak) holatlar soni ≥ 6 mi? Bo'lmasa qo'shiladi | backend + AI | ochiq |
| Q10 | Karniz (`cornice`) `ServicePrice` kaliti bormi? | backend pricing | ochiq |
| Q11 | Postgres `agent` schema uchun alohida DB user/ruxsat | devops | ochiq |
| Q12 | Ovoz formati (kirish `pcm_s16le 16k`, chiqish `mp3`) — mobil + gateway | backend + mobile | ochiq |

---

## Ish tartibi eslatmasi

1. Task boshlashdan oldin `rules.md` tegishli bo'limi o'qiladi; kelishuv kerak bo'lsa `[!]` — kod stub bilan, test stub xatosini tekshiradi.
2. Har task: kod → test → `make lint test` → `tasks.md` belgisi → commit (`feat(scope): ... (#task)`).
3. Bosqich yakuni: golden + live + integratsiya → teg → `docs/LIVE-RUNS.md`.
4. Har hafta: `[!]` ro'yxati backend bilan ko'rib chiqiladi, yechilganlar `[ ]` ga o'tadi.
