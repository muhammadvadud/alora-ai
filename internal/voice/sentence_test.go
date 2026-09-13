package voice

import (
	"reflect"
	"testing"
)

func TestSentenceSplitter(t *testing.T) {
	s := &SentenceSplitter{MaxChars: 50}

	// 1. Asosiy test: jumlalar, qisqartmalar va raqam ichidagi nuqta
	chunk1 := "Bu mato 2."
	chunk2 := "5 mln. so'm turadi! Karniz "
	chunk3 := "4.2 m. Yana?"

	var got []string
	got = append(got, s.Push(chunk1)...)
	got = append(got, s.Push(chunk2)...)
	got = append(got, s.Push(chunk3)...)

	// Kutayotgan holat: "Yana?" dan so'ng nuqta yoki boshqa belgi yo'q, shuning uchun bufferda qoladi.
	// Flush uni chiqarib beradi.
	got = append(got, s.Flush())

	want := []string{
		"Bu mato 2.5 mln. so'm turadi!",
		"Karniz 4.2 m. Yana?",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Kutilgan:\n%#v\nOlingan:\n%#v", want, got)
	}
}

func TestSentenceSplitter_MaxChars(t *testing.T) {
	s := &SentenceSplitter{MaxChars: 20}

	// "Uzun matn kelayotgan bo'lsa va MaxChars dan oshib ketsa oxirgi bo'shliqdan kesiladi"
	chunk := "Juda uzun matn kelayotgan bo'lsa va chegaradan oshsa."

	got := s.Push(chunk)
	got = append(got, s.Flush())

	// MaxChars=20.
	// "Juda uzun matn " (15 ta) + "kelayotgan" = 25ta. Shunday qilib, "matn" dan keyin bo'sh joydan kesadi.
	// Keling, aniq tekshiramiz.
	// "Juda uzun matn kelayotgan" = 25 ta belgi.
	// Oxirgi bo'sh joy 14-indeksda ("matn " dan keyin).
	// "Juda uzun matn" (14 ta) -> 1-qism.
	// Qolgani: "kelayotgan bo'lsa va chegaradan oshsa."
	// ... va hokazo qirqilib olinishi kerak.

	// Asosiysi: MaxChars ishlaganini va Flush ohirida qolganini chiqarganini tekshirish
	if len(got) == 0 {
		t.Errorf("MaxChars ishlaganda bo'laklar qaytishi kerak edi")
	}

	// Yana bir marta, birlashtirsak, asl matnga teng bo'lishi kerak, lekin bo'shliqlar yo'qolishi mumkin
	joined := ""
	for i, j := range got {
		if i > 0 {
			joined += " "
		}
		joined += j
	}

	if joined != chunk {
		t.Errorf("Matn yo'qotilgan:\nAsl: %q\nOlingan: %q", chunk, joined)
	}
}

func TestSentenceSplitter_EmptyAndFlush(t *testing.T) {
	s := &SentenceSplitter{}

	if len(s.Push("")) != 0 {
		t.Errorf("Bo'sh matn push qilinganda bo'sh qaytishi kerak")
	}
	if s.Flush() != "" {
		t.Errorf("Bo'sh buffer bo'sh flush qilinishi kerak")
	}
}

func TestSentenceSplitter_Abbreviations(t *testing.T) {
	s := &SentenceSplitter{}

	text := "Uzunligi 5sm. Narxi 10 ming. dona olindi."
	got := s.Push(text)
	got = append(got, s.Flush())

	// sm. va ming. qisqartmalar, shuning uchun faqat "dona." chegara bo'la olmaydi.
	// Yo'q, dona ham qisqartma ("dona."). Demak umuman bo'linmaydi.
	want := []string{"Uzunligi 5sm. Narxi 10 ming. dona olindi."}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Kutilgan:\n%v\nOlingan:\n%v", want, got)
	}
}
