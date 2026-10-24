# Alora AI Agent (Go)

[![Version](https://img.shields.io/badge/version-v0.1.0-blue.svg)](CHANGELOG.md)

**Modul yo'li:** `github.com/muhammadvadud/alora-ai`

Alora AI agent (Go) — parda do'koni uchun AI sotuvchi-yordamchi. Loyihaning asosiy maqsadi mijozlar bilan muloqot qilish, ehtiyojlarini aniqlash, katalogni qidirish va hisob-kitob qilishda yordam berishdir.

Tizim ishlashi gRPC protokoli orqali Django gateway va Go agenti o'rtasidagi aloqaga asoslangan. Arxitektura yupqa tool adapterlari va markaziy domen qatlami atrofida qurilgan bo'lib, qat'iy ma'lumotlar sinxronizatsiyasi Django backend bilan ta'minlanadi.

## Arxitektura
Tizim arxitekturasi: **Django gateway ↔ Go agent gRPC**

## Tool'lar
Agent foydalanadigan asosiy tool'lar:
- `product_search`: Katalogdan mahsulotlarni qidirish
- `set_context`: Foydalanuvchi talablarini aniqlab, state'ga yozish
- `curtain_calculate`: Pardaning xomaki hisob-kitobi
- `price_quote`: Xizmat va chegirmalar bilan yakuniy narx
- `build_options`: Uch xil variantni shakllantirish (arzon, o'rta, premium)
- `order_draft`: Yakuniy taklif/qoralamani beradi (mijoz ma'lumotlari shu yerda yig'ilmaydi — ADR-020)
- `fason_lookup`: Xona/uslubga tayyor fason shablon topish
- `knowledge_lookup`: Do'kon bilimlar bazasidan qoidalar olish

## Tez boshlash
Talablar:
- Go 1.26+

Ishga tushirish jarayoni:
1. `make install-tools` buyrug'ini ishga tushiring (kerakli generatsiya va lint vositalari).
2. `.env.example` faylidan `.env` nusxasini yarating.
3. **Kalitsiz sinov**: `provider: mock` sozlamasidan foydalaning (kalitlar talab qilinmaydi).
4. **Real provider bilan**: `.env` ichida `DEEPSEEK_API_KEY` kabi kalitlarni kiriting.
5. CLI rejimida sinab ko'rish uchun: `make run-cli`
6. gRPC serverni ishga tushirish uchun: `make run-grpc` (yoki Docker orqali: `docker compose up agent -d`)
7. Qo'lda (manual) live sinov o'tkazish uchun `cmd/livechat` dan foydalaning.

## ENV Kalitlar Jadvali

| Kalit | Tavsifi |
| --- | --- |
| `DEEPSEEK_API_KEY` | DeepSeek LLM uchun API kaliti |
| `ANTHROPIC_API_KEY` | Anthropic (Claude) uchun API kaliti |
| `OPENAI_API_KEY` | OpenAI API kaliti (Whisper STT va zaxira LLM; provider'lar hali ro'yxatga kirmagan, 3.2) |
| `ELEVENLABS_API_KEY` | ElevenLabs TTS uchun API kaliti (hali ro'yxatga kirmagan, 3.3) |
| `GOOGLE_API_KEY` | Google TTS/STT zaxirasi uchun API kaliti (hali ro'yxatga kirmagan, 3.2/3.3) |
| `AGENT_SERVICE_TOKEN` | gRPC ulanishlarini xavfsizligini ta'minlovchi service token |
| `DATABASE_URL` | PostgreSQL ulanishi uchun URL manzili |
| `ALORA_BACKEND_ADDR` | Django backend serverining gRPC manzili |
| `ALORA_LIVE_BACKEND` | Real Django ulanishi manzili (faqat live testlar uchun, masalan 127.0.0.1:50051) |
| `ALORA_LIVE_AGENT` | Real Live Agent ulanish manzili (live testlar uchun) |

## Make Buyruqlari
- `make build` - Loyihani build qilish.
- `make test` - Testlarni ishga tushirish.
- `make vet` - Go vet analizatoridan o'tkazish.
- `make lint` - Linterlarni ishga tushirish.
- `make fmt` - Kodni formatlash.
- `make tidy` - Go modullar ro'yxatini tozalash va yangilash.
- `make proto` - Proto fayllarni generatsiya qilish.
- `make check-fixtures` - Fixture'lar sinxronligini tekshirish.
- `make sync-fixtures` - Fixture'larni backenddan tortib olish.
- `make bench` - Benchmark'larni yurgizish.
- `make docker` - Docker image build qilish.
- `make run-cli` - CLI rejimida ishga tushirish.
- `make run-grpc` - gRPC rejimida ishga tushirish.
- `make install-tools` - Kerakli generatsiya vositalarini o'rnatish.

## Testlash
Loyiha bir necha bosqichli testlashga ega:
- **Unit testlar:** Domen, utilitalar (`go test -race ./...`).
- **Golden dialoglar:** 20 ta etalon ssenariylar mock LLM yordamida sinaladi (`go test ./tests/golden/...`).
- **Live testlar:** Real LLM javoblari bilan sinaladi (`ALORA_LIVE_BACKEND=127.0.0.1:50051 ALORA_LIVE_AGENT=127.0.0.1:9090 AGENT_SERVICE_TOKEN=secret go test -tags live ./tests/live/`).
- **E2E testlar:** Tizimning uchdan-uchiga sinovi (`go test ./tests/e2e/...`).
- **Load testlar:** Tizimning yuklamalarga chidamliligini tekshirish (`go test -tags load ./tests/load/`).

## Hujjatlar Havolasi
- [Ishlab chiqish qoidalari (rules.md)](rules.md)
- [Ish rejasi (tasks.md)](tasks.md)
- [Backend bilan shartnoma (GO-CONTRACT.md)](../backend/docs/GO-CONTRACT.md)
