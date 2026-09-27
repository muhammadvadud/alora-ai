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

// SessionState dialogning joriy holati.
type SessionState struct {
	Stage           Stage           `json:"stage,omitempty"`
	Room            string          `json:"room,omitempty"`
	Style           string          `json:"style,omitempty"`
	Pattern         string          `json:"pattern,omitempty"`
	Transparency    string          `json:"transparency,omitempty"`
	Colors          []string        `json:"colors,omitempty"`
	LightPreference string          `json:"light_preference,omitempty"`
	Construction    string          `json:"construction,omitempty"`
	BudgetMin       decimal.Decimal `json:"budget_min,omitempty"`
	BudgetMax       decimal.Decimal `json:"budget_max,omitempty"`
	CorniceLengthM  decimal.Decimal `json:"cornice_length_m,omitempty"`
	HeightM         decimal.Decimal `json:"height_m,omitempty"`
	Coefficient     decimal.Decimal `json:"coefficient,omitempty"`
	Layers          []string        `json:"layers,omitempty"`
	Parts           []PartSpec      `json:"parts,omitempty"`
	ChosenVariants  []string        `json:"chosen_variants,omitempty"`
	LastCalc        *CalcResult     `json:"last_calc,omitempty"`
	LastQuote       *CalcResult     `json:"last_quote,omitempty"`
	UpdatedAt       time.Time       `json:"updated_at,omitempty"`
}

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
		if v, ok := args["cornice_length_m"]; ok {
			s.CorniceLengthM = toDecimal(v)
		}
		if v, ok := args["height_m"]; ok {
			s.HeightM = toDecimal(v)
		}
		if v, ok := args["parts"]; ok {
			if partsList, ok := v.([]interface{}); ok {
				s.Parts = make([]PartSpec, 0, len(partsList))
				for _, partItem := range partsList {
					if pm, ok := partItem.(map[string]interface{}); ok {
						spec := PartSpec{
							VariantID: toString(pm["variant_id"]),
							Role:      PartRole(toString(pm["role"])),
							WidthM:    toDecimal(pm["width_m"]),
							HeightM:   toDecimal(pm["height_m"]),
							Coef:      toDecimal(pm["coefficient"]),
						}
						s.Parts = append(s.Parts, spec)
					}
				}
			}
		}
	case "build_options":
		advanceStage(StageVariants)
	case "order_draft":
		if v, ok := args["action"]; ok && toString(v) == "confirm" {
			advanceStage(StageOrder)
		}
	}
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
func (s *SessionState) NextQuestion() (field, hint string) {
	switch s.Stage {
	case StageCalc, StageVariants, StageChoose, StageOrder:
		return "", ""
	case StageRecommend, StageMeasure:
		if s.Room == "" {
			return "room", "xona turi"
		}
		return s.nextMeasurement()
	}
	if s.Room == "" {
		return "room", "xona turi"
	}
	if s.LightPreference == "" {
		return "light", "yorug'lik va deraza tomoni"
	}
	if s.Pattern == "" {
		return "pattern", "gulli/gulsiz"
	}
	if s.Style == "" {
		return "style", "uslub"
	}
	if len(s.Colors) == 0 {
		return "colors", "xona ranglari"
	}
	if s.Construction == "" {
		return "construction", "deraza konstruksiyasi: rom, balkon eshigi, karniz"
	}
	if s.BudgetMin.IsZero() && s.BudgetMax.IsZero() {
		return "budget", "budjet"
	}
	return s.nextMeasurement()
}

func (s *SessionState) nextMeasurement() (field, hint string) {
	if s.CorniceLengthM.IsZero() {
		return "cornice_length_m", "karniz uzunligi"
	}
	if s.HeightM.IsZero() {
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
