# Alora AI Agent (Go)

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
- `order_draft`: Buyurtma qoralamasini saqlash
- `fason_lookup`: Xona/uslubga tayyor fason shablon topish
- `knowledge_lookup`: Do'kon bilimlar bazasidan qoidalar olish

## Ishga tushirish
Talablar:
- Go 1.26+

Ishga tushirish jarayoni:
1. `make install-tools` buyrug'ini ishga tushiring (kerakli generatsiya va lint vositalari).
2. `.env.example` faylidan `.env` nusxasini yarating va kalitlarni kiriting.
3. CLI rejimida sinab ko'rish uchun: `make run-cli`
4. gRPC serverni ishga tushirish uchun: `make run-grpc`
5. Docker compose orqali ishga tushirish uchun: `docker compose up agent -d`

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
- **Unit testlar:** Domen qismi (`go test -race ./...`)
- **Golden dialoglar:** 20 ta etalon ssenariylar mock LLM yordamida sinaladi (`go test ./tests/golden/...`).
- **Live testlar:** Real Django backend serveriga bog'langan holda sinovdan o'tkazish (`ALORA_LIVE_BACKEND=127.0.0.1:50051 AGENT_SERVICE_TOKEN=secret go test -tags live ./tests/live/`).
- **Load testlar:** Tizimning yuklamalarga chidamliligini tekshirish (`go test -tags load ./tests/load/`).

## Hujjatlar Havolasi
- [Ishlab chiqish qoidalari (rules.md)](rules.md)
- [Ish rejasi (tasks.md)](tasks.md)
- [Backend bilan shartnoma (GO-CONTRACT.md)](../backend/docs/GO-CONTRACT.md)
