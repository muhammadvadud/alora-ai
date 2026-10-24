package curtains

import "testing"

func TestKnowledgeSearch_MultilingualSalesQuestions(t *testing.T) {
	rules := []KnowledgeRule{
		{ID: "care", Kind: "general", Recommendation: "Parvarish: baxmal uchun yuvish yorlig'i tekshiriladi."},
		{ID: "privacy", Kind: "light", Recommendation: "Tul kechasi maxfiylik bermaydi, blackout boshqa vazifani bajaradi."},
		{ID: "safety", Kind: "room_style", Condition: map[string]any{"room": "kids"}, Recommendation: "Bolalar uchun shnur xavfsizligi, shnursiz boshqaruv."},
		{ID: "measure", Kind: "construction", Recommendation: "O'lchov: karniz uzunligi va balandlik aniqlanadi."},
	}
	cases := []struct {
		query string
		topic string
		want  string
	}{
		{"Бархат можно стирать?", "general", "care"},
		{"Бахмални ювиш мумкинми?", "general", "care"},
		{"baxmalni qanday yuvaman", "general", "care"},
		{"Тюль обеспечивает приватность ночью?", "light", "privacy"},
		{"Тул кечаси ташқаридан кўринадими?", "light", "privacy"},
		{"Безопасность шнуров в детской", "room_style", "safety"},
		{"Болалар хонасида шнур хавфлими?", "room_style", "safety"},
		{"Как измерить высоту и длину карниза?", "construction", "measure"},
	}
	for _, test := range cases {
		t.Run(test.query, func(t *testing.T) {
			found := KnowledgeSearch(rules, SessionState{}, test.topic, test.query)
			if len(found) == 0 || found[0].ID != test.want {
				t.Fatalf("query=%q: want=%s got=%+v", test.query, test.want, found)
			}
		})
	}
}

func TestKnowledgeSearch_QueryCannotOverrideKnownRoom(t *testing.T) {
	rules := []KnowledgeRule{{ID: "bedroom", Kind: "room_style", Condition: map[string]any{"room": "bedroom"}, Recommendation: "Yotoqxona uchun blackout."}}
	if found := KnowledgeSearch(rules, SessionState{Room: "kids"}, "room_style", "спальня blackout"); len(found) != 0 {
		t.Fatalf("boshqa xona qoidasi o'tmasligi kerak: %+v", found)
	}
}
