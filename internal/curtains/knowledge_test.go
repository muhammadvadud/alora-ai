package curtains

import (
	"testing"
)

func TestMatchKnowledge_Conditions(t *testing.T) {
	rules := []KnowledgeRule{
		{ID: "g", Kind: "room", Recommendation: "umumiy", Priority: 1, Condition: nil},
		{ID: "lr", Kind: "room", Recommendation: "living", Priority: 5, Condition: map[string]any{"room": "living"}},
		{ID: "lrs", Kind: "style", Recommendation: "living-classic", Priority: 3, Condition: map[string]any{"room": "living", "style": "classic"}},
		{ID: "no", Kind: "room", Recommendation: "not-mos", Priority: 9, Condition: map[string]any{"room": "bedroom"}},
	}
	st := SessionState{Room: "living", Style: "classic"}
	got := MatchKnowledge(rules, st)
	// umumiy (1) + living (5) + living-classic (3); priority tartibi: lr, lrs, g.
	if len(got) != 3 {
		t.Fatalf("son: %d, kutilgan 3; ids: %v", len(got), idsK(got))
	}
	want := []string{"lr", "lrs", "g"}
	if got[0].ID != "lr" || got[1].ID != "lrs" || got[2].ID != "g" {
		t.Fatalf("tartib: %v, kutilgan %v", idsK(got), want)
	}
}

func TestMatchKnowledge_ListCondition(t *testing.T) {
	rules := []KnowledgeRule{
		{ID: "a", Kind: "style", Priority: 1, Recommendation: "r1", Condition: map[string]any{"style": []any{"classic", "modern"}}},
		{ID: "b", Kind: "style", Priority: 1, Recommendation: "r2", Condition: map[string]any{"style": []any{"provence"}}},
	}
	got := MatchKnowledge(rules, SessionState{Style: "modern"})
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("ro'yxat mos: %v", idsK(got))
	}
}

func TestMatchKnowledge_ListStringCondition(t *testing.T) {
	rules := []KnowledgeRule{
		{ID: "s", Kind: "style", Priority: 1, Condition: map[string]any{"style": []string{"minimal", "modern"}}},
		{ID: "o", Kind: "style", Priority: 1, Condition: map[string]any{"style": []string{"provence"}}},
	}
	if got := MatchKnowledge(rules, SessionState{Style: "minimal"}); len(got) != 1 || got[0].ID != "s" {
		t.Fatal("[]string condition")
	}
}

func TestMatchKnowledge_LightField(t *testing.T) {
	rules := []KnowledgeRule{
		{ID: "l", Kind: "light", Priority: 1, Condition: map[string]any{"light": "qorong'iroq, shimol"}},
	}
	got := MatchKnowledge(rules, SessionState{LightPreference: "qorong'iroq, shimol"})
	if len(got) != 1 {
		t.Fatal("light mos emas")
	}
	got2 := MatchKnowledge(rules, SessionState{})
	if len(got2) != 0 {
		t.Fatal("bo'sh light")
	}
}

func TestMatchKnowledge_PriorityAndDeterminism(t *testing.T) {
	rules := []KnowledgeRule{
		{ID: "c", Kind: "room", Priority: 1, Condition: map[string]any{"room": "living"}},
		{ID: "a", Kind: "room", Priority: 1, Condition: map[string]any{"room": "living"}},
		{ID: "b", Kind: "room", Priority: 1, Condition: map[string]any{"room": "living"}},
		{ID: "d", Kind: "room", Priority: 3, Condition: map[string]any{"room": "living"}},
	}
	got := MatchKnowledge(rules, SessionState{Room: "living"})
	if len(got) != 4 {
		t.Fatalf("son: %d", len(got))
	}
	if got[0].ID != "d" || got[1].ID != "a" || got[2].ID != "b" || got[3].ID != "c" {
		t.Fatalf("tartib: %v", idsK(got))
	}
}

func TestMatchKnowledge_UnknownKeyIgnored(t *testing.T) {
	rules := []KnowledgeRule{
		{ID: "x", Kind: "room", Priority: 1, Condition: map[string]any{"noma'lum": "qiymat"}},
	}
	if got := MatchKnowledge(rules, SessionState{Room: "living"}); len(got) != 0 {
		t.Fatal("noma'lum kalit → mos emas")
	}
}

func TestSanitize_RemovesHTMLAndLimits(t *testing.T) {
	in := "<script>alert(1)</script> qo'llanma"
	got := Sanitize(in, 10)
	// `<`/`>` olib tashlandi → "scriptalert(1)/script qo'llanma", 10 belgi → "scriptaler"
	if len([]rune(got)) != 10 {
		t.Fatalf("uzunlik: %d", len([]rune(got)))
	}
	for _, r := range got {
		if r == '<' || r == '>' {
			t.Fatalf("HTML belgi qoldi: %q", got)
		}
	}
}

func TestSanitize_NoHTMLShorterThanMax(t *testing.T) {
	in := "qo'llanma"
	got := Sanitize(in, 500)
	if got != in {
		t.Fatalf("HTML yo'q: %q != %q", got, in)
	}
}

func TestSanitize_MaxZero(t *testing.T) {
	if got := Sanitize("salom", 0); got != "" {
		t.Fatalf("max=0: %q", got)
	}
}

func TestSanitize_OnlyTagsNoContent(t *testing.T) {
	got := Sanitize("<img><br/>", 20)
	// `<img><br/>` belgilaridan hech narsa qolmasligi kerak
	if got != "imgbr/" {
		t.Fatalf("faqat tag: %q", got)
	}
}

func idsK(rules []KnowledgeRule) []string {
	out := make([]string, len(rules))
	for i, r := range rules {
		out[i] = r.ID
	}
	return out
}
