# AI Agent Blueprint (Go + Eino)

Go'da tool-chaqiruvchi **ReAct AI agent** qurish uchun qayta ishlatiladigan asos.
Yangi agent kerak bo'lsa: shu repodan nusxa oling, config'ni o'zgartiring, tool qo'shing — tayyor.

## Imkoniyatlar

- **Pluggable LLM provider** — DeepSeek / OpenAI / Claude / Ollama, config orqali almashadi
- **Ikki transport** — interaktiv **CLI** va **HTTP API** (SSE streaming)
- **Session xotira** — `Store` interfeysi + in-memory (keyin Redis/DB)
- **Tool'lar** — bitta modulda bir nechta tool; real tashqi API namunasi (Open-Meteo)
- **Streaming** — javob va tool chaqiruvlari real vaqtda. Event ketma-ketligi:
  `start` → `tool_call` (tool nomi) → `tool_args` (params) → `tool_result` → `content…` → `done`.
  Tool nomi model oqimidan paydo bo'lishi bilanoq keladi, argumentlar to'lishini kutmasdan.

## Tuzilma

```
cmd/agent/main.go            # wiring: config→llm→tools→agent→(cli|http)
configs/config.yaml          # provider, model, server, mode, log
internal/
├── config/                  # Viper: provider'ga qarab API-kalit ENV'ini yechadi
├── llm/                     # PLUGGABLE PROVIDER (registry; har provider 1 fayl)
├── agent/                   # ReAct agent + delivery-agnostik Event oqimi (chat.go)
├── session/                 # Store interfeysi + in-memory xotira
├── tools/                   # tool modullari (utils.go = 2 tool, weather.go = real API)
└── transport/
    ├── cli/                 # REPL: Event → stdout
    └── http/                # net/http: /chat, /chat/stream (SSE), /healthz
```

Barcha transport bitta manbadan — `agent.Chat.Stream(ctx, sessionID, input) <-chan Event` — render qiladi.

## Ishga tushirish

```bash
cp .env.example .env        # provider'ga mos kalit (masalan DEEPSEEK_API_KEY)

make cli                    # interaktiv terminal
make http                   # HTTP server (:8080)
make test                   # testlar
make dev                    # hot reload (avval: make install-tools)
```

CLI'da tool chaqiruvi real vaqtda ko'rinadi:
```
Siz: Toshkent obhavosi va 25 ni 4 ga bo'l?
  ⚙ get_weather({"city": "Toshkent"})
  ⚙ calculator({"op": "div", "a": 25, "b": 4})
  ↳ 6.25
  ↳ {"city":"Tashkent, Uzbekistan","temperature_c":38.5,"condition":"ochiq havo"}
Agent: Toshkentda 38.5°C, ochiq havo. 25 / 4 = 6.25.
```

## HTTP API

```bash
curl localhost:8080/healthz

# JSON javob (tool tafsilotlari bilan)
curl -XPOST localhost:8080/chat -d '{"session_id":"u1","message":"Toshkent obhavosi?"}'

# SSE streaming (content + tool_call/tool_result event'lari)
curl -N -XPOST localhost:8080/chat/stream -d '{"session_id":"u1","message":"100 ni 7 ga bo'l"}'
```

Bir xil `session_id` bilan yuborilgan xabarlar suhbat tarixini saqlaydi. `session_id` bo'sh bo'lsa avtomatik generatsiya qilinadi.

## Konfiguratsiya

`configs/config.yaml` — provider, model, agent, server, mode. Har qiymat muhit
o'zgaruvchisi bilan ustidan yoziladi: `MODE`, `LLM_MODEL`, `LLM_PROVIDER`,
`SERVER_ADDR`, `LOG_LEVEL`. `-mode cli|http` flag'i config'dagi mode'ni ustidan yozadi.

**Provider almashtirish** — `llm.provider` ni o'zgartiring va mos kalitni bering:

| provider | model misoli | kalit ENV |
|----------|--------------|-----------|
| deepseek | `deepseek-v4-pro` | `DEEPSEEK_API_KEY` |
| openai | `gpt-4o` | `OPENAI_API_KEY` |
| claude | `claude-sonnet-4` | `ANTHROPIC_API_KEY` |
| ollama | `llama3.1` | — (lokal) |

## Yangi tool qo'shish

Mavjud modulga (`internal/tools/utils.go`) funksiya + `InferTool` qatorini qo'shing,
yoki yangi modul yarating:

1. `internal/tools/dto.go`: kirish/chiqish struct'lari (jsonschema teglari bilan).
2. `internal/tools/<nom>.go`: handler funksiya va konstruktor —
   `func <Nom>(d Deps) []tool.BaseTool` (tashqi bog'liqlik kerak bo'lsa `Deps` orqali),
   yoki bog'liqliksiz bo'lsa `func <Nom>() tool.BaseTool`.
3. `cmd/agent/main.go` da (composition root) OCHIQ register qiling:
   `toolList = append(toolList, tools.<Nom>(deps)...)`.
   Aggregator/`modules` ro'yxati YO'Q — kim nima qila olishi bir joyda ko'rinadi.

## Yangi provider qo'shish

1. `internal/llm/<nom>.go`: `func init() { register("<nom>", build<Nom>) }` va
   `build<Nom>(ctx, cfg) (model.ToolCallingChatModel, error)` — mos eino-ext paketini chaqiradi.
2. `config/config.go` dagi `providerKeyEnv` ga kalit ENV nomini qo'shing.

Keraksiz provayderni o'chirish: `internal/llm/<nom>.go` ni o'chiring + `go mod tidy`.

## Docker

```bash
make docker
docker run -p 8080:8080 -e DEEPSEEK_API_KEY=sk-... baholash-agent
```

## Kengaytirish / hissa qo'shish

Kodni o'zgartirishdan oldin **[AGENTS.md](./AGENTS.md)** ni o'qing — u yerda arxitektura
qoidalari, Event kontrakti, retseptlar (tool/provider/transport/xotira qo'shish) va kritik
nozikliklar bor. AI kod-agentlari uchun ham shu fayl mo'ljallangan.

## Stek

Eino · DeepSeek/OpenAI/Claude/Ollama · Viper · godotenv · stdlib `net/http` + `slog`
