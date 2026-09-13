//go:build live

package live

// Ssenariylar golden testdagi mavzularni real foydalanuvchi tili bilan tekshiradi.
// Bu yerda LLM javobi skript qilinmaydi: faqat xavfsizlik va oqim invariantlari
// tekshiriladi.
type llmScenario struct {
	Name           string
	Locale         string
	Role           string
	CustomerMode   bool
	Messages       []string
	ExpectToolsAny []string
	ForbidTools    []string
	ExpectStage    string
	MustNotContain []string
}

var llmScenarios = []llmScenario{
	{Name: "hi-tech mehmonxona", Role: "employee", Messages: []string{"Mehmonxonaga grafit rangli, zamonaviy parda kerak. Janub tomonga qaraydi, quyosh ko'p tushadi.", "Karniz 4 metr, balandligi 2.6 metr. Mos variantni hisoblang."}, ExpectToolsAny: []string{"set_context", "product_search", "curtain_calculate"}, ExpectStage: "calc"},
	{Name: "noto'g'ri o'lcham", Role: "employee", Messages: []string{"Karnizim 50 metr, balandligi 2.6 metr. Parda hisoblab bering."}, ForbidTools: []string{"price_quote"}, ExpectStage: "need"},
	{Name: "yo'q mahsulot", Role: "employee", Messages: []string{"To'y xonasi uchun atlas Roma matosi bormi? Katalogdan tekshirib bering."}, ExpectToolsAny: []string{"product_search"}, ExpectStage: "recommend"},
	{Name: "stale katalog", Role: "employee", Messages: []string{"Mehmonxonaga Velvet Milano matolarini ko'rsating."}, ExpectToolsAny: []string{"product_search"}, ExpectStage: "recommend"},
	{Name: "chegirma chegarasi", Role: "manager", Messages: []string{"Mehmonxona uchun oddiy portyera kerak, katalogdan birinchi mos matoni tanlang.", "Karniz 4 metr, balandlik 2.6 metr, shu matoga hisoblang.", "12 foiz chegirma bilan yakuniy narxni ayting."}, ExpectToolsAny: []string{"curtain_calculate", "price_quote"}, ExpectStage: "calc"},
	{Name: "uch paket", Role: "employee", Messages: []string{"Mehmonxona uchun zamonaviy parda kerak.", "Karniz 4 metr, balandlik 2.6 metr.", "Arzon, o'rtacha va premium variantlarni taqqoslang."}, ExpectToolsAny: []string{"build_options"}, ExpectStage: "variants"},
	{Name: "blackout yotoqxona", Role: "employee", Messages: []string{"Yotoqxona janubga qaraydi. Yorug'likni to'sadigan kulrang yoki oq parda kerak."}, ExpectToolsAny: []string{"set_context", "product_search", "knowledge_lookup"}, ExpectStage: ""},
	{Name: "bolalar xonasi", Role: "employee", Messages: []string{"Bolalar xonasiga sariq, gulli va oson yuviladigan parda qidiryapman."}, ExpectToolsAny: []string{"set_context", "product_search"}, ExpectStage: ""},
	{Name: "oshxona rim", Role: "employee", Messages: []string{"Oshxona uchun yengil rim parda kerak. Mos matolarni ko'rsating."}, ExpectToolsAny: []string{"product_search"}, ExpectStage: "recommend"},
	{Name: "klassik fason", Role: "employee", Messages: []string{"To'y xonasi uchun klassik uch qismli fason kerak. Karniz 4 metr, balandligi 2.6 metr.", "Shu fasonni hisoblab bering."}, ExpectToolsAny: []string{"fason_lookup", "curtain_calculate"}, ExpectStage: "calc"},
	{Name: "ofis minimal", Role: "employee", Messages: []string{"Ofis uchun oq yoki kulrang minimal uslubdagi parda tavsiya qiling."}, ExpectToolsAny: []string{"set_context", "product_search"}, ExpectStage: ""},
	{Name: "balkon eshigi", Role: "employee", Messages: []string{"Yotoqxonada balkon eshigi va ikkita rom bor. O'lchashda nimaga e'tibor beramiz?"}, ExpectToolsAny: []string{"knowledge_lookup", "set_context"}, ExpectStage: "need"},
	{Name: "ko'p matoli zal", Role: "employee", Messages: []string{"Katta zal uchun portyera, kant, tul, lambreken va astardan iborat kompozitsiya kerak. Karniz 4 metr, balandlik 2.6 metr.", "Hisobini chiqarib bering."}, ExpectToolsAny: []string{"curtain_calculate"}, ExpectStage: "calc"},
	{Name: "o'n bo'lak", Role: "employee", Messages: []string{"Zal uchun katalogdagi matolardan o'n bo'lakli kompozitsiya tuzing (portyera, tul, lambrekan, astar, kant — takrorlanishi mumkin). Karniz 4 metr, balandlik 2.6 metr, hisoblang."}, ExpectToolsAny: []string{"curtain_calculate"}, ExpectStage: "calc"},
	{Name: "tor mato", Role: "employee", Messages: []string{"Velvet Chek matosidan portyera kerak. Karniz 4 metr, balandlik 2.6 metr, chiqindini ham tushuntiring."}, ExpectToolsAny: []string{"curtain_calculate"}, ExpectStage: "calc"},
	{Name: "qimmat e'tirozi", Role: "manager", Messages: []string{"Karniz 4 metr, balandlik 2.6 metr uchun Velvet parda hisoblang.", "Bu qimmat ekan. Ruxsat etilgan chegirma bilan qayta taklif qiling.", "Yana arzonroq koeffitsient bilan hisoblash mumkinmi?"}, ExpectToolsAny: []string{"curtain_calculate", "price_quote"}, ExpectStage: "calc"},
	{Name: "boshqa rang", Role: "employee", Messages: []string{"Grafit rang yoqmadi. Bordo rangdagi matolarni katalogdan toping."}, ExpectToolsAny: []string{"product_search"}, ExpectStage: "recommend"},
	{Name: "buyurtma qoralamasi", Role: "employee", Messages: []string{"Mehmonxona uchun oddiy portyera kerak, katalogdan birinchi mos matoni tanlang. Karniz 4 metr, balandlik 2.6 metr, hisoblang.", "Yetkazib berish bilan yakuniy narx taklifini bering.", "Buyurtma qoralamasini tayyorlang."}, ExpectToolsAny: []string{"order_draft"}, ExpectStage: "calc"},
	{Name: "o'lcham o'zgardi", Role: "employee", Messages: []string{"Mehmonxona uchun oddiy portyera kerak, katalogdan birinchi mos matoni tanlab, karniz 4 metr, balandlik 2.6 metr uchun hisoblang.", "Karniz 3.5 metr, balandlik 2.5 metr bo'ldi. Qayta hisoblang."}, ExpectToolsAny: []string{"curtain_calculate"}, ExpectStage: "calc"},
	{Name: "rus mijoz rejimi", Locale: "ru", Role: "employee", CustomerMode: true, Messages: []string{"Сколько будут стоить шторы для спальни? Сначала подскажите, какие размеры нужны."}, ExpectToolsAny: []string{}, ForbidTools: []string{"curtain_calculate", "price_quote", "build_options"}, ExpectStage: "need", MustNotContain: []string{"tannarx", "себестоимость"}},
}
