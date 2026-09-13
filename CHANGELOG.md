# Changelog

Barcha muhim o'zgarishlar ushbu faylda yozib boriladi.

Format [Keep a Changelog](https://keepachangelog.com/uz/1.1.0/) ga asoslangan.

## [v0.1.0] - 2026-09-14

### Qo'shildi
- Asosiy tool'lar: `product_search`, `set_context`, `curtain_calculate`, `price_quote`, `build_options`, `order_draft`, `fason_lookup`, `knowledge_lookup`.
- gRPC transport va Django backend bilan aloqa.
- VoiceChat oqimi (STT, agent, jumla-jumla TTS, barge-in).
- Asinxron xabarlar yuborish uchun outbox pattern (usage va loglar uchun).
- Sessiya va state'ni saqlash uchun Postgres ma'lumotlar bazasi (ichki migrator bilan).
- Xavfsizlik va tarmoq: mTLS, Kubernetes manifestlar (Deployment, Service, HPA, PDB) va Grafana dashboard.
- Testlar: 20 ta golden etalon dialog, live, e2e, yuklama (load) testlari.
- API kalitsiz tekshirish uchun mock provider.
- LLM provayderida uzilish bo'lsa fallback ishlash imkoniyati (retry -> fallback).

### Xavfsizlik
- Audit bo'yicha tuzatishlar kiritildi:
  - ResetSession da sessiya egasi ekanligi tekshiruvi.
  - Fail-closed rol validatsiyasi kiritildi.
  - Rate-limiter (unary/stream) yagona instansga o'tkazildi va yozuvlar tozalanishi joriy etildi.
  - MaxConcurrentStreams orqali global sessiya chegarasi o'rnatildi.
  - Xabar uzunligiga qat'iy cheklov qo'shildi.
  - LLM xatolari to'g'ridan-to'g'ri qaytarilmay, umumiy xato orqali sanitizatsiya qilindi.
  - Outbox retry backoff sikli tuzatildi.
  - Multi-tenant uchun `tenantID` to'g'ri o'qib uzatilishi joriy etildi.

### O'zgardi
- Modul yo'li `github.com/muhammadvadud/alora-ai` ga o'zgartirildi (repo nomi bilan bir xil bo'lishi uchun).

### Ma'lum cheklovlar
- Ovoz (Voice provider'lar) uchun 3.2 va 3.3 bandlari hali to'liq ishlamaydi (kalitlar yo'q).
- Kontrakt 1.5 taklifi backend tomondan tasdiq kutmoqda.
- Hallucination sinovi hozircha 40 ta suhbat uchun o'tkazildi (jami 100 ta qilinishi rejalashtirilgan).
