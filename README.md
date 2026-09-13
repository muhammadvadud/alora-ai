# Alora AI Agent (Go)

Alora AI agent (Go) — parda do'koni uchun AI sotuvchi-yordamchi. Loyihaning asosiy maqsadi mijozlar bilan muloqot qilish, ehtiyojlarini aniqlash, katalogni qidirish va hisob-kitob qilishda yordam berishdir.

Tizim ishlashi gRPC protokoli orqali Django gateway va Go agenti o'rtasidagi aloqaga asoslangan. Arxitektura yupqa tool adapterlari va markaziy domen qatlami atrofida qurilgan bo'lib, qat'iy ma'lumotlar sinxronizatsiyasi Django backend bilan ta'minlanadi.

## Arxitektura
Tizim arxitekturasi: **Django gateway ↔ Go agent gRPC**

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
| `OPENAI_API_KEY` | OpenAI API kaliti (zaxira va Whisper uchun) |
| `ELEVENLABS_API_KEY` | ElevenLabs TTS uchun API kaliti |
| `GOOGLE_API_KEY` | Google TTS/STT zaxirasi uchun API kaliti |
| `AGENT_SERVICE_TOKEN` | gRPC ulanishlarini xavfsizligini ta'minlovchi service token |
| `DATABASE_URL` | PostgreSQL ulanishi uchun URL manzili |
| `ALORA_BACKEND_ADDR` | Django backend serverining gRPC manzili |

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
- `make run-cli` - CLI rejimida ishga tushirish.
- `make run-grpc` - gRPC rejimida ishga tushirish.

## Testlash
Loyiha bir necha bosqichli testlashga ega:
- **Etalon `calc_cases.json`:** Backend bilan bir xil narx va hisob-kitob mexanizmi qo'llanilishi uchun umumiy etalon testlar o'tkaziladi.
- **Golden dialoglar:** 20 ta yozib olingan etalon ssenariylar mock LLM yordamida sinovdan o'tkazilib, tool ketma-ketligi va holat (stage) to'g'ri ishlashini kafolatlaydi. Hallucination testlari alohida olib boriladi.

## Hujjatlar Havolasi
- [Ishlab chiqish qoidalari (rules.md)](rules.md)
- [Ish rejasi (tasks.md)](tasks.md)
- [Backend bilan shartnoma (GO-CONTRACT.md)](../backend/docs/GO-CONTRACT.md)
