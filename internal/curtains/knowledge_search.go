package curtains

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// knowledge_search.go — bilim qoidalarini so'z bo'yicha qidirish (ADR-020).
//
// Nima uchun alohida: 2026-09-28 live'da "bolalar xonasi xavfsizlik" so'rovi
// uchun so'z RO'YXATI bo'yicha qidirilgan, lekin tavsiya matni butun ibora
// ("Bolalar xonasi uchun xavfsiz mato...") bilan kelgani uchun hech narsa
// topilmagan. Endi qidiruv so'z/prefix bo'yicha, kamida bitta mos so'z kifoya.

// knowledgeTopic — so'rov mavzusining Yagona lug'ati. Topic katalog qoidasi
// (`kind`) bilan to'g'ri kelmaydi: `room` va `style` ikkalasi ham `room_style`
// ga xaritalanadi, `color` va `light` o'z nomi bilan qoladi.
type KnowledgeTopic string

const (
	TopicRoomStyle    KnowledgeTopic = "room_style"
	TopicLight        KnowledgeTopic = "light"
	TopicColor        KnowledgeTopic = "color"
	TopicConstruction KnowledgeTopic = "construction"
	TopicObjection    KnowledgeTopic = "objection"
	TopicGeneral      KnowledgeTopic = "general"
)

// knowledgeTopicAliases — model yuborgan topic → kanonik mavzu. Qabul qilinadigan
// qiymat shu ro'yxatdan; noma'lum yoki bo'sh — TopicGeneral (hech narsa
// filtrlanmaydi).
var knowledgeTopicAliases = map[string]KnowledgeTopic{
	"room":         TopicRoomStyle,
	"style":        TopicRoomStyle,
	"room_style":   TopicRoomStyle,
	"light":        TopicLight,
	"color":        TopicColor,
	"colour":       TopicColor,
	"construction": TopicConstruction,
	"objection":    TopicObjection,
	"general":      TopicGeneral,
	// Eski katalog `kind` qiymatlari — xaritalanadi, aks holda bo'sh natija.
	"pattern":        TopicColor,
	"transparency":   TopicLight,
	"product":        TopicGeneral,
	"pricing":        TopicGeneral,
	"service":        TopicGeneral,
	"care":           TopicGeneral,
	"measurement":    TopicGeneral,
	"installation":   TopicConstruction,
	"recommendation": TopicGeneral,
}

// NormalizeTopic — so'rovdagi topic matnini kanonik KnowledgeTopic'ga aylantiradi.
// Noma'lum yoki bo'sh — TopicGeneral.
func NormalizeTopic(topic string) KnowledgeTopic {
	t, ok := knowledgeTopicAliases[strings.ToLower(strings.TrimSpace(topic))]
	if !ok {
		return TopicGeneral
	}
	return t
}

// TopicMatches — qoida turi (`kind`) so'ralgan mavzuga tegishlimi. General
// mavzu hech qanday filtr qo'ymaydi (query bo'yicha qidiruv ishlaydi).
func (t KnowledgeTopic) TopicMatches(kind string) bool {
	if t == "" || t == TopicGeneral {
		return true
	}
	return TopicForKind(kind) == t
}

// TopicForKind — katalog qoidasi turi → so'rov mavzusi.
func TopicForKind(kind string) KnowledgeTopic {
	if t, ok := knowledgeTopicAliases[strings.ToLower(strings.TrimSpace(kind))]; ok {
		return t
	}
	return TopicGeneral
}

// minTokenRunes — qidiruvdagi eng qisqa so'z. "bor", "va" kabi kalitlarda
// tavsiya matni har doim bo'lgani uchun ulardan foyda yo'q.
const minTokenRunes = 3

// prefixMinRunes — prefix mosligi qanchadan boshlab ishonchli. "bolalar" va
// "bola" shakli 4+ belgi bilan bog'lanadi; 3 belgida ("bor" ~ "borr")
// yolg'on moslik chiqadi.
const prefixMinRunes = 4

// KnowledgeSearch — qoidalarni ikki bosqichli qidiradi:
//  1. Holatga mos qoidalar (MatchKnowledge) — shartli qoidalar faqat mos kelsa;
//  2. Umumiy (shartsiz) qoidalar — query berilganda holatga qarab o'tkazilmaydi.
//
// Nega umumiy qoidalar har doim qo'shiladi: "bolalar xonasi" savoli kelganda
// state'da `room=kids` HALI yozilmagan bo'lishi mumkin, shunda faqat holat
// filtri qo'llansa "xavfsiz mato" tavsiyasi topilmasdi.
func KnowledgeSearch(rules []KnowledgeRule, st SessionState, topic, query string) []KnowledgeRule {
	byState := MatchKnowledge(rules, st)
	t := NormalizeTopic(topic)
	tokens := KnowledgeTokens(query)
	if len(tokens) > 0 {
		seen := make(map[string]bool, len(byState))
		for _, rule := range byState {
			seen[rule.ID] = true
		}
		for _, rule := range rules {
			if !seen[rule.ID] && queryMatchesUnknownConditions(rule, st, tokens) {
				byState = append(byState, rule)
			}
		}
	}

	if len(tokens) == 0 {
		// Query yo'q — faqat mavzu bo'yicha. MatchKnowledge umumiy qoidalarni
		// ham qaytaradi (condition bo'sh), ular alohida qo'shilmaydi.
		out := filterByTopic(byState, t)
		if len(out) == 0 && t != TopicGeneral {
			out = byState // mavzu mos kelmadi — filtrsiz qaytarish
		}
		return out
	}

	// Mavzu qidiruvni TIKLAMAYDI: so'ralgan mavzuda natija chiqmasa, mavzusiz
	// qayta urinish — model noto'g'ri mavzu tanlaganda bo'sh javobga tushmasin.
	out := searchByWords(rules, byState, t, tokens)
	if len(out) == 0 && t != TopicGeneral {
		out = searchByWords(rules, byState, TopicGeneral, tokens)
	}
	return out
}

// searchByWords — so'z ballari bo'yicha qidiruv: holatga mos qodalar, so'ng
// (mavzu tanlangan bo'lsa) umumiy qodalar. Kamida bitta mos so'z kifoya.
func searchByWords(rules, byState []KnowledgeRule, t KnowledgeTopic, tokens []string) []KnowledgeRule {
	score := map[string]int{}
	order := make([]KnowledgeRule, 0, len(byState))
	add := func(r KnowledgeRule) {
		if _, seen := score[r.ID]; seen {
			return
		}
		if s := knowledgeScore(r, tokens); s > 0 {
			score[r.ID] = s
			order = append(order, r)
		}
	}
	for _, r := range byState {
		add(r)
	}
	if t != TopicGeneral {
		for _, r := range rules {
			if len(r.Condition) == 0 { // umumiy — holatga qarab o'tkazilmaydi
				add(r)
			}
		}
		order = filterByTopic(order, t)
	}
	return sortScored(order, score)
}

// numberMark — kasr sonlardagi ("2.5", "1,4") nuqta/yvergul o'rniga qo'yiladigan
// vaqtinchalik belgi: u harf ham, raqam ham emas, shuning uchun alohida token
// bo'lmaydi.
const numberMark = '\x00'

// numberRe — kasr son bitta token bo'lishi uchun.
var numberRe = regexp.MustCompile(`\d+(?:[.,]\d+)*`)

// markNumbers — kasr sonlarni `numberMark` bilan belgilaydi. KnowledgeTokens
// ichida Map DAN OLDIN chaqiriladi: aks holda nuqta birinchilikda bo'shliqqa
// aylanib, "2.5" ikkita qisqa raqamga bo'linib ketardi.
func markNumbers(s string) string {
	return numberRe.ReplaceAllStringFunc(s, func(n string) string {
		return strings.NewReplacer(".", string(numberMark), ",", string(numberMark)).Replace(n)
	})
}

// KnowledgeTokens — so'rov matnidan qidiruv tokenlarini ajratadi: kichik
// harfga, apostrofning uch ko'rinishini bir belgiga keltirib, raqam va
// belgilarni olib tashlab, kamida 3 belgili qismlardan iborat ro'yxat.
func KnowledgeTokens(query string) []string {
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case 'ʻ', '’', 'ʼ', '`', '\'':
			return 'ʼ' // o'zbek apostrofi bitta ko'rinishga
		case numberMark:
			return numberMark // kasr son belgisi — keyin tiklanadi
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return ' '
	}, markNumbers(query))
	seen := map[string]bool{}
	var out []string
	for _, tok := range strings.Fields(cleaned) {
		tok = strings.ReplaceAll(tok, string(numberMark), ".")
		tok = strings.Trim(tok, "ʼ")
		tok = canonicalKnowledgeToken(tok)
		if countRunes(tok) < minTokenRunes || seen[tok] {
			continue
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return out
}

// knowledgeScore — qoida matni so'zlar bilan nechta mos keladi. Bitta mos so'z
// kifoya (0 — mos emas), ko'proq mos so'z yuqori ball. Tavsiya matni va shart
// qiymatlari (xona turi, uslub) ikkalasi ham tekshiriladi.
func knowledgeScore(r KnowledgeRule, tokens []string) int {
	haystack := normalizeHaystack(r.Recommendation)
	if len(haystack) == 0 {
		return 0
	}
	for k, v := range r.Condition {
		haystack += " " + normalizeHaystack(k+" "+toString(v))
	}
	words := strings.Fields(haystack)
	score := 0
	for _, tok := range tokens {
		if matchesToken(words, tok) {
			score++
		}
	}
	return score
}

// matchesToken — token so'zlar ro'yxatida bormi: aniq tenglik yoki ishonchli prefix.
func matchesToken(words []string, token string) bool {
	for _, w := range words {
		if w == token {
			return true
		}
		if countRunes(token) >= prefixMinRunes && strings.HasPrefix(w, token) {
			return true
		}
		if countRunes(w) >= prefixMinRunes && strings.HasPrefix(token, w) {
			return true
		}
	}
	return false
}

// filterByTopic — mavzu filtri. TopicGeneral yoki bo'sh bo'lsa o'zgartirmaydi,
// aks holda TARTIB SAQLANADI (MatchKnowledge tartibi yoki ball tartibi).
func filterByTopic(rules []KnowledgeRule, t KnowledgeTopic) []KnowledgeRule {
	if t == TopicGeneral || t == "" {
		return rules
	}
	out := make([]KnowledgeRule, 0, len(rules))
	for _, r := range rules {
		if t.TopicMatches(r.Kind) {
			out = append(out, r)
		}
	}
	return out
}

// sortScored — ball kamayish, tenglikda priority kamayish, keyin ID —
// deterministik (rules §4.6.1).
func sortScored(rules []KnowledgeRule, score map[string]int) []KnowledgeRule {
	sort.SliceStable(rules, func(i, j int) bool {
		si, sj := score[rules[i].ID], score[rules[j].ID]
		if si != sj {
			return si > sj
		}
		if rules[i].Priority != rules[j].Priority {
			return rules[i].Priority > rules[j].Priority
		}
		return rules[i].ID < rules[j].ID
	})
	return rules
}

// normalizeHaystack — matnni qidiruv shakliga keltiradi (apostrof, kichik harf).
func normalizeHaystack(s string) string { return strings.Join(KnowledgeTokens(s), " ") }

func countRunes(s string) int { return len([]rune(s)) }

func queryMatchesUnknownConditions(rule KnowledgeRule, state SessionState, tokens []string) bool {
	if len(rule.Condition) == 0 {
		return false
	}
	for key, value := range rule.Condition {
		actual, supported := state.matchValue(key)
		if !supported {
			return false
		}
		if actual != "" {
			if !state.conditionMatches(key, value) {
				return false
			}
			continue
		}
		matched := false
		for _, candidate := range conditionValues(value) {
			words := KnowledgeTokens(candidate)
			for _, token := range tokens {
				if matchesToken(words, token) {
					matched = true
				}
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
