package curtains

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Stage parda tanlash jarayoni bosqichi.
type Stage string

const (
	StageNeed      Stage = "need"
	StageRecommend Stage = "recommend"
	StageMeasure   Stage = "measure"
	StageCalc      Stage = "calc"
	StageVariants  Stage = "variants"
	StageChoose    Stage = "choose"
	StageOrder     Stage = "order"
)

// Backend Django uchun bosqich nomini qaytaradi (choose o'rniga variants).
func (s Stage) Backend() string {
	if s == StageChoose {
		return "variants"
	}
	return string(s)
}

// PartSpec bitta parda bo'lagi (mato) xususiyatlari.
type PartSpec struct {
	Role      PartRole        `json:"role"`
	VariantID string          `json:"variant_id"`
	WidthM    decimal.Decimal `json:"width_m"`
	HeightM   decimal.Decimal `json:"height_m"`
	Coef      decimal.Decimal `json:"coef"`
}

// WindowSpec — bitta deraza o'lchovi. curtain_calculate bir nechta o'chovni
// windows[] orqali qabul qiladi; natijalar (price_quote, order_draft) shu
// ko'rinishda qaytadi, shuning uchun xona nomi va qty hisobdan chiqmaydi (ADR-016).
type WindowSpec struct {
	RoomName string          `json:"room_name,omitempty"`
	LengthM  decimal.Decimal `json:"length_m"`
	HeightM  decimal.Decimal `json:"height_m"`
	Qty      int             `json:"qty,omitempty"`
	Parts    int             `json:"parts,omitempty"`
}

// OptionSnapshot — build_options'ning bitta paketi: Django'dan kelgan CalcResult.
// Mijoz paketni tanlaganda price_quote/order_draft shu natijani asos qiladi —
// aks holda oxirgi curtain_calculate olinib, "premium" qoralama boshqa summada
// chiqardi (2026-09-28 live, ADR-016). CalcInput saqlanmaydi: uning JSON'i Django
// formatida (faqat MarshalJSON), holat Postgres'dan qaytganda tiklanmasdi —
// kerak bo'lsa kirish natijadan rebuildInput bilan quriladi (LastCalc kabi).
type OptionSnapshot struct {
	Result CalcResult `json:"result"`
}

// SessionState dialogning joriy holati.
type SessionState struct {
	Stage           Stage                         `json:"stage,omitempty"`
	Room            string                        `json:"room,omitempty"`
	Style           string                        `json:"style,omitempty"`
	Pattern         string                        `json:"pattern,omitempty"`
	Transparency    string                        `json:"transparency,omitempty"`
	Colors          []string                      `json:"colors,omitempty"`
	LightPreference string                        `json:"light_preference,omitempty"`
	Construction    string                        `json:"construction,omitempty"`
	BudgetMin       decimal.Decimal               `json:"budget_min,omitempty"`
	BudgetMax       decimal.Decimal               `json:"budget_max,omitempty"`
	CorniceLengthM  decimal.Decimal               `json:"cornice_length_m,omitempty"`
	HeightM         decimal.Decimal               `json:"height_m,omitempty"`
	Coefficient     decimal.Decimal               `json:"coefficient,omitempty"`
	Layers          []string                      `json:"layers,omitempty"`
	Windows         []WindowSpec                  `json:"windows,omitempty"`
	Parts           []PartSpec                    `json:"parts,omitempty"`
	ChosenVariants  []string                      `json:"chosen_variants,omitempty"`
	LastCalc        *CalcResult                   `json:"last_calc,omitempty"`
	LastQuote       *CalcResult                   `json:"last_quote,omitempty"`
	LastOptions     map[OptionTier]OptionSnapshot `json:"last_options,omitempty"`
	// EmptySearches — ketma-ket bo'sh qidiruvlar soni. Model qidiruvni
	// takrorlab siklda qolmasligi kerak (ADR-020): 2-marta bo'sh natija
	// kelganda product_search `stop` bilan javob beradi.
	EmptySearches int            `json:"empty_searches,omitempty"`
	Asked         map[string]int `json:"asked,omitempty"`
	UpdatedAt     time.Time      `json:"updated_at,omitempty"`
}

// maxAskedQuestions — bitta maydonni qancha marta taklif qilish mumkin.
// 2-martadan keyin NextQuestion keyingi maydonga o'tadi: model bir savolni
// takror-takror so'rab, mijozni charchatmasligi kerak (ADR-020).
const maxAskedQuestions = 2

// NoteEmptySearch — bo'sh qidiruvni qayd etadi, natijada ketma-ket bo'sh
// qidiruvlar SONINI qaytaradi. Natija topilganda hisob nolga qaytadi.
func (s *SessionState) NoteEmptySearch() int {
	s.EmptySearches++
	return s.EmptySearches
}

// ResetEmptySearches — topilgan natija yoki boshqa tool: sikl urishidan oldin
// hisobni tozalaydi.
func (s *SessionState) ResetEmptySearches() { s.EmptySearches = 0 }

// SearchExhausted — takrorlanadigan bo'sh qidiruv chegarasi (ADR-020).
func (s *SessionState) SearchExhausted() bool { return s.EmptySearches >= maxAskedQuestions }

// MarkAsked — maydonni "so'ralgan" deb belgilaydi (NextQuestion orqali chiqqan savol).
func (s *SessionState) MarkAsked(field string) {
	if field == "" {
		return
	}
	if s.Asked == nil {
		s.Asked = map[string]int{}
	}
	s.Asked[field]++
}

// AskedCount — maydonni necha marta taklif qilganimiz.
func (s SessionState) AskedCount(field string) int { return s.Asked[field] }

// OrderDraft — Django'ga yuboriladigan buyurtma qoralamasi (rules §5.8). Faqat
// mijoz ma'lumotlari va itemlar; session/user/tenant ctx'dan to'ldiriladi (backend).
type OrderDraft struct {
	CustomerName  string
	CustomerPhone string
	Address       string
	ItemsJSON     string
	Note          string
}

// OrderDraftResult — SaveOrderDraft javobi. Telefon/ism YO'Q (rules §1.6, §16.3) —
// sotuvchiga faqat qisqa referens qaytadi.
type OrderDraftResult struct {
	OrderID     string
	OrderNumber string
}

// NewSessionState boshlang'ich holatni qaytaradi (need).
func NewSessionState() SessionState {
	return SessionState{
		Stage: StageNeed,
	}
}

// toDecimal kiritilgan qiymatni Decimal'ga o'tkazadi (float64, string, json.Number, int).
func toDecimal(v any) decimal.Decimal {
	if v == nil {
		return decimal.Zero
	}
	switch val := v.(type) {
	case float64:
		return decimal.NewFromFloat(val)
	case float32:
		return decimal.NewFromFloat(float64(val))
	case int:
		return decimal.NewFromInt(int64(val))
	case int32:
		return decimal.NewFromInt(int64(val))
	case int64:
		return decimal.NewFromInt(val)
	case string:
		d, err := decimal.NewFromString(val)
		if err == nil {
			return d
		}
	case json.Number:
		d, err := decimal.NewFromString(string(val))
		if err == nil {
			return d
		}
	}
	return decimal.Zero
}

// toString yordamchi funksiya.
func toString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// Apply tool chaqiruvidan holatni yangilaydi (faqat oldinga siljiydi).
func (s *SessionState) Apply(tool string, args map[string]any, now time.Time) {
	s.UpdatedAt = now

	stageOrder := map[Stage]int{
		StageNeed:      0,
		StageRecommend: 1,
		StageMeasure:   2,
		StageCalc:      3,
		StageVariants:  4,
		StageChoose:    5,
		StageOrder:     6,
	}

	advanceStage := func(target Stage) {
		if stageOrder[s.Stage] < stageOrder[target] {
			s.Stage = target
		}
	}

	switch tool {
	case "set_context":
		// O'lcham suhbatda aytilganda ham state'ga tushadi (2026-09-14 live: "yozib
		// oldim" deb hech qayerga yozilmagan, build_options "o'lcham yo'q" degan).
		if v, ok := args["cornice_length_m"]; ok && toDecimal(v).IsPositive() {
			s.CorniceLengthM = toDecimal(v)
			advanceStage(StageMeasure)
		}
		if v, ok := args["height_m"]; ok && toDecimal(v).IsPositive() {
			s.HeightM = toDecimal(v)
			advanceStage(StageMeasure)
		}
		if v, ok := args["room"]; ok {
			s.Room = toString(v)
		}
		if v, ok := args["style"]; ok {
			s.Style = toString(v)
		}
		if v, ok := args["pattern"]; ok {
			s.Pattern = toString(v)
		}
		if v, ok := args["transparency"]; ok {
			s.Transparency = toString(v)
		}
		if v, ok := args["colors"]; ok {
			switch c := v.(type) {
			case string:
				if c != "" {
					parts := strings.Split(c, ",")
					for i := range parts {
						parts[i] = strings.TrimSpace(parts[i])
					}
					s.Colors = parts
				} else {
					s.Colors = nil
				}
			case []interface{}:
				s.Colors = make([]string, 0, len(c))
				for _, item := range c {
					s.Colors = append(s.Colors, toString(item))
				}
			case []string:
				s.Colors = c
			}
		}
		if v, ok := args["light"]; ok {
			s.LightPreference = toString(v)
		}
		if v, ok := args["construction"]; ok {
			s.Construction = toString(v)
		}
		if v, ok := args["budget_min"]; ok {
			s.BudgetMin = toDecimal(v)
		}
		if v, ok := args["budget_max"]; ok {
			s.BudgetMax = toDecimal(v)
		}
	case "product_search":
		// product_search -> room/style/pattern/transparency (agar bo'sh bo'lmasa)
		advanceStage(StageRecommend)
		if v, ok := args["room"]; ok && toString(v) != "" {
			s.Room = toString(v)
		}
		if v, ok := args["style"]; ok && toString(v) != "" {
			s.Style = toString(v)
		}
		if v, ok := args["pattern"]; ok && toString(v) != "" {
			s.Pattern = toString(v)
		}
		if v, ok := args["transparency"]; ok && toString(v) != "" {
			s.Transparency = toString(v)
		}
	case "curtain_calculate":
		advanceStage(StageCalc)
		if v, ok := args["cornice_length_m"]; ok && toDecimal(v).IsPositive() {
			s.CorniceLengthM = toDecimal(v)
		}
		if v, ok := args["height_m"]; ok && toDecimal(v).IsPositive() {
			s.HeightM = toDecimal(v)
		}
		// windows[] — bir nechta o'chov; bo'lmasa eski top-level kiritim bir deraza
		// deb qaraladi. Natija bo'laklari shu o'chovlarga tegishli, shuning uchun
		// keyingi hisob (price_quote/order_draft) xona nomini ham saqlashi kerak.
		if v, ok := args["windows"]; ok {
			s.Windows = windowSpecsFrom(v)
		}
		if v, ok := args["parts"]; ok {
			if parts := partsFrom(v); parts != nil {
				s.Parts = parts
			}
		} else if len(s.Windows) > 0 {
			// windows[] bo'laklari oyna ichida; holat birinchi oynaning bo'laklarini
			// saqlaydi (Summary uchun), to'liq hisob esa natija va snapshot'da.
			s.Parts = firstWindowParts(args["windows"])
		}
		if len(s.Windows) == 0 && s.CorniceLengthM.IsPositive() && s.HeightM.IsPositive() {
			s.Windows = []WindowSpec{{
				LengthM: s.CorniceLengthM,
				HeightM: s.HeightM,
				Parts:   len(s.Parts),
			}}
		}
	case "build_options":
		advanceStage(StageVariants)
	case "order_draft":
		if v, ok := args["action"]; ok && toString(v) == "confirm" {
			advanceStage(StageOrder)
		}
	}
}

// partsFrom — args["parts"] ro'yxatini PartSpec'ga aylantiradi (bo'sh/bo'lmagan — nil).
func partsFrom(v any) []PartSpec {
	list, ok := v.([]interface{})
	if !ok || len(list) == 0 {
		return nil
	}
	out := make([]PartSpec, 0, len(list))
	for _, item := range list {
		pm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		out = append(out, PartSpec{
			VariantID: toString(pm["variant_id"]),
			Role:      PartRole(toString(pm["role"])),
			WidthM:    toDecimal(pm["width_m"]),
			HeightM:   toDecimal(pm["height_m"]),
			Coef:      toDecimal(pm["coefficient"]),
		})
	}
	return out
}

// firstWindowParts — windows[0].parts: ko'p oynali hisobda holat birinchi oynaning
// bo'laklarini saqlaydi, qolganlari hisob natijasida qoladi.
func firstWindowParts(v any) []PartSpec {
	list, ok := v.([]interface{})
	if !ok || len(list) == 0 {
		return nil
	}
	wm, ok := list[0].(map[string]interface{})
	if !ok {
		return nil
	}
	return partsFrom(wm["parts"])
}

// windowSpecsFrom — args["windows"] ro'yxatini WindowSpec'ga aylantiradi. Model
// odatda qisqa yozadi, shuning uchun qty bo'lmasa 1, parts bo'lmasa 0.
func windowSpecsFrom(v any) []WindowSpec {
	list, ok := v.([]interface{})
	if !ok || len(list) == 0 {
		return nil
	}
	out := make([]WindowSpec, 0, len(list))
	for _, item := range list {
		wm, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		w := WindowSpec{
			RoomName: toString(wm["room_name"]),
			LengthM:  toDecimal(wm["cornice_length_m"]),
			HeightM:  toDecimal(wm["height_m"]),
			Qty:      1,
		}
		if q := toDecimal(wm["qty"]); q.IsPositive() {
			w.Qty = int(q.InexactFloat64())
		}
		if parts, ok := wm["parts"].([]interface{}); ok {
			w.Parts = len(parts)
		}
		out = append(out, w)
	}
	return out
}

// SetStage orqaga qaytish imkoniyati bilan bosqichni aniq belgilash.
func (s *SessionState) SetStage(stage Stage) {
	s.Stage = stage
}

// NextQuestion joriy bosqich (Stage) bo'yicha yetishmayotgan birinchi maydon va o'zbekcha
// yordamchini qaytaradi (rules §4.5.4). Ehtiyoj savollari faqat `need` bosqichida; o'lcham
// kelgach (`measure`) faqat xona va o'lcham, hisobdan keyin — savol yo'q. Ilgari bosqich
// hisobga olinmasdi: o'lcham va hisob tayyor bo'lsa ham "gulli/gulsiz" qayta-qayta
// so'ralib, "eng arzonini" degan mijozga narx kechikdi (2026-09-27 live).
//
// Maydonga 2 marta taklif qilingandan keyin u o'tkazib yuboriladi (ADR-020): mijoz
// javob bermasa, model bir savolni takrorlab siklda qolmasin.
func (s *SessionState) NextQuestion() (field, hint string) {
	switch s.Stage {
	case StageCalc, StageVariants, StageChoose, StageOrder:
		return "", ""
	case StageRecommend, StageMeasure:
		return s.nextBelow(maxAskedQuestions, []string{"room"}, s.nextMeasurementHint)
	}
	return s.nextBelow(maxAskedQuestions, []string{
		"room", "light", "pattern", "style", "colors", "construction", "budget",
	}, s.nextMeasurementHint)
}

// nextBelow — maydonlar ro'yxatidan birinchisini qaytaradi: allaqachon to'ldirilgan
// yoki `max` marta taklif qilingan maydonlarni o'tkazadi.
func (s *SessionState) nextBelow(max int, fields []string, last func() (string, string)) (field, hint string) {
	for _, f := range fields {
		if s.isFilled(f) || s.AskedCount(f) >= max {
			continue
		}
		return f, s.questionHint(f)
	}
	return last()
}

// isFilled — maydon allaqachon aniqlanganmi.
func (s *SessionState) isFilled(field string) bool {
	switch field {
	case "room":
		return s.Room != ""
	case "light":
		return s.LightPreference != ""
	case "pattern":
		return s.Pattern != ""
	case "style":
		return s.Style != ""
	case "colors":
		return len(s.Colors) > 0
	case "construction":
		return s.Construction != ""
	case "budget":
		return !s.BudgetMin.IsZero() || !s.BudgetMax.IsZero()
	}
	return false
}

// questionHint — maydon nomidan tushunarli o'zbekcha savol matni.
func (s *SessionState) questionHint(field string) string {
	switch field {
	case "room":
		return "xona turi"
	case "light":
		return "yorug'lik va deraza tomoni"
	case "pattern":
		return "gulli/gulsiz"
	case "style":
		return "uslub"
	case "colors":
		return "xona ranglari"
	case "construction":
		return "deraza konstruksiyasi: rom, balkon eshigi, karniz"
	case "budget":
		return "budjet"
	}
	return ""
}

// nextMeasurementHint — o'lcham maydonlari ham xuddi shu chegaraga obeydi
// (ADR-020): karniz uzunligi 2 marta so'ralgan bo'lsa, uchinchi marta
// takrorlanmaydi — aks holda o'lchamga qarab siklda qolib ketardi.
func (s *SessionState) nextMeasurementHint() (field, hint string) {
	if s.CorniceLengthM.IsZero() && s.AskedCount("cornice_length_m") < maxAskedQuestions {
		return "cornice_length_m", "karniz uzunligi"
	}
	if s.HeightM.IsZero() && s.AskedCount("height_m") < maxAskedQuestions {
		return "height_m", "balandlik"
	}
	return "", ""
}

// PromptBlock hozirgi holatning qisqacha ko'rinishi (<= 400 belgi).
func (s SessionState) PromptBlock() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("bosqich=%s", s.Stage))
	if s.Room != "" {
		parts = append(parts, fmt.Sprintf("xona=%s", s.Room))
	}
	if s.Style != "" {
		parts = append(parts, fmt.Sprintf("uslub=%s", s.Style))
	}
	// Ehtiyoj maydonlari ham kiradi: trimHistory eski xabarlarni kesganda model mijozning
	// blackout/rang talabini faqat shu blokdan biladi (TZ §5 — state kontekstga tayanmaydi).
	if s.Pattern != "" {
		parts = append(parts, fmt.Sprintf("bezak=%s", s.Pattern))
	}
	if s.Transparency != "" {
		parts = append(parts, fmt.Sprintf("shaffoflik=%s", s.Transparency))
	}
	if s.LightPreference != "" {
		parts = append(parts, fmt.Sprintf("yorug'lik=%s", s.LightPreference))
	}
	if len(s.Colors) > 0 {
		parts = append(parts, fmt.Sprintf("ranglar=%s", strings.Join(s.Colors, ",")))
	}
	if !s.BudgetMin.IsZero() || !s.BudgetMax.IsZero() {
		parts = append(parts, fmt.Sprintf("budjet=%s–%s", s.BudgetMin.String(), s.BudgetMax.String()))
	}
	if !s.CorniceLengthM.IsZero() {
		parts = append(parts, fmt.Sprintf("karniz=%s m", s.CorniceLengthM.String()))
	}
	if !s.HeightM.IsZero() {
		parts = append(parts, fmt.Sprintf("balandlik=%s m", s.HeightM.String()))
	}
	if len(s.Layers) > 0 {
		parts = append(parts, fmt.Sprintf("qatlamlar=%s", strings.Join(s.Layers, ",")))
	}

	res := "Hozirgi holat: " + strings.Join(parts, "; ")
	runes := []rune(res)
	if len(runes) > 400 {
		return string(runes[:399]) + "…"
	}
	return res
}

// Summary KindStage event result uchun JSON kalitlari.
func (s SessionState) Summary() map[string]any {
	res := map[string]any{
		"stage": string(s.Stage),
	}
	if s.Room != "" {
		res["room"] = s.Room
	}
	if s.Style != "" {
		res["style"] = s.Style
	}
	if !s.CorniceLengthM.IsZero() {
		res["cornice_length_m"] = s.CorniceLengthM
	}
	if !s.HeightM.IsZero() {
		res["height_m"] = s.HeightM
	}
	if len(s.Parts) > 0 {
		res["parts"] = len(s.Parts)
	}
	if s.LastCalc != nil {
		res["last_total"] = s.LastCalc.Total
	}
	return res
}
