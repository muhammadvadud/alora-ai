package tools

// Bu fayl — tool'larning agent bilan gaplashadigan DTO'lari (kirish/chiqish struct'lari).
// Handler mantiqi alohida fayllarda; struct'lar shu yerda jamlangan.
// jsonschema teglari MAJBURIY — Eino shu orqali tool sxemasini yasaydi.

// ---- calculator ----

type calcArgs struct {
	Op string  `json:"op" jsonschema:"description=amal turi,enum=add,enum=sub,enum=mul,enum=div"`
	A  float64 `json:"a" jsonschema:"description=birinchi son"`
	B  float64 `json:"b" jsonschema:"description=ikkinchi son"`
}

// ---- current_time ----

type timeArgs struct {
	Timezone string `json:"timezone" jsonschema:"description=IANA vaqt mintaqasi, masalan Asia/Tashkent"`
}
