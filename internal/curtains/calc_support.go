package curtains

// Rulon/rim — tayyor mexanizmli, burmasiz mahsulot. Hozirgi kalkulyator ularni
// to'g'ri narxlay olmaydi, shuning uchun buyurtmachi qarori: AI ularni
// hisoblamaydi, narxni sotuvchi qo'lda belgilaydi (ADR-014).
//
// Tekshiruv ROLGA emas, MAHSULOTNING QATLAMIGA (LayerOf) qaraydi: 2026-09-28
// live'da model `role:"rulon"` rad etilgach, "portyera" roli bilan K=2.5 va
// tikuv xizmati bilan ~2.5x noto'g'ri narxni hisoblagan edi.

// uncalcableLayers — AI orqali hisoblanmaydigan qatlamlar (LayerOf qiymatlari).
var uncalcableLayers = map[string]bool{"rulon": true, "rim": true}

// MsgNotCalcSupported — sotuvchiga beriladigan aniq xato matni. Bir xil matn
// tool natijasida (errors-as-results) va fallback promptda qo'llanadi.
const MsgNotCalcSupported = "rulon/rim pardalar hozircha AI orqali hisoblanmaydi — narxni sotuvchi qo'lda belgilaydi"

// CalcSupportLayer — variant qatlamini LayerOf bo'yicha aniqlaydi va uni AI
// orqali hisoblash mumkinligini qaytaradi. Variant katalogda topilmasa
// qatlam bo'sh, supported=true — yo'qligini validatsiya alohida tekshiradi.
func CalcSupportLayer(v *View, variantID string) (layer string, supported bool) {
	if v == nil {
		return "", true
	}
	vx, ok := v.Variant(variantID)
	if !ok {
		return "", true
	}
	p, ok := v.Product(vx.ProductID)
	if !ok {
		return "", true
	}
	return LayerOf(v, p), CalcSupportedProduct(v, p)
}

// CalcSupportedProduct — mahsulot AI orqali hisoblanadimi (rulon/rim = yo'q).
func CalcSupportedProduct(v *View, p Product) bool {
	return !uncalcableLayers[LayerOf(v, p)]
}

// PartsCalcSupported — bo'laklar ro'yxatida hisoblanmaydigan qatlam bormi.
// Paket qatlami katalogdan yig'iladi; model rolni boshqa qilib yuborishi
// mumkin, shuning uchun tekshiruv variant -> mahsulot -> LayerOf zanjirida.
func PartsCalcSupported(v *View, parts []PartSpec) bool {
	for _, p := range parts {
		if _, ok := CalcSupportLayer(v, p.VariantID); !ok {
			return false
		}
	}
	return true
}

// DropUnsupportedCards — qidiruv natijasidan rulon/rim mahsulotlarini olib
// tashlaydi. Qidiruvning o'zida qatlam filtri qat'iy, lekin katalog nomi yoki
// tegi noto'g'ri bo'lsa (masalan rulon "Portyera" kategoriyasida) mahsulot
// portyera deb o'tib ketishi mumkin — paketga tushmasligi uchun ikkinchi qatlam.
func DropUnsupportedCards(v *View, cards []ProductCard) []ProductCard {
	if len(cards) == 0 {
		return cards
	}
	out := cards[:0]
	for _, c := range cards {
		if p, ok := v.Product(c.ProductID); ok && !CalcSupportedProduct(v, p) {
			continue
		}
		out = append(out, c)
	}
	return out
}
