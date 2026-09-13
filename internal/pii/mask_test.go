package pii

import "testing"

func TestMaskPhone(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"+998901234567", "+998****4567"},
		{"998 90 123 45 67", "+998****4567"},
		{"90 123 45 67", "****4567"},
	} {
		if got := MaskPhone(tc.input); got != tc.want {
			t.Errorf("MaskPhone(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestMaskName(t *testing.T) {
	if got := MaskName("Aziz Karimov"); got != "A. K." {
		t.Fatalf("MaskName() = %q", got)
	}
}

func TestScrub(t *testing.T) {
	got := Scrub("Telefon: +998901234567 yoki 90 123 45 67")
	want := "Telefon: +998****4567 yoki ****4567"
	if got != want {
		t.Fatalf("Scrub() = %q, want %q", got, want)
	}
}
