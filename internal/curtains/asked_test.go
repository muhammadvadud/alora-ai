package curtains

import "testing"

// B4 (ADR-020): bitta maydonni 2 marta taklif qilish mumkin, uchinchi marta — yo'q.
//
// 2026-09-28 live: model `room` maydonini takror-takror so'rab, mijoz javob
// bermaganda ham siklda qolib ketdi. Chegara ikki savol bilan: mijoz javob
// bersa maydon to'ladi, bermasa — o'tkazib yuboriladi va keyingi maydonga o'tiladi.
func TestSessionState_NextQuestionRespectsAskedLimit(t *testing.T) {
	s := NewSessionState()
	s.Stage = StageRecommend

	q1, _ := s.NextQuestion()
	if q1 != "room" {
		t.Fatalf("birinchi savol room bo'lishi kerak: %q", q1)
	}
	s.MarkAsked(q1)

	q2, _ := s.NextQuestion()
	if q2 != "room" {
		t.Fatalf("ikkinchi savol ham room bo'lishi kerak: %q", q2)
	}
	s.MarkAsked(q2)

	// uchinchi marta room takrorlanmaydi — keyingi maydonga o'tiladi
	q3, hint3 := s.NextQuestion()
	if q3 == "room" {
		t.Fatal("room uchinchi marta takrorlandi")
	}
	if q3 != "cornice_length_m" || hint3 == "" {
		t.Fatalf("o'lchamga o'tilishi kerak: %q/%q", q3, hint3)
	}
	s.MarkAsked(q3)

	// o'lcham maydonlari ketma-ket, har biri 2 marta
	q4, _ := s.NextQuestion()
	if q4 != "cornice_length_m" {
		t.Fatalf("karniz ikkinchi marta berilishi kerak: %q", q4)
	}
	s.MarkAsked(q4)

	q5, _ := s.NextQuestion()
	if q5 != "height_m" {
		t.Fatalf("karniz chegaraga yetdi — balandlikka o'tilishi kerak: %q", q5)
	}
	s.MarkAsked(q5)

	// balandlikning ikkinchi berilishi
	if q6, _ := s.NextQuestion(); q6 != "height_m" {
		t.Fatalf("balandlik ikkinchi marta berilishi kerak: %q", q6)
	}
	s.MarkAsked("height_m")

	if q7, hint7 := s.NextQuestion(); q7 != "" || hint7 != "" {
		t.Fatalf("hammasi 2 marta so'ralgan bo'lishi kerak: %q/%q", q7, hint7)
	}
}

// B4: maydon to'ldirilsa, u qayta taklif qilinmaydi (bu avvalgi xatti-harakat).
func TestSessionState_FilledFieldNotAskedAgain(t *testing.T) {
	s := NewSessionState()
	s.Stage = StageRecommend
	s.Room = "living"

	q, hint := s.NextQuestion()
	if q == "room" {
		t.Fatal("to'ldirilgan room qayta taklif qilinmasligi kerak")
	}
	if q != "cornice_length_m" || hint == "" {
		t.Fatalf("o'lchamga o'tilishi kerak: %q/%q", q, hint)
	}
}

// B4: boshqa maydon to'ldirilgan bo'lsa, chegara tegmagan room yana ikki marta beriladi.
func TestSessionState_AskedLimitIsPerField(t *testing.T) {
	s := NewSessionState()
	s.Stage = StageRecommend
	s.LightPreference = "qorong'iroq"
	s.Pattern = "plain"
	s.Style = "modern"

	s.MarkAsked("room")
	q, _ := s.NextQuestion()
	if q != "room" {
		t.Fatalf("room o'z chegarasiga yetmagan bo'lishi kerak: %q", q)
	}
}

// B4: MarkAsked bo'sh maydonni hisobga olmaydi (NextQuestion savol bermaganda
// "" qaytaradi — sanovchi osilmasin).
func TestSessionState_MarkAskedIgnoresEmptyField(t *testing.T) {
	s := NewSessionState()
	s.MarkAsked("")
	s.MarkAsked("nope")
	if s.AskedCount("") != 0 {
		t.Fatal("bo'sh maydon sanovchiga yozilmasligi kerak")
	}
	if s.AskedCount("nope") != 1 {
		t.Fatal("nomа'lum maydon sanovchisi saqlanishi kerak")
	}
}
