package pii

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestScrub_ExplicitPersonalFields(t *testing.T) {
	cases := []struct{ input, secret string }{
		{"Mening ismim Sinovbek Auditov, telefonim +998901234567; manzilim Sinov ko'chasi 123", "Sinovbek"},
		{"Ismim: O'ktam G'aniyev; karniz 3 metr", "G'aniyev"},
		{"Меня зовут Иван Иванов; мой адрес: Тестовая улица, дом 123", "Иванов"},
		{"Manzil: Toshkent shahri, Sinov ko'chasi 123; karniz 3 m", "Sinov ko'chasi"},
		{"Email: sinovbek.audit@example.test, telefon 90 123 45 67", "sinovbek.audit@"},
	}
	for _, tc := range cases {
		t.Run(tc.secret, func(t *testing.T) {
			got := Scrub(tc.input)
			if strings.Contains(got, tc.secret) || strings.Contains(got, "1234567") {
				t.Fatalf("shaxsiy ma'lumot o'tdi: %s", got)
			}
			if Scrub(got) != got {
				t.Fatalf("niqob qayta buzilmasligi kerak: %s → %s", got, Scrub(got))
			}
		})
	}
}

func TestScrub_KeepsCurtainFactsAndMoney(t *testing.T) {
	for _, text := range []string{
		"Velvet Milano nomi va parvarishini ayting. Karniz 3 m, balandlik 2.6 m.",
		"Mato nomi Office Minimal, 500000000 so'm budjet.",
		"Mening manzilimga yetkazish bormi?", "Какой адрес магазина?",
		"Blackout Duo oq 95 000 so'm/m², 2.8 m rulon.",
		"10000000000 so'm", "Narx 500000000 UZS",
	} {
		if got := Scrub(text); got != text {
			t.Fatalf("sotuv ma'lumoti buzildi: %s → %s", text, got)
		}
	}
}

func TestScrub_StructuredPIIPreservesJSONAndNumericPrecision(t *testing.T) {
	raw := `{"customer_name":"Sinovbek Auditov","customer_phone":"+998901234567","address":"Sinov ko'chasi, 123","email":"audit@example.test","budget_max":500000000,"total":"500000000","id":9007199254740993,"parts":[{"variant_id":"v-901234567","name":"Velvet Milano","width_m":"3"}]}`
	got := ScrubToolResult(raw)
	for _, secret := range []string{"Sinovbek", "Sinov ko'chasi", "audit@example.test", "+998901234567"} {
		if strings.Contains(got, secret) {
			t.Fatalf("JSONda shaxsiy ma'lumot qoldi: %s", got)
		}
	}
	if !json.Valid([]byte(got)) || !strings.Contains(got, "9007199254740993") || !strings.Contains(got, `"total":"500000000"`) || !strings.Contains(got, `"budget_max":500000000`) || !strings.Contains(got, "Velvet Milano") || !strings.Contains(got, `"variant_id":"v-901234567"`) {
		t.Fatalf("JSON yoki narx/aniqlik buzildi: %s", got)
	}
	if ScrubToolResult(got) != got {
		t.Fatal("strukturali maska idempotent emas")
	}
}

func TestScrub_JSONWithoutPIIStaysByteIdentical(t *testing.T) {
	raw := `{ "total": "500000000", "name": "Velvet Milano", "id": 9007199254740993 }`
	if got := ScrubToolResult(raw); got != raw {
		t.Fatalf("shaxsiy maydonsiz JSON qayta formatlanmasin: %s", got)
	}
}
