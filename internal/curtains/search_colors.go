package curtains

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

// deltaEMax — CIE76 bo'yicha "bir-biriga yaqin" rang chegarasi (rules §4.3.3: ΔE < 25).
const deltaEMax = 25.0

// colorGroup — bitta rangning sinonimlari va etalon kodi. aliases normalize() bilan
// qayta ishlangan (faqat lotin kichik harf) — har variantda qayta normalizatsiya shart emas.
type colorGroup struct {
	hex     string
	aliases []string
}

// rawColor — buildColorGroups uchun xom jadval satri (dastlabki yozuvlar, kirill bilan).
type rawColor struct {
	words []string
	hex   string
}

// colorGroups — rang sinonimlari lug'ati. Bu BIZNES-QIYMAT EMAS, til yordamchisi
// (rules §4.3.3): uz lotin + kirill + en nomlar bitta rangga birlashtiriladi.
// Shunda "oq" so'rovi "Белый" nomli variantni ham topadi (kirill→lotin normalizatsiya).
var colorGroups = buildColorGroups()

// buildColorGroups xom jadvalni bir marta normalizatsiya qilib quriladigan o'zgarmas lug'at.
// Paket boshlanishida yig'iladi, keyin hech qayerda o'zgartirilmaydi (global mutable holat yo'q).
func buildColorGroups() []colorGroup {
	raw := []rawColor{
		{words: []string{"bej", "beige", "бежевый"}, hex: "#F5F5DC"},
		{words: []string{"grafit", "graphite", "графит"}, hex: "#404040"},
		{words: []string{"oq", "white", "белый"}, hex: "#FFFFFF"},
		{words: []string{"qora", "black", "чёрный", "черный"}, hex: "#000000"},
		{words: []string{"kulrang", "gray", "grey", "серый"}, hex: "#808080"},
		{words: []string{"ko'k", "blue", "синий"}, hex: "#0000FF"},
		{words: []string{"yashil", "green", "зелёный"}, hex: "#008000"},
		{words: []string{"jigarrang", "brown", "коричневый"}, hex: "#8B4513"},
		{words: []string{"qizil", "red", "красный"}, hex: "#FF0000"},
		{words: []string{"pushti", "pink", "розовый"}, hex: "#FFC0CB"},
		{words: []string{"sariq", "yellow", "жёлтый"}, hex: "#FFFF00"},
		{words: []string{"oltin", "gold", "золотой"}, hex: "#FFD700"},
		{words: []string{"kumush", "silver", "серебряный"}, hex: "#C0C0C0"},
		{words: []string{"binafsha", "purple", "фиолетовый"}, hex: "#800080"},
		{words: []string{"krem", "cream", "кремовый"}, hex: "#FFFDD0"},
	}
	groups := make([]colorGroup, 0, len(raw))
	for _, r := range raw {
		aliases := make([]string, 0, len(r.words))
		for _, w := range r.words {
			aliases = append(aliases, normalize(w))
		}
		groups = append(groups, colorGroup{hex: r.hex, aliases: aliases})
	}
	return groups
}

// normalize rang so'zini qiyos shakliga keltiradi: kichik harf, kirill → lotin,
// apostrof (', ʼ) olib tashlanadi. Sabab — uz lotin "oq" bilan kirill "белый"
// bitta lug'atga, "ko'k" esa "kok" ga tushishi kerak.
func normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		r = unicode.ToLower(r)
		switch r {
		case '\'', 'ʼ', '‘', '’', '`', '´':
			continue
		}
		if t, ok := cyrillicToLatin[r]; ok {
			b.WriteString(t)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// cyrillicToLatin — kirill harflarini lotin transliteratsiyasi (Cyrillic+Uzbek).
// Kirill yozilgan rang nomlari normalize'dan keyin lotin so'zlar bilan bir xil bo'ladi.
var cyrillicToLatin = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "yo",
	'ж': "zh", 'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m",
	'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u",
	'ф': "f", 'х': "h", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "shch",
	'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
	'ў': "o", 'қ': "q", 'ғ': "g", 'ҳ': "h",
}

// lookupGroup so'zni sinonim guruhiga o'xshatadi. Guruhlar soni kichik (15) —
// har variantda chiziqli qidiruv 5 ms byudjetga sig'adi (rules §4.3.5).
func lookupGroup(word string) (colorGroup, bool) {
	for i := range colorGroups {
		if colorGroups[i].has(word) {
			return colorGroups[i], true
		}
	}
	return colorGroup{}, false
}

// has so'z shu guruh a'zosimi ekanini tekshiradi (normalizatsiya qilingan holda).
func (g colorGroup) has(word string) bool {
	for _, a := range g.aliases {
		if a == word {
			return true
		}
	}
	return false
}

// wordMatch — bitta so'rov so'zi va uning sinonim guruhi (topilmasa bo'sh group).
type wordMatch struct {
	word  string
	group colorGroup
}

// colorMatcher — so'rov rangi bir marta tahlil qilinadi, har variantda qayta
// normalizatsiya/guruh qidiruvi takrorlanmaydi (10k×3 variantda takrori qimmat edi).
// labs — variant hex'larining Lab keshla: katalogda bitta rang ko'p variantda takrorlanadi,
// shu sababli har match'da math.Cbrt/Pow qaytadan hisoblanmaydi (rules §4.3.5).
type colorMatcher struct {
	words   []wordMatch
	etalon  lab // birinchi topilgan guruhning Lab koordinatasi (ΔE uchun)
	haveHex bool
	labs    map[string]labVal // colorHex → Lab (valid flag bilan)
}

// labVal — keshlangan Lab va uning yaroqligi (noto'g'ri hex ham "koklangan", qayta parse'lanmaydi).
type labVal struct {
	l     lab
	valid bool
}

// newColorMatcher so'rovni parserlab matcher quriladi. Bo'sh so'rov — bo'sh matcher
// (hech narsa mos kelmaydi, Search chiqishida tez yo'l).
func newColorMatcher(query string) colorMatcher {
	var m colorMatcher
	if query == "" {
		return m
	}
	m.labs = make(map[string]labVal, 8)
	for _, w := range strings.Fields(normalize(query)) {
		if w == "" {
			continue
		}
		g, ok := lookupGroup(w)
		if ok && g.hex != "" && !m.haveHex {
			m.etalon = srgbToLab(parseHex(g.hex))
			m.haveHex = true
		}
		m.words = append(m.words, wordMatch{word: w, group: g})
	}
	return m
}

// match variant rangiga moslikni tekshiradi (rules §4.3.3):
// (a) so'z yoki uning sinonimi colorName ichida (substring);
// (b) so'z ma'lum rang bo'lsa — colorHex bilan etalon CIE76 ΔE < 25.
// Hex bo'sh/noto'g'ri bo'lsa faqat (a) ishlaydi.
func (m colorMatcher) match(colorName, colorHex string) bool {
	if len(m.words) == 0 {
		return false
	}
	// Fast yo'l: nom butunlay ASCII bo'lsa normalize'ning rune-builder traini kerak emas,
	// oddiy ToLower yetarli (o'sha natija — apostrof/kirill bo'lmaganida). Katalog nomlari
	// ko'pincha "Oq", "Grafit" kabi — 10k×3 da bu sezilarli (rules §4.3.5).
	cn := colorName
	ascii := true
	for i := range cn {
		if cn[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		cn = strings.ToLower(cn)
	} else {
		cn = normalize(cn)
	}
	for _, wm := range m.words {
		if strings.Contains(cn, wm.word) {
			return true
		}
		for _, a := range wm.group.aliases {
			if strings.Contains(cn, a) {
				return true
			}
		}
	}
	// Yuqorida o'xshashlik bo'lmasa — etalon rang bilan ΔE (agar so'rov ma'lum rang bo'lsa).
	// Hex yaroqsiz bo'lsa ΔE yo'li ishlamaydi, natija false (faqat nom bilan moslashardi).
	if m.haveHex {
		e, ok := m.labs[colorHex]
		if !ok {
			r := parseHex(colorHex)
			if r.valid {
				e = labVal{l: srgbToLab(r), valid: true}
			}
			m.labs[colorHex] = e
		}
		if e.valid {
			return deltaE(m.etalon, e.l) < deltaEMax
		}
	}
	return false
}

// colorScore — moslik tekshiruvining ochiq ko'rinishi; testlar shu orqali yoziladi.
func colorScore(query, colorName, colorHex string) bool {
	return newColorMatcher(query).match(colorName, colorHex)
}

// rgb — RGB komponentalari; valid false bo'lsa hex yaroqsiz.
type rgb struct {
	r, g, b float64
	valid   bool
}

// parseHex "#RGB"/"#RRGGBB"/"RRGGBB" shaklidagi rangni o'qiydi.
// Noto'g'ri shakl → valid=false: ΔE hisoblanmaydi, faqat nom bo'yicha moslanadi.
func parseHex(s string) rgb {
	var c rgb
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	if len(s) == 3 {
		s = s[:1] + s[:1] + s[1:2] + s[1:2] + s[2:3] + s[2:3]
	}
	if len(s) != 6 {
		return c
	}
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return c
	}
	c.r = float64((n >> 16) & 0xff)
	c.g = float64((n >> 8) & 0xff)
	c.b = float64(n & 0xff)
	c.valid = true
	return c
}

// lab — CIE Lab koordinatalari (ΔE hisobi uchun).
type lab struct {
	l, a, b float64
}

// srgbToLab sRGB → Lab (D65, CIE76 standart konvertatsiyasi). Taqqoslash Lab fazoda
// o'tkaziladi — RGB da emas, chunki Lab "ko'zga yaqinmi"ni fiziologikga mos hisoblaydi.
func srgbToLab(c rgb) lab {
	lin := func(v float64) float64 {
		v /= 255
		if v > 0.04045 {
			return math.Pow((v+0.055)/1.055, 2.4)
		}
		return v / 12.92
	}
	r, g, b := lin(c.r), lin(c.g), lin(c.b)
	x := (r*0.4124564 + g*0.3575761 + b*0.1804375) / 0.95047
	y := r*0.2126729 + g*0.7151522 + b*0.0721750
	z := (r*0.0193339 + g*0.1191920 + b*0.9503041) / 1.08883
	f := func(t float64) float64 {
		if t > 0.008856 {
			return math.Cbrt(t)
		}
		return 7.787*t + 16.0/116.0
	}
	fx, fy, fz := f(x), f(y), f(z)
	return lab{l: 116*fy - 16, a: 500 * (fx - fy), b: 200 * (fy - fz)}
}

// deltaE — CIE76 rang farqi (Lab fazosidagi Evklid masofasi).
func deltaE(x, y lab) float64 {
	dl, da, db := x.l-y.l, x.a-y.a, x.b-y.b
	return math.Sqrt(dl*dl + da*da + db*db)
}
