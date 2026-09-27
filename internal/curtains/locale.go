package curtains

import (
	"strings"
	"unicode"
)

// MessageLocale foydalanuvchi xabari matnidan javob tilini aniqlaydi. Faqat aniq holat
// qaytadi — rus kirill matni → "ru"; qolgan hammasi (lotin, raqam, o'zbek kirill, aralash)
// → "" va prompt ilova `locale` iga tayanadi (rules §7.5, ADR-013). Lotin matn ataylab
// aniqlanmaydi: "Velvet Milano" kabi mahsulot nomi rus suhbatini o'zbekchaga burmasin.
//
// Harf emas, so'z sanaladi: ruscha gapdagi uzun lotin mahsulot nomi ("Хочу Velvet Milano
// в спальню") harf bo'yicha ko'pchilikni egallab qo'yadi.
func MessageLocale(text string) string {
	var cyr, lat int
	for _, w := range strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) }) {
		for _, r := range w {
			if isUzbekCyrillic(r) {
				return ""
			}
		}
		if unicode.Is(unicode.Cyrillic, []rune(w)[0]) {
			cyr++
		} else {
			lat++
		}
	}
	// Bitta so'z ("да") — til signali ishonchsiz.
	if cyr >= 2 && cyr > lat {
		return "ru"
	}
	return ""
}

// isUzbekCyrillic — faqat o'zbek kirill alifbosida bor harflar (ў қ ғ ҳ).
func isUzbekCyrillic(r rune) bool {
	switch unicode.ToLower(r) {
	case 'ў', 'қ', 'ғ', 'ҳ':
		return true
	}
	return false
}
