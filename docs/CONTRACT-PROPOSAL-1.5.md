# Alora - GO-CONTRACT 1.5 uchun TAKLIF

Bu hujjat Alora tizimining Go va Django qismlari o'rtasidagi kontraktni (v1.5) yangilash bo'yicha takliflarni o'z ichiga oladi.

## 1. 2.19 — stale katalog ogohlantirishi
* **Maqsad:** Katalog uzilib yangilanmay eski bo'lib qolganida (stale), frontend'da bildirishnoma chiqarish uchun ogohlantirish.
* **Proto/kontrakt o'zgarishi:** `ChatEvent` xabariga quyidagilar qo'shiladi:
  - `bool catalog_stale = 9;`
  - `repeated string warnings = 10;`
  *Tanlov va sabab:* `catalog_stale` bayrog'i UI'da qat'iy qizil banner ko'rsatish uchun aniq signal, `warnings` ro'yxati esa boshqa biznes ogohlantirishlari (masalan, "zapas kam") uchun kengaytiriluvchan platforma beradi.
* **Django tomonidagi ish:** WebSocket gateway orqali kelgan `catalog_stale` va `warnings` ni mobil ilovaga (Frontend) uzatish.
* **Go tomonidagi ish:** Katalog eski (stale) bo'lgan holatda har bir so'nggi `done` event'da `catalog_stale = true` ni yozish va `warnings` ga ogohlantirish kiritish.
* **Test rejasi:** Stale katalog simulatsiyasi yordamida Go javobida bayroq va ogohlantirishning to'g'ri chiqishi tekshiriladi.
* **Xavf:** Oqim ko'p uzilsa, frontend doimiy xato ko'rsatib mijozni cho'chitishi mumkin.
* **Tasdiqlanmasa nima bo'ladi:** Mijozlar eskirgan narxda va katalogda ma'lumot olayotganligini sezmaydi.
* **Kim tasdiqlaydi:** Frontend (Flutter) va Backend jamoasi.
* **Tartib:** kontrakt → Django → Go → e2e.

## 2. 4.2 — prompt A/B
* **Maqsad:** Turli xil prompt versiyalarini (A/B testing) bir vaqtda ishlatish va solishtirish.
* **Proto/kontrakt o'zgarishi:**
  - `CatalogSnapshot` ga `repeated CatalogPrompt prompt_versions = 14;` qo'shiladi.
  - `CatalogPrompt` ga `bool is_current = 4; int32 weight = 5;` qo'shiladi.
  - `Usage` xabariga (yoki UsageRequest.usage) `int32 prompt_version = 13;` qo'shiladi.
* **Django tomonidagi ish:** `PromptVersion` modelida `is_active` va `traffic_weight` maydonlari yaratilishi. Snapshot va Watch orqali barcha faol ro'yxatni jo'natish. So'rovda tanlanganini yuborish. `UsageRecord` ga `prompt_version` yozilishi.
* **Go tomonidagi ish:** Keshda `prompt_versions` ro'yxatini saqlash. So'rovda `context["prompt_version"]` kelsa ushani oladi, bo'lmasa tasodifiy (weight asosida) yoki `is_current` ni tanlaydi. Usage yuborganda shu versiya raqamini xabar qiladi.
* **Test rejasi:** Snapshot orqali kelgan ro'yxatning to'g'ri keshlash, so'rovdagi parametr bo'yicha to'g'ri versiyani ishlatish.
* **Xavf:** Prompt caching samaradorligi tushib, token xarajatlari ortishi.
* **Tasdiqlanmasa nima bo'ladi:** Faqatgina yagona joriy system prompt ishlaydi, sinovlar xavfsiz o'tkazilmaydi.
* **Kim tasdiqlaydi:** Product Manager va Backend.
* **Tartib:** kontrakt → Django → Go → e2e.

## 3. 4.9 — Agent.ArchiveSession
* **Maqsad:** Manager chatni qo'lda yopganida, arxivlashni zudlik bilan, aniq buyruq orqali bajarish.
* **Proto/kontrakt o'zgarishi:** `Agent` servisiga yangi RPC qo'shiladi (yangi metod bo'lgani uchun backward compatible):
  - `rpc ArchiveSession(ArchiveRequest) returns (Ack);`
  - `message ArchiveRequest { string session_id = 1; string reason = 2; }`
* **Django tomonidagi ish:** Manager suhbatni yopgan paytda, ushbu metodni chaqirib yuborish.
* **Go tomonidagi ish:** `internal/agent` dagi tayyor Archiver ga RPC ni ulash. Chaqirilganda `sessions` jadvalidan `archived_messages` ga ko'chirish. `ResetSession` ham ichkaridan o'zgarishsiz ishlayveradi (u ham shuni chaqiradi).
* **Test rejasi:** `ArchiveSession` ni chaqirib, baza darajasida ma'lumotning arxivga to'g'ri o'tishini sinash.
* **Xavf:** Noto'g'ri (faol) sessiyalar arxivlanib qolib ma'lumot yo'qolishi mumkin.
* **Tasdiqlanmasa nima bo'ladi:** Manager yopganda agent bilmaydi, 90 kunlik cron tozalashini kutib, xotira band bo'lib turadi.
* **Kim tasdiqlaydi:** Backend jamoasi.
* **Tartib:** kontrakt → Django → Go → e2e.

## 4. 4.5 — rasm bo'yicha tavsiya
* **Maqsad:** Mijoz tashlagan uy/oyna rasmi orqali vizual uslub tahlili va parda tavsiyasi qilish.
* **Proto/kontrakt o'zgarishi:** Proto bevosita o'zgarmaydi, ammo `ChatRequest.context["image_url"]` kaliti ruxsat etiladi (qoidaga qo'shiladi).
* **Django tomonidagi ish:** Yuklangan rasmni S3/MinIO ga saqlab, vaqtinchalik yaroqli (presigned) URL ni `context["image_url"]` qilib yuborish.
* **Go tomonidagi ish:** `internal/vision` paketi taklif etiladi. LLM vision provider (GPT-4o/Claude) yordamida rasm o'qilib, natijalari uslub/rang formatiga keladi va `set_context` orqali tizim state'iga yoziladi. Vaqt, hajm, va PII xavfsizlik cheklovlari bo'ladi.
* **Test rejasi:** Mock URL orqali vision model xulqi, xato holatlari, va state ga to'g'ri yozilishi tekshiriladi.
* **Xavf:** Mijozning shaxsiy sirlari (PII) chiqib ketishi va katta hisoblash xarajati/kechikish.
* **Tasdiqlanmasa nima bo'ladi:** Tizim rasm formatini ko'ra olmaydi, mijoz vizual tavsiya ola olmaydi.
* **Kim tasdiqlaydi:** Buyurtmachi (xarajat uchun) va Backend.
* **Tartib:** kontrakt → Django → Go → e2e.

## 5. 4.12 — ombor qoldig'i v2
* **Maqsad:** Zaxirada yo'q matolarni tavsiya qilmaslik hamda mavjud variantlarni reytingda yuqoriga ko'tarish.
* **Proto/kontrakt o'zgarishi:** `CatalogVariant` ga quyidagilar qo'shiladi:
  - `string stock_qty = 11;` (decimal matn ko'rinishida)
  - `bool in_stock = 12;`
* **Django tomonidagi ish:** Qoldiq miqdori o'zgarganda Delta/Snapshot orqali joriy ma'lumotni Go'ga muntazam jo'natish.
* **Go tomonidagi ish:** Katalogda bularni saqlash, `product_search` skoriga `in_stock = true` holatida `+2` qo'shish, `stock_check` tool qo'shish va zarurat bo'lganda ogohlantirish matniga xabar yozish.
* **Test rejasi:** Skorlash ro'yxatida mavjud matolar birinchi chiqishi va stock xabari o'tishi.
* **Xavf:** Har qanday savdo harakatidan keyin Delta oqimi oshib ketishi va gRPC/Redis ga ortiqcha yuk.
* **Tasdiqlanmasa nima bo'ladi:** AI mavjud bo'lmagan matoni sotib yuboradi, mijoz buyurtmasi ortga qaytadi.
* **Kim tasdiqlaydi:** Buyurtmachi va Backend jamoasi.
* **Tartib:** kontrakt → Django → Go → e2e.

## 6. 1.11 — tests/fixtures/calc_cases.json ko'p matoli holatlar
* **Maqsad:** 5 va 10 bo'lakli (murakkab) pardalarning narx hisobini aniq ekanligini isbotlash.
* **Proto/kontrakt o'zgarishi:** Hech qanday proto o'zgarmaydi. JSON struktura saqlanib ma'lumot qo'shiladi.
* **Django tomonidagi ish:** 5 va 10 ta bo'lakli o'lchovlar uchun etalon yechimlar `calc_cases.json` ga hisoblab kiritilishi.
* **Go tomonidagi ish:** `sync_fixtures.sh` orqali yangilangan faylni ko'chirib olib, Go testlari golden/live da shu yangi etalonlar bilan muvaffaqiyatli ishlayotganini tasdiqlash.
* **Test rejasi:** `make check-fixtures` va `go test` etalon testlarini ishga tushirib 100% o'tishini nazorat qilish.
* **Xavf:** Yangi murakkab etalon algoritmlardagi zaifliklarni fosh qilib, katta refaktoring so'rashi.
* **Tasdiqlanmasa nima bo'ladi:** Tizim katta bo'lakli buyurtmalarga qanchalik tayyorligi sirligicha qoladi.
* **Kim tasdiqlaydi:** Buyurtmachi va Backend.
* **Tartib:** kontrakt (etalon json) → Django → Go → e2e.

---

## Xulosa

| band | proto o'zgarishi bormi | backward compatible | taxminiy hajm (S/M/L) |
|---|---|---|---|
| 1. stale ogohlantirishi | Ha | Ha | S |
| 2. prompt A/B | Ha | Ha | M |
| 3. ArchiveSession | Ha | Ha | S |
| 4. rasm tavsiyasi | Yo'q (faqat context) | Ha | L |
| 5. ombor qoldig'i | Ha | Ha | M |
| 6. ko'p matoli test | Yo'q (faqat json) | Ha | S |
