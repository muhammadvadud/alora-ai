package voice

import (
	"testing"
)

func TestNumberToWordsUz(t *testing.T) {
	cases := []struct {
		n        int64
		expected string
	}{
		{0, "nol"},
		{-5, "minus besh"},
		{7, "yetti"},
		{10, "o'n"},
		{11, "o'n bir"},
		{21, "yigirma bir"},
		{99, "to'qson to'qqiz"},
		{100, "yuz"},
		{101, "yuz bir"},
		{250, "ikki yuz ellik"},
		{1000, "ming"},
		{1001, "ming bir"},
		{2000, "ikki ming"},
		{15000, "o'n besh ming"},
		{100000, "yuz ming"},
		{1000000, "bir million"},
		{2500000, "ikki million besh yuz ming"},
		{1234567890, "bir milliard ikki yuz o'ttiz to'rt million besh yuz oltmish yetti ming sakkiz yuz to'qson"},
	}

	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			result := NumberToWordsUz(tc.n)
			if result != tc.expected {
				t.Errorf("NumberToWordsUz(%d) = %q, kutildi %q", tc.n, result, tc.expected)
			}
		})
	}
}

func TestNumberToWordsRu(t *testing.T) {
	cases := []struct {
		n        int64
		expected string
		gender   string
	}{
		{0, "ноль", "m"},
		{-5, "минус пять", "m"},
		{1, "один", "m"},
		{1, "одна", "f"},
		{1, "одно", "n"},
		{2, "два", "m"},
		{2, "две", "f"},
		{5, "пять", "m"},
		{21, "двадцать один", "m"},
		{1000, "одна тысяча", "m"},
		{2000, "две тысячи", "m"},
		{5000, "пять тысяч", "m"},
		{1000000, "один миллион", "m"},
		{2000000, "два миллиона", "m"},
		{5000000, "пять миллионов", "m"},
	}

	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			result := NumberToWordsRu(tc.n, tc.gender)
			if result != tc.expected {
				t.Errorf("NumberToWordsRu(%d, %q) = %q, kutildi %q", tc.n, tc.gender, result, tc.expected)
			}
		})
	}
}

func TestNormalizeForTTS(t *testing.T) {
	cases := []struct {
		name     string
		lang     string
		input    string
		expected string
	}{
		// O'zbek tili holatlari
		{"uz pul", "uz", "2 500 000 so'm", "ikki million besh yuz ming so'm"},
		{"uz pul bo'shliqsiz", "uz", "2500000 so'm", "ikki million besh yuz ming so'm"},
		{"uz pul nbsp", "uz", "2\u00A0500\u00A0000 so'm", "ikki million besh yuz ming so'm"},
		{"uz kasr", "uz", "2.5", "ikki butun besh"},
		{"uz kasr metr", "uz", "4.2 m", "to'rt butun ikki metr"},
		{"uz m2", "uz", "1 m²", "bir kvadrat metr"},
		{"uz m2 alt", "uz", "10 m2", "o'n kvadrat metr"},
		{"uz kv.m", "uz", "15 kv.m", "o'n besh kvadrat metr"},
		{"uz foiz", "uz", "100 %", "yuz foiz"},
		{"uz koeffitsient K=", "uz", "K=2.5", "koeffitsient ikki butun besh"},
		{"uz koeffitsient probel bilan", "uz", "K = 2.5", "koeffitsient ikki butun besh"},
		{"uz koeffitsient so'z bilan", "uz", "koeffitsient 2.5", "koeffitsient ikki butun besh"},
		{"uz qisqartma mln", "uz", "1.5 mln", "bir butun besh million"},
		{"uz qisqartma ming", "uz", "100 ming", "yuz ming"},
		{"uz telefon +998", "uz", "+998901234567", "telefon raqami"},
		{"uz telefon bo'shliqlar bilan", "uz", "+998 90 123 45 67", "telefon raqami"},
		{"uz telefon 90 boshlangan", "uz", "90 123 45 67", "telefon raqami"},
		{"uz aralash jumla", "uz", "Bu mato 1 m² 180 000 so'm, karniz 4.2 m, K=2.5", "Bu mato bir kvadrat metr yuz sakson ming so'm, karniz to'rt butun ikki metr, koeffitsient ikki butun besh"},
		{"uz sm", "uz", "15 sm", "o'n besh santimetr"},
		{"uz noma'lum", "uz", "25 kg", "yigirma besh kg"},
		{"uz oddiy matn o'zgarmas", "uz", "Assalomu alaykum! Yaxshimisiz?", "Assalomu alaykum! Yaxshimisiz?"},
		{"uz tinish belgilari saqlanadi", "uz", "2.5 m, va 1 m².", "ikki butun besh metr, va bir kvadrat metr."},

		// Rus tili holatlari
		{"ru pul", "ru", "2 500 000 сум", "два миллиона пятьсот тысяч сум"},
		{"ru pul bo'shliqsiz", "ru", "2500000 сум", "два миллиона пятьсот тысяч сум"},
		{"ru kasr", "ru", "2.5", "две целых пять"},
		{"ru kasr metr", "ru", "4.2 m", "четыре целых два метра"},
		{"ru m2", "ru", "1 m²", "один квадратный метр"},
		{"ru m2 5 m2", "ru", "5 m²", "пять квадратных метров"},
		{"ru kv.m", "ru", "2 kv.m", "два квадратных метра"},
		{"ru sm", "ru", "15 sm", "пятнадцать сантиметров"},
		{"ru foiz", "ru", "20 %", "двадцать процентов"},
		{"ru koeffitsient", "ru", "K=2.5", "коэффициент две целых пять"},
		{"ru qisqartma mln", "ru", "1.5 mln", "одна целая пять миллиона"},
		{"ru ming qisqartma", "ru", "100 ming", "сто ming"},
		{"ru noma'lum", "ru", "25 kg", "двадцать пять kg"},
		{"ru so'm", "ru", "100 so'm", "сто сум"},
		{"ru telefon +998", "ru", "+998901234567", "telefon raqami"},
		{"ru telefon bo'shliqlar bilan", "ru", "90 123 45 67", "telefon raqami"},
		{"ru aralash jumla", "ru", "Этот материал 1 m² 180 000 сум, карниз 4.2 m, K=2.5", "Этот материал один квадратный метр сто восемьдесят тысяч сум, карниз четыре целых два метра, коэффициент две целых пять"},
		{"ru oddiy matn", "ru", "Привет! Как дела?", "Привет! Как дела?"},

		// Chegaraviy holatlar
		{"bo'sh satr", "uz", "", ""},
		{"faqat bo'shliqlar", "uz", "   ", ""},
		{"ketma-ket bo'shliqlar qisqaradi", "uz", "A    B", "A B"},
		{"ko'p sonlar", "uz", "1, 2, 3.", "bir, ikki, uch."},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := NormalizeForTTS(tc.input, tc.lang)
			if result != tc.expected {
				t.Errorf("NormalizeForTTS(%q, %q) = %q, kutildi %q", tc.input, tc.lang, result, tc.expected)
			}
		})
	}
}
