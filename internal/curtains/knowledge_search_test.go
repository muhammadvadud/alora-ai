package curtains

import "testing"

// B7 (ADR-020): so'z bo'yicha qidiruv.
//
// 2026-09-28 live: "bolalar xonasi uchun xavfsiz mato" so'rovi uchun so'z
// RO'YXATI bo'yicha qidirildi, lekin tavsiya matni butun ibora bilan kelgani
// uchun (`strings.Contains` butun `query` ni talab qilardi) hech narsa
// topilmagan edi. Endi so'z/prefix mosligi bilan kamida bitta mos so'z kifoya.

func knowledgeFixtures() []KnowledgeRule {
	return []KnowledgeRule{
		{ID: "k-kids-safe", Kind: "room", Priority: 5, Recommendation: "Bolalar xonasi uchun xavfsiz, yuviladigan mato tavsiya etiladi."},
		{ID: "k-blackout", Kind: "light", Priority: 3, Recommendation: "Janubga qaragan yotoqxona uchun blackout matoni."},
		{ID: "k-budget", Kind: "objection", Priority: 1, Recommendation: "Narx e'tiroziga bosqichli javob beriladi."},
	}
}

// B7: butun ibora emas, so'z mosligi — tavsiya topiladi.
func TestKnowledgeSearch_WordMatchInsteadOfWholePhrase(t *testing.T) {
	rules := knowledgeFixtures()
	got := KnowledgeSearch(rules, SessionState{}, "room_style", "bolalar xonasi uchun xavfsiz mato")
	if len(got) == 0 {
		t.Fatal("butun ibora mos kelmasa ham kamida bitta so'z mos kelishi kerak")
	}
	if got[0].ID != "k-kids-safe" {
		t.Fatalf("eng mos qoida birinchi bo'lishi kerak: %+v", got)
	}
}

// B7: registr va apostrof farqi ahamiyatga ega emas.
func TestKnowledgeSearch_CaseAndApostropheInsensitive(t *testing.T) {
	rules := []KnowledgeRule{{ID: "k1", Kind: "room", Priority: 1, Recommendation: "O'zbekiston ʻ geography uchun yengil mato."}}
	for _, q := range []string{"o'zbekiston", "OʻZBEKISTON", "geography uchun", "o`zbekiston"} {
		if got := KnowledgeSearch(rules, SessionState{}, "room_style", q); len(got) == 0 {
			t.Fatalf("q=%q: mos qoida topilmadi", q)
		}
	}
}

// B7: prefix mosligi — 4+ belgidan boshlab ishonchli.
func TestKnowledgeSearch_PrefixMatch(t *testing.T) {
	rules := knowledgeFixtures()
	if got := KnowledgeSearch(rules, SessionState{}, "room_style", "bolalar"); len(got) == 0 {
		t.Fatal("prefix mosligi ishlashi kerak (bolalar ← bolalar)")
	}
	// 3 belgidan qisqa token — kalit so'z, prefix mosligi qat'iy emas
	if got := KnowledgeSearch(rules, SessionState{}, "room_style", "mat"); len(got) != 0 {
		t.Fatalf("3 belgili prefix mosligi berilmasligi kerak: %+v", got)
	}
}

// B7: so'z bo'yicha qidiruv — bitta mos so'z kifoya, ko'proq mos so'z yuqorida.
func TestKnowledgeSearch_ScoreOrdering(t *testing.T) {
	rules := []KnowledgeRule{
		{ID: "weak", Kind: "general", Priority: 9, Recommendation: "Parda — mato — narx."},
		{ID: "strong", Kind: "general", Priority: 1, Recommendation: "Parda — mato — kichik bolalar uchun yengil."},
	}
	got := KnowledgeSearch(rules, SessionState{}, "general", "bolalar uchun yengil parda")
	if len(got) < 2 || got[0].ID != "strong" {
		t.Fatalf("ko'p mos so'zli qoida oldinga chiqishi kerak: %+v", got)
	}
}

// B7: holatga mos SHARTLI qoidalar saqlanadi — mos kelmasa chiqariladi.
func TestKnowledgeSearch_KeepsStateCondition(t *testing.T) {
	rules := []KnowledgeRule{
		{ID: "k-bed", Kind: "room", Priority: 5, Recommendation: "Yotoqxona uchun blackout.", Condition: map[string]any{"room": "bedroom"}},
	}
	// state mos kelmaydi → qoida chiqariladi (eski xatti-harakat saqlanadi)
	if got := KnowledgeSearch(rules, SessionState{Room: "kids"}, "room_style", "yotoqxona uchun"); len(got) != 0 {
		t.Fatalf("holatga mos kelmagan qoida chiqarilishi kerak: %+v", got)
	}
	// state mos keladi → qoida chiqadi
	if got := KnowledgeSearch(rules, SessionState{Room: "bedroom"}, "room_style", "yotoqxona uchun"); len(got) != 1 {
		t.Fatalf("holatga mos qoida chiqishi kerak: %+v", got)
	}
}

// B7: shartsiz UMUMIY qoidalar holatga qarab o'tkazilmaydi — "bolalar xonasi"
// savolida state'da `room` hali yozilmagan bo'lishi mumkin.
func TestKnowledgeSearch_GeneralRuleIgnoresState(t *testing.T) {
	rules := knowledgeFixtures() // barchasi shartsiz
	if got := KnowledgeSearch(rules, SessionState{}, "room_style", "xavfsiz"); len(got) == 0 {
		t.Fatal("shartsiz qoida bo'sh holatda ham ko'rinishi kerak")
	}
}

// B7: so'ralgan mavzuda natija chiqmasa — mavzusiz qayta qidiriladi (model
// noto'g'ri mavzu tanlaganda bo'sh javobga tushmasin).
func TestKnowledgeSearch_TopicFallback(t *testing.T) {
	rules := knowledgeFixtures()
	// mavzu "color" — hech qanday rang qoidasi yo'q, lekin "xavfsiz" so'zi mos
	got := KnowledgeSearch(rules, SessionState{}, "color", "xavfsiz mato")
	if len(got) == 0 {
		t.Fatal("mavzuda natija yo'q bo'lsa, mavzusiz qayta qidirilishi kerak")
	}
}

// B7: so'z topilmasa hech narsa qaytmaydi (hallucination yo'q).
func TestKnowledgeSearch_NoMatchIsEmpty(t *testing.T) {
	if got := KnowledgeSearch(knowledgeFixtures(), SessionState{}, "general", "velyatsiya noyob"); len(got) != 0 {
		t.Fatalf("mos so'z yo'q — bo'sh ro'yxat: %+v", got)
	}
}

// B7: topic lug'ati — eski `room`/`style` bitta `room_style` ga xarilanadi.
func TestNormalizeTopic_Aliases(t *testing.T) {
	cases := map[string]KnowledgeTopic{
		"room":         TopicRoomStyle,
		"style":        TopicRoomStyle,
		"room_style":   TopicRoomStyle,
		"light":        TopicLight,
		"color":        TopicColor,
		"colour":       TopicColor,
		"construction": TopicConstruction,
		"objection":    TopicObjection,
		"general":      TopicGeneral,
		"":             TopicGeneral,
		"noma'lum":     TopicGeneral,
	}
	for in, want := range cases {
		if got := NormalizeTopic(in); got != want {
			t.Fatalf("NormalizeTopic(%q) = %q, %q kutilgan edi", in, got, want)
		}
	}
}

// B7: `general` hech narsani filtrlamaydi.
func TestKnowledgeTopic_TopicMatches(t *testing.T) {
	if !TopicGeneral.TopicMatches("room") {
		t.Fatal("general mavzu hech qanday tur bilan mos kelishi kerak")
	}
	if TopicRoomStyle.TopicMatches("light") {
		t.Fatal("room_style yorug'lik qoidasi bilan mos kelmemasligi kerak")
	}
	if !TopicRoomStyle.TopicMatches("room") || !TopicRoomStyle.TopicMatches("style") {
		t.Fatal("room_style ham room, ham style qoidasi bilan mos kelishi kerak")
	}
}

// B7: token ajratish — 3 harfdan kichik so'z, raqam va belgilar tashlanadi.
func TestKnowledgeTokens(t *testing.T) {
	got := KnowledgeTokens("Bolalar xonasi, 3 dona — K=2.5!")
	want := []string{"bolalar", "xonasi", "dona", "2.5"}
	if len(got) != len(want) {
		t.Fatalf("tokenlar: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tokenlar: %v != %v", got, want)
		}
	}
}
