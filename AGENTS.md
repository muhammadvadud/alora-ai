# AGENTS.md — bu blueprint bilan ishlash qoidalari

> Bu fayl AI kod-agentlari (va odamlar) uchun. Kodni o'zgartirishdan **oldin** o'qing.
> Maqsad: shu asosdan yangi AI agent qurishda arxitekturani buzmaslik.

## Bu nima

Go + [Eino](https://github.com/cloudwego/eino) da **tool-chaqiruvchi ReAct AI agent** blueprinti.
Pluggable LLM provider, CLI + HTTP (SSE) transport, session xotira, streaming tool chaqiruvlari.

Yangi agent = shu repodan nusxa → config o'zgartirish → tool qo'shish. **Yadroga tegmang.**

## Oltin qoidalar (buzilmasin)

1. **API kalit hech qachon kodda yoki `config.yaml` da bo'lmaydi.** Faqat ENV'dan
   (`config.resolveAPIKey`, provider→ENV xaritasi `providerKeyEnv`). Yangi provider — yangi ENV.
2. **Barcha transport bitta manbadan render qiladi:** `agent.Chat.Stream(ctx, sessionID, input) <-chan Event`.
   Yangi UI/transport yozsangiz — shu Event oqimini render qiling, agentga to'g'ridan-to'g'ri tegmang.
3. **Tool = sof funksiya + struct.** Biznes mantiq `internal/tools/` da; LLM'ga bog'liq emas.
   Struct maydonlarida `jsonschema` teglari **majburiy** (Eino shu orqali schema yasaydi).
4. **Provider qo'shish = registry'ga fayl.** `internal/llm/<nom>.go` da `init()` orqali
   `register(...)`. Yadro fayllar (`llm.go`, `agent.go`, `chat.go`) o'zgarmaydi.
5. **Xatolar aniq qaytariladi**, jim yutilmaydi. Tashqi kirish tekshiriladi (masalan bo'sh shahar).
6. **O'zgartirishdan keyin doim:** `go build ./... && go vet ./... && go test ./...` + qo'lda sinov.
7. **Kod izohlari — NEGA, WHAT emas.** Mavjud uslubga mos (o'zbekcha izohlar, `gofmt`).

## Arxitektura xaritasi

```
cmd/agent/main.go        # wiring (DI): config→llm→tools→agent→chat→(cli|http|telegram). tool'lar shu yerda OCHIQ register qilinadi
configs/config.yaml      # provider, model, server, telegram, db, mode, log (Viper; ENV ustidan yozadi)
internal/
├── config/              # config.Load(path) -> *Config; providerKeyEnv (provider→ENV kaliti)
├── llm/                 # llm.New(ctx,cfg) -> model.ToolCallingChatModel; registry (har provider 1 fayl)
├── agent/               # agent.New(...)=ReAct; Chat.Stream=Event oqimi; Event/Kind turlari
├── session/             # Store interfeysi (ctx+error) + NewMemory (in-memory) / NewPostgres (DB)
├── weather/             # domen-feature namunasi: tashqi API mijozi + domen turi (tools'dan alohida)
├── tools/               # yupqa adapter qatlami: dto.go + <feature>.go konstruktorlar (registratsiya main.go da)
└── transport/
    ├── cli/             # cli.Run(ctx, chat) — Event → stdout
    ├── http/            # http.Serve(ctx, chat, addr) — /chat, /chat/stream (SSE), /healthz
    └── telegram/        # telegram.Serve(ctx, chat, token) — long-polling bot; har chat = sessiya
```

### Ma'lumot oqimi (bir so'rov)

```
foydalanuvchi kirishi
      │
      ▼
transport (cli/http/telegram) ──► Chat.Stream(ctx, sessionID, input) ──► <-chan Event
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

Umumiy tartib: `start → (tool_call → tool_args → tool_result)* → content* → done`.
Parallel tool'larda `tool_call`/`tool_args`/`tool_result` `tool` nomi bo'yicha juftlanadi (tartibga tayanmang).

## Retsept: yangi tool qo'shish

Tool ikki qatlam: **domen paketi** (`internal/<feature>/` — tashqi API/mantiq, LLM'dan mustaqil)
va **yupqa adapter** (`internal/tools/<feature>.go` — DTO validatsiya + domen chaqiruvi).
Registratsiya `main.go` da (composition root) — **ochiq**, `modules` ro'yxati YO'Q.

**Trivial tool** (tashqi API kerak emas, masalan `utils.go` dagi calculator/clock):
handler funksiya + DTO'ni `dto.go` ga + `Xxx() tool.BaseTool` konstruktor (`must` bilan).

**Domen-feature tool** (tashqi API bilan, `weather` namunasidek):

1) Domen paketi `internal/foo/`:
```go
package foo

type Result struct { /* domen turi, json teglari bilan */ }

type Client struct { /* http.Client ... */ }
func NewClient() *Client { /* ... */ }
func (c *Client) Do(ctx context.Context, q string) (Result, error) { /* mantiq */ }
```

2) DTO'lar `internal/tools/dto.go` (jsonschema teglari MAJBURIY, `error` maydoni bilan):
```go
type fooArgs struct {
	Query string `json:"query" jsonschema:"description=nima qidirilsin"`
}
type fooResult struct {
	Value string `json:"value,omitempty"`
	Error string `json:"error,omitempty"` // errors-as-results
}
```

3) Adapter `internal/tools/foo.go` — konstruktor `Deps` orqali domen mijozini oladi:
```go
func Foo(d Deps) []tool.BaseTool {
	h := &fooHandler{c: d.Foo}
	return []tool.BaseTool{must(utils.InferTool("foo", "Foo qiladi.", h.do))}
}

type fooHandler struct{ c *foo.Client }

func (h *fooHandler) do(ctx context.Context, a fooArgs) (fooResult, error) {
	if strings.TrimSpace(a.Query) == "" {
		return fooResult{Error: "query bo'sh"}, nil // Go error EMAS — natijada
	}
	r, err := h.c.Do(ctx, a.Query)
	if err != nil {
		return fooResult{Error: err.Error()}, nil
	}
	return fooResult{Value: r.Value}, nil
}
```
`tools.Deps` ga `Foo *foo.Client` maydonini qo'shing.

4) `main.go` da OCHIQ register qiling:
```go
deps := tools.Deps{Weather: weatherClient, Foo: fooClient}
toolList = append(toolList, tools.Foo(deps)...)
```
**Tamom.**

**Errors-as-results (MUHIM):** foydalanuvchi kiritmasi/tashqi API xatosida tool `error`
maydonli natija qaytaradi (`fooResult{Error: ...}, nil`), Go `error` EMAS — ReAct agent so'rovni
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
	"baholash-chatbot/internal/config"
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
render qiling (`transport/cli`, `transport/http` yoki `transport/telegram` ni namuna oling).
`main.go` `switch cfg.Mode` ga yangi `case` qo'shing. Telegram namunasi: `-mode telegram`,
token `TELEGRAM_BOT_TOKEN` ENV'dan (har chat ID alohida sessiya).

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
go run ./cmd/agent -mode cli                         # tool sinash (calculator/clock/weather)
go run ./cmd/agent -mode http                        # so'ng: curl localhost:8080/healthz
go run ./cmd/agent -mode telegram                    # TELEGRAM_BOT_TOKEN kerak
```
SSE tekshiruvi: `curl -N -XPOST localhost:8080/chat/stream -d '{"message":"..."}'`.

## Qilmang (anti-patternlar)

- ❌ Tool ichida LLM'ga to'g'ridan-to'g'ri murojaat / global holat.
- ❌ Transport ichida biznes mantiq — u faqat Event render qiladi.
- ❌ `config.yaml` ga API kalit yozish.
- ❌ Barcha tool'ni bitta ulkan faylga tiqish — modul = fayl.
- ❌ Qo'lda yozilgan ReAct siklini Eino react agentiga qaytarish — streaming yoki tool sinadi.
- ❌ Tarixga faqat yakuniy javobni yozish — model natijani to'qib chiqara boshlaydi.
- ❌ `trimHistory` ni User chegarasiga qaramasdan kesish — provayder 400 beradi.
- ❌ Event `tool`/`content` maydonlarini transportda o'zgartirish — kontrakt buziladi.
