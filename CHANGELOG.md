# Changelog

Barcha muhim o'zgarishlar ushbu faylda yozib boriladi.

Format [Keep a Changelog](https://keepachangelog.com/uz/1.1.0/) ga asoslangan.

## [Unreleased] - 2026-09-28

### O'zgartirildi (ADR-014, ADR-015, ADR-016, ADR-020)
- Rulon/rim AI orqali hisoblanmaydi: tekshiruv roly emas, `LayerOf` qatlamiga; tool aniq `MsgNotCalcSupported` qaytaradi (ADR-014).
- `build_options` da xona qat'iy filtr; `curtain_calculate windows[] + qty` ko'p o'chov, `MaxMeasurements=20`; paket `OptionSnapshot` (Django natijasi) sifatida qaytadi, keyingi hisob shu ustida (ADR-016).
- `product_search` takrorlanuvchi bo'sh natijada to'xtaydi (`EmptySearches`, `stop:true`); bo'sh katalog faqat tenant ma'lumotiga qaraydi; `catalog_sync` Watch uzilganda to'liq Snapshot so'raydi (`reason=bulk_import`).
- Har maydon, jumladan o'lchovlar, ko'pi bilan 2 marta taklif etiladi.
- Chegirma natijada ko'rinadi: `price_quote` va `curtain_calculate` uchun `discount` bloki (mijoz rejimida chegirma chegarasi yashiriladi).
- **`order_draft` PII yig'maydi va tasdiqlamaydi** (breaking, ADR-020): `action=quote|draft`, `confirm`/`phone`/`name`/`address` argumentlari olib tashlandi, `customer_form_required` qo'shildi; gRPC LLM kontekstiga yuborishdan oldin telefon maskalaydi (`pii.Scrub`), Django `LogMessage` asl matnni oladi. `backend.Orders` saqlanadi, tool uni ishlatmaydi.
- `knowledge_lookup` butun ibora emas, so'z/prefix mosligi bilan qidiradi; `room`+`style` → `room_style`; topic'siz fallback.

### Testlar
- Yangi: `internal/tools/catalog_empty_test.go`, `internal/tools/discount_test.go`, `internal/transport/grpc/pii_test.go`, `internal/curtains/asked_test.go`, `internal/curtains/knowledge_search_test.go`; `catalog_sync_test.go` boshqariladigan fake Watch/Snapshot ga o'tdi; golden 14/18 yangilandi.

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
