# AGENTS.md — bu blueprint bilan ishlash qoidalari

> Bu fayl AI kod-agentlari (va odamlar) uchun. Kodni o'zgartirishdan **oldin** o'qing.
> Maqsad: shu asosdan yangi AI agent qurishda arxitekturani buzmaslik.
> Umumiy ishlab chiqish qoidalari uchun [rules.md](rules.md) (xususan §0 Qat'iy qonun) ga qarang.
> Loyiha ish rejasi bilan tanishish uchun [tasks.md](tasks.md) ni ko'ring.

## Bu nima

Go + [Eino](https://github.com/cloudwego/eino) da **tool-chaqiruvchi ReAct AI agent** blueprinti.
Alora AI agenti (Go) uchun moslashtirilgan.
Pluggable LLM provider, CLI + HTTP (SSE) + gRPC transport, session xotira, streaming tool chaqiruvlari.

Yangi agent = shu repodan nusxa → config o'zgartirish → tool qo'shish. **Yadroga tegmang.**

## Oltin qoidalar (buzilmasin)

1. **API kalit hech qachon kodda yoki `config.yaml` da bo'lmaydi.** Faqat ENV'dan
   (`config.resolveAPIKey`, provider→ENV xaritasi `providerKeyEnv`). Yangi provider — yangi ENV.
2. **Barcha transport bitta manbadan render qiladi:** `agent.Chat.Stream(ctx, sessionID, input) <-chan Event`.
   Yangi UI/transport yozsangiz — shu Event oqimini render qiling, agentga to'g'ridan-to'g'ri tegmang.
3. **Tool = sof funksiya + struct.** Biznes mantiq `internal/curtains/` yoki `internal/tools/` da; LLM'ga bog'liq emas.
   Struct maydonlarida `jsonschema` teglari **majburiy** (Eino shu orqali schema yasaydi).
4. **Provider qo'shish = registry'ga fayl.** `internal/llm/<nom>.go` da `init()` orqali
   `register(...)`. Yadro fayllar (`llm.go`, `agent.go`, `chat.go`) o'zgarmaydi.
5. **Xatolar aniq qaytariladi**, jim yutilmaydi. Tashqi kirish tekshiriladi (masalan bo'sh shahar).
6. **O'zgartirishdan keyin doim:** `go build ./... && go vet ./... && golangci-lint run && go test -race ./...` + qo'lda sinov.
7. **Kod izohlari — NEGA, WHAT emas.** Mavjud uslubga mos (o'zbekcha izohlar, `gofmt`).

## Arxitektura xaritasi

```
alora-agent/
├── cmd/agent/main.go              # composition root: config→llm→catalog→tools→agent→transport
├── configs/config.yaml            # fallback sozlamalar; kalitlar YO'Q
├── proto/alora.proto              # backend repodan NUSXA (export_proto.sh); qo'lda tahrirlanmaydi
├── gen/alorav1/                   # buf generate natijasi (*.pb.go, *_grpc.pb.go); qo'lda tahrirlanmaydi
├── buf.yaml · buf.gen.yaml        # generatsiya konfiguratsiyasi
├── internal/
│   ├── config/                    # Load(), providerKeyEnv; grpc/backend/catalog/stt/tts bo'limlari
│   ├── llm/                       # providerlar + usage helpers
│   ├── agent/                     # ReAct sikli (chat.go), Event turlari, history.go
│   ├── session/                   # Store interfeysi + Postgres / Memory holat saqlash
│   ├── curtains/                  # ★ DOMEN — LLM, gRPC, Eino'dan MUSTAQIL (katalog, narx hisobi)
│   ├── tools/                     # ★ yupqa adapterlar (registratsiya main.go da)
│   ├── prompt/                    # system prompt yig'uvchi (Django prompt + holat + bilimlar)
│   ├── backend/                   # Go→Django gRPC klient + outbox navbati
│   ├── voice/                     # STT/TTS interfeyslar + registry
│   ├── observability/             # slog sozlash, Prometheus metrikalar
│   └── transport/
│       ├── grpc/                  # ★ asosiy: Agent server (Chat, VoiceChat)
│       ├── http/                  # /healthz, /metrics
│       └── cli/                   # dev: terminalda sinash
```

### Ma'lumot oqimi (bir so'rov)

```
foydalanuvchi kirishi
      │
      ▼
transport (grpc/cli/http) ──► Chat.Stream(ctx, sessionID, input) ──► <-chan Event
                              │
                              ├─ session.WithID(ctx, sessionID)         (tool'lar sessiyani biladi)
                              ├─ session.Store.History(ctx, sessionID)  (tarixni yuklaydi)
                              ├─ react.Agent.Stream(...) + callbacks
                              │     ├─ ChatModel oqimi ─► tool_call (nom) + tool_args
                              │     ├─ Tool node       ─► tool_result
                              │     └─ yakuniy javob   ─► content
                              └─ tugagach: session.Store.Append(ctx, user, assistant)
```

## Event kontrakti (MUHIM)

`Chat.Stream` qaytaradigan Event ketma-ketligi (`internal/agent/event.go`):

| Kind | Qachon | Maydonlar |
|------|--------|-----------|
| `start` | so'rov boshlanishi bilan darhol | — |
| `tool_call` | model tool **nomini** oqizganda (args'dan oldin) | `tool` |
| `tool_args` | tool **argumentlari** to'lgach | `tool`, `args` |
| `tool_result` | tool bajarilgach | `tool`, `result` |
| `content` | javob matni bo'lagi (token-ba-token) | `content` |
| `done` | javob tugadi | — |
| `error` | xato | `error` |

Umumiy tartib: `start → (tool_call → tool_args → tool_result → products?)* → content* → stage → done`.
Parallel tool'larda `tool_call`/`tool_args`/`tool_result` `tool` nomi bo'yicha juftlanadi (tartibga tayanmang).

## Retsept: yangi tool qo'shish

Tool ikki qatlam: **domen paketi** (`internal/curtains/` — tashqi API/mantiq, LLM'dan mustaqil)
va **yupqa adapter** (`internal/tools/<feature>.go` — DTO validatsiya + domen chaqiruvi).
Registratsiya `tools.go` da (`Alora()` yoki `AloraLookups()`).

Misol: **Alora product_search tooli**

1) Domen paketi `internal/curtains/search.go`:
```go
package curtains

type SearchQuery struct {
    Room string
    // Boshqa filtrlar...
}

func Search(v *View, q SearchQuery) []ProductCard {
    // Katalog orqali qidirish mantiqi
}
```

2) DTO'lar `internal/tools/dto.go` (jsonschema teglari MAJBURIY, `error` maydoni bilan):
```go
type productSearchArgs struct {
	Room string `json:"room,omitempty" jsonschema:"description=Qaysi xona uchun parda kerak"`
}
type productSearchResult struct {
	Products []productCard `json:"products,omitempty"`
	Error    string        `json:"error,omitempty"` // errors-as-results
}
```

3) Adapter `internal/tools/product_search.go` — konstruktor `Deps` oladi:
```go
package tools

import (
	"context"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

func ProductSearch(d Deps) tool.BaseTool {
	h := &productSearchHandler{d: d}
	return must(utils.InferTool("product_search", "Katalogdan qidirish", h.do))
}

type productSearchHandler struct{ d Deps }

func (h *productSearchHandler) do(ctx context.Context, a productSearchArgs) (productSearchResult, error) {
	v, errMsg, stale := readyView(h.d)
	if errMsg != "" {
		return productSearchResult{Error: errMsg}, nil
	}
	
	// Domen chaqiruvi
	cards := curtains.Search(v, curtains.SearchQuery{Room: a.Room})
	
	// State'ga yozish
	st, sid := loadState(ctx, h.d)
	st.Apply("product_search", argsMap(a), now(h.d))
	saveState(ctx, h.d, sid, st)
	
	return productSearchResult{Products: cards}, nil // Go error EMAS
}
```

4) `internal/tools/tools.go` dagi `Alora()` (yoki `AloraLookups()`) ro'yxatiga qo'shing:
```go
func Alora(d Deps) []tool.BaseTool {
	return []tool.BaseTool{
		ProductSearch(d),
		// ...
	}
}
```
Yangi qo'shilgan tool nomlari albatta `alora_test.go` da (TestAlora_ConstructorsAndNames) qayd etilgan bo'lishi kerak.
**Tamom.**

**Errors-as-results (MUHIM):** foydalanuvchi kiritmasi/tashqi API xatosida tool `error`
maydonli natija qaytaradi (`productSearchResult{Error: ...}, nil`), Go `error` EMAS — ReAct agent so'rovni
yiqitmasdan o'zini tuzatadi. Chinakam dasturlash xatosi (masalan `InferTool` ta'rifi) — `must` panic.

**Sessiya:** tool `session.IDFromContext(ctx)` orqali joriy sessiya ID'sini oladi
(`Chat.Stream` `session.WithID` bilan inyeksiya qiladi) — masalan sessiya bo'yicha holat saqlash uchun.

## Retsept: yangi LLM provider qo'shish

`internal/llm/<nom>.go`:

```go
package llm

import (
	"context"
	"github.com/cloudwego/eino-ext/components/model/<nom>"
	"github.com/cloudwego/eino/components/model"
	"github.com/muhammadvadud/alora-agent/internal/config"
)

func init() { register("<nom>", build<Nom>) }

func build<Nom>(ctx context.Context, cfg config.LLMConfig) (model.ToolCallingChatModel, error) {
	return <nom>.NewChatModel(ctx, &<nom>.ChatModelConfig{ /* cfg dan maydonlar */ })
}
```

So'ng `internal/config/config.go` dagi `providerKeyEnv` ga kalit ENV nomini qo'shing
(kalitsiz provider uchun `""`). Keraksizni o'chirish: faylni o'chiring + `go mod tidy`.

**Diqqat — provider config'lari har xil:**
- `deepseek`/`openai`: `ChatModelConfig`; openai'da `MaxTokens *int`, `Temperature *float32` (pointer).
- `claude`: `Config`; `MaxTokens int` **majburiy**, `BaseURL *string`, `Temperature *float32`.
- `ollama`: `ChatModelConfig`; **kalit yo'q**, `BaseURL` + `Model`.

## Retsept: xotira (Store) almashtirish

`session.Store` — endi `History`/`Append`/`Reset` metodlari `ctx context.Context` va `error`
qaytaradi (DB implementatsiyalari uchun; in-memory ularni e'tiborsiz qoldiradi). Tayyor:
`NewMemory()` (in-memory) va `NewPostgres(ctx, dsn)` (PostgreSQL, JSONB). `main.go` `newStore`
`DATABASE_URL` bo'lsa Postgres, aks holda in-memory tanlaydi (`docker compose up -d` bilan DB koʻtariladi).
Yangi backend (masalan Redis) — `Store` ni implement qiling va `newStore` ga uling.

## Retsept: yangi transport qo'shish

`Chat.Stream(ctx, sessionID, input) <-chan Event` ni chaqirib, Event'larni o'z formatingizga
render qiling (`transport/cli`, `transport/grpc` yoki `transport/http` ni namuna oling).
`main.go` `switch cfg.Mode` ga yangi `case` qo'shing.

## Kritik nozikliklar (bularni noto'g'ri qilsangiz sinadi)

1. **Qo'lda yozilgan ReAct sikli (`agent/chat.go`)** — Eino'ning react agenti ishlatilmaydi.
   Sababi: uning `StreamToolCallChecker`'i tool'ni ishonchli aniqlash uchun butun oqimni
   o'qishga majbur (DeepSeek matnni tool_calls'dan **oldin** yuboradi), bu esa javobni
   buferlaydi — streaming yo'qoladi. "Birinchi chunk'da tekshirish" esa preambula bo'lsa
   tool'ni butunlay o'tkazib yuboradi. Sikl ikkalasini ham beradi — **react agentini qaytarmang**.
2. **Tarix invarianti (ikki yarim)** — `Chat.Stream` tarixga BUTUN navbatni yozadi: user +
   assistant (tool_calls bilan) + tool natijalari. Faqat yakuniy matnni saqlash xato edi —
   model o'zining "bajardim" degan gapini ko'rib, tool'ni qayta chaqirmasdan natijani to'qib
   chiqarardi. `trimHistory` (`agent/history.go`) esa o'qishda shu yaxlitlikni himoyalaydi:
   kesim **faqat User xabaridan** boshlanadi, aks holda tool natijasi juftsiz qolib provayder
   400 qaytaradi. **Ikkalasini birga o'zgartiring** — biri buzilsa ikkinchisi ma'nosiz.
3. **`emit()` va ctx** — `out` kanalga yozish ctx bekor bo'lsa bloklanmaydi (iste'molchi
   ketgan bo'lishi mumkin). Kanal `Stream` goroutine'i qaytganda `defer close(out)` bilan
   yopiladi — boshqa joydan yozmang.
4. **Reasoning provider'ga xos** — o'ylash matni `message.Extra` da keladi; uni o'qish
   `llm.Reasoning()` da turadi, `agent` qatlami provider'ni bilmaydi. `reasoning_effort` esa
   `WithExtraFields` orqali so'rov darajasida ketadi, shuning uchun model `modelWithOptions`
   bilan o'raladi — **`WithTools` o'ramni saqlashi shart**, aks holda daraja tool bog'langach
   jimgina yo'qoladi (agent doim tool bilan ishlaydi → hech qachon ketmasdi).
5. **API kalit** — `config.resolveAPIKey` provider'ga qarab ENV'dan oladi; ollama'dan boshqada
   bo'sh bo'lsa xato. Kalitni config yoki kodga yozmang.

## Verifikatsiya (tayyor deyishdan oldin)

```bash
go build ./... && go vet ./... && go test ./...     # toza bo'lishi shart
make run-cli                         # CLI rejimida ishga tushirish (dev rejim)
make run-grpc                        # gRPC rejimida ishga tushirish (prod rejim)
```

## Qilmang (anti-patternlar)

- ❌ Tool ichida LLM'ga to'g'ridan-to'g'ri murojaat / global holat.
- ❌ Transport ichida biznes mantiq — u faqat Event render qiladi.
- ❌ `config.yaml` ga API kalit yozish.
- ❌ Barcha tool'ni bitta ulkan faylga tiqish — modul = fayl.
- ❌ Qo'lda yozilgan ReAct siklini Eino react agentiga qaytarish — streaming yoki tool sinadi.
- ❌ Tarixga faqat yakuniy javobni yozish — model natijani to'qib chiqara boshlaydi.
- ❌ `trimHistory` ni User chegarasiga qaramasdan kesish — provayder 400 beradi.
- ❌ Event `tool`/`content` maydonlarini transportda o'zgartirish — kontrakt buziladi.
