package curtains

import (
	"sort"
	"strings"
)

// MatchKnowledge — sessiya holatiga mos keluvchi bilim qoidalarini qaytaradi.
// Condition'dagi har kalit state maydoniga teng bo'lsa qoida mos keladi; bo'sh
// condition — umumiy (hamma holatga). Tartib: priority kamayish, tenglikda ID —
// deterministik (rules §4.6.1). Kalit xaritasi: room/style/pattern/transparency/light.
func MatchKnowledge(rules []KnowledgeRule, st SessionState) []KnowledgeRule {
	out := make([]KnowledgeRule, 0, len(rules))
	for _, r := range rules {
		if knowledgeMatches(r, st) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func knowledgeMatches(r KnowledgeRule, st SessionState) bool {
	if len(r.Condition) == 0 {
		return true
	}
	for key, cond := range r.Condition {
		if !st.conditionMatches(key, cond) {
			return false
		}
	}
	return true
}

// conditionMatches — bitta condition kalitini state maydoniga tenglaydi. Qiymat
// string yoki ro'yxat bo'lishi mumkin — birorta mos kelishi (qator bo'lsa) kifoya.
func (s SessionState) conditionMatches(key string, cond any) bool {
	sv, ok := s.matchValue(key)
	if !ok {
		return false
	}
	return contains(conditionValues(cond), sv)
}

// matchValue — condition kalitini SessionState maydoniga xaritalaydi. Noma'lum
// kalit hech qachon mos kelmaydi (jim o'tkazib yuborilmaydi — rules §1.5).
func (s SessionState) matchValue(key string) (string, bool) {
	switch key {
	case "room":
		return s.Room, true
	case "style":
		return s.Style, true
	case "pattern":
		return s.Pattern, true
	case "transparency":
		return s.Transparency, true
	case "light":
		return s.LightPreference, true
	}
	return "", false
}

// conditionValues — condition qiymatini ruxsat etilgan qiymatlar ro'yxatiga
// aylantiradi (string → bitta element, []any → string elementlar). Bo'sh qiymat
// bo'sh state maydoniga teng keladi — tenglik qat'iy, ko'p ma'nolik emas.
func conditionValues(cond any) []string {
	switch v := cond.(type) {
	case string:
		return []string{v}
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return append([]string(nil), v...)
	}
	return nil
}

// Sanitize — prompt injection himoyasi (rules §16.2): `<`/`>` belgilarini olib
// tashlaydi va matnni max belgiga kesadi. Tool va prompt.Builder bitta manbadan
// ishlatadi — ikki joyda turli filtr xatoga olib kelmaydi.
func Sanitize(text string, max int) string {
	clean := strings.Map(func(r rune) rune {
		if r == '<' || r == '>' {
			return -1
		}
		return r
	}, text)
	if max <= 0 {
		return ""
	}
	runes := []rune(clean)
	if len(runes) > max {
		return string(runes[:max])
	}
	return clean
}
