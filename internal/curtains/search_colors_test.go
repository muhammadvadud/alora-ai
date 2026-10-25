package curtains

import "testing"

func TestColorScore_SynonymsLatinCyrillic(t *testing.T) {
	cases := []struct {
		name      string
		query     string
		colorName string
		colorHex  string
		want      bool
	}{
		// uz lotin — uz lotin
		{"latin-latin", "oq", "Oq", "#FFFFFF", true},
		{"latin-latin ko'k apostrof", "ko'k", "Ko'k", "#0000FF", true},
		// uz lotin so'rov — kirill nom
		{"latin-cyrillic oq", "oq", "Белый", "#FFFFFF", true},
		{"latin-cyrillic yaxshi", "qora", "Чёрный", "#000000", true},
		{"latin cyrillic без ё", "qora", "черный", "#000000", true},
		// kirill so'rov — lotin nom
		{"cyrillic-latin", "белый", "Oq", "#FFFFFF", true},
		// en sinonim
		{"en-uz", "white", "Oq", "#FFFFFF", true},
		{"uz-en nom", "oq", "White", "#FFFFFF", true},
		// kirill variant o'z sinonimlari bilan
		{"cyrillic-cyrillic", "серый", "Серый", "#808080", true},
		// umuman boshqa rang — mos emas
		{"farib beruvchi", "oq", "Grafit", "#404040", false},
	}
	for _, tc := range cases {
		if got := colorScore(tc.query, tc.colorName, tc.colorHex); got != tc.want {
			t.Errorf("%s: colorScore(%q, %q, %q) = %v, kutilgan %v",
				tc.name, tc.query, tc.colorName, tc.colorHex, got, tc.want)
		}
	}
}

func TestColorScore_CIE76NearFar(t *testing.T) {
	// #FF1111 va #FF0000 (qizil etalon) orasida ΔE ≈ 4.3 — < 25, mos.
	if !colorScore("qizil", "Olcha", "#FF1111") {
		t.Error("qizilga yaqin hex (#FF1111, ΔE≈4.3) mos deb topilishi kerak")
	}
	// #FF8080 va #FF0000 orasida ΔE ≈ 56.8 — > 25, mos emas.
	if colorScore("qizil", "Sohil qumi", "#FF8080") {
		t.Error("qizildan uzoq hex (#FF8080, ΔE≈56.8) mos EMAS deb topilishi kerak")
	}
	if colorScore("qizil", "Amirsoy", "#00FF00") {
		t.Error("yashil hex qizil so'rovi bilan mos kelmasligi kerak")
	}
	if colorScore("qizil", "Gul", "#800080") {
		t.Error("binafsha hex qizil so'rovi bilan mos kelmasligi kerak")
	}
}

func TestColorScore_EtalonHexExactByHex(t *testing.T) {
	// nom mos kelmasa ham hex aynan etalon (ΔE=0) — mos.
	if !colorScore("ko'k", "Denim", "#0000FF") {
		t.Error("ko'k etaloniga teng hex (ΔE=0) mos deb topilishi kerak")
	}
	if colorScore("ko'k", "Denim", "#808080") {
		t.Error("kulrang hex ko'k so'rovi bilan mos kelmasligi kerak (ΔE≈135.5)")
	}
}

func TestColorScore_InvalidHexFallsBackToNameOnly(t *testing.T) {
	// noto'g'ri hex → ΔE yo'li ishlamaydi; nomda mos so'z bo'lmasa — false.
	if colorScore("qizil", "Palit", "not-hex") {
		t.Error("noto'g'ri hex va nomda mos so'z bo'lmaganda false qaytarishi kerak")
	}
	if colorScore("qizil", "Palit", "#ff000") {
		t.Error("5 belgili hex yaroqsiz — ΔE hisoblanmasligi kerak")
	}
	if !colorScore("qizil", "Qizil palit", "not-hex") {
		t.Error("noto'g'ri hex'da ham nom bo'yicha mos topilishi kerak (a yo'li)")
	}
	// bo'sh hex — faqat nom yo'li.
	if !colorScore("oq", "Oq", "" /* hex yo'q */) {
		t.Error("bo'sh hex'da nom bo'yicha mos topilishi kerak")
	}
}

func TestColorScore_EmptyQueryNeverMatches(t *testing.T) {
	if colorScore("", "Oq", "#FFFFFF") {
		t.Error("bo'sh so'rov hech narsaga mos kelmasligi kerak")
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Ko'k", "kok"},
		{"KO'K", "kok"},
		{"Белый", "belyy"},
		{"Чёрный", "chyornyy"},
		{"черный", "chernyy"},
		{"Графит", "grafit"},
		{"gre y", "gre y"},  // bo'shliq saqlanadi — nom ichida sinonim o'zgarmaydi
		{"ʼbelgi", "belgi"}, // U+02BC apostrof olib tashlanadi
	}
	for _, tc := range cases {
		if got := normalize(tc.in); got != tc.want {
			t.Errorf("normalize(%q) = %q, kutilgan %q", tc.in, got, tc.want)
		}
	}
}

func TestBuildColorGroupsCompleteness(t *testing.T) {
	want := 16 // Asosiy ranglar va qum/pesochniy til sinonimlari.
	if got := len(colorGroups); got != want {
		t.Fatalf("rang guruhlari soni %d, kutilgan %d", got, want)
	}
	for i := range colorGroups {
		if len(colorGroups[i].aliases) < 2 {
			t.Errorf("guruh[%d] (%#v) uz+en+ru sinonimi to'liq emas", i, colorGroups[i])
		}
	}
}

func TestSandColorAliasesDoNotMeanSilver(t *testing.T) {
	for _, query := range []string{"qum", "песочный", "sand"} {
		matcher := newColorMatcher(query)
		if !matcher.match("Kum rang", "") {
			t.Fatalf("sand alias not recognized: %s", query)
		}
		if matcher.match("Kumush", "") || matcher.match("Kulrang", "") {
			t.Fatalf("unrelated shade matched: %s", query)
		}
	}
}
