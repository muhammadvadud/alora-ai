package curtains

import (
	"fmt"
	"strings"
)

// PromptBlock hozirgi holatning qisqacha ko'rinishi (<= 400 belgi).
func (s SessionState) PromptBlock() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("bosqich=%s", s.Stage))
	if s.QuoteNeedsReview {
		parts = append(parts, "oldingi taklif eskirgan; yangilangan talab bilan qayta hisob kerak")
	}
	if s.SelectedOption != "" {
		parts = append(parts, fmt.Sprintf("tanlangan_paket=%s", s.SelectedOption))
	}
	if s.Stage == StageOrder {
		parts = append(parts, "qoralama tayyor; mijoz ma'lumotlari ilova formasida; tasdiq sotuvchida")
	}
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
		"stage":              string(s.Stage),
		"quote_needs_review": s.QuoteNeedsReview,
		"budget_max":         s.BudgetMax,
	}
	if s.SelectedOption != "" {
		res["selected_option"] = s.SelectedOption
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
