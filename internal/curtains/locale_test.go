package curtains

import "testing"

func TestMessageLocale(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Здравствуйте, нужны шторы на кухню", "ru"},
		{"Окно 1.5 на 1.4 метра", "ru"},
		{"Хочу Velvet Milano в спальню", "ru"},
		{"Yotoqxonamga parda kerak", ""},
		{"Velvet Milano", ""},
		{"Ассалому алайкум, парда керак, қанча туради?", ""}, // o'zbek kirill
		{"1.5 x 1.4", ""},
		{"да", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := MessageLocale(tt.in); got != tt.want {
			t.Errorf("MessageLocale(%q) = %q, kutilgan %q", tt.in, got, tt.want)
		}
	}
}
