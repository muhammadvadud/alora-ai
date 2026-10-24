package tools

import "github.com/muhammadvadud/alora-ai/internal/curtains"

// selectOption — mijoz tanlagan paketni (build_options natijasi) keyingi taklif va
// qoralama uchun asos qiladi. Bo'sh tier — hech narsa o'zgarmaydi (oxirgi hisob).
// Nega: model paket tanlanganda variant ID'larini qo'lda ko'chirib boshqa hisob
// qilardi va "premium" deb aytilgan qoralama boshqa summada chiqardi (ADR-016).
func selectOption(st *curtains.SessionState, tier string) string {
	if tier == "" {
		return ""
	}
	t := curtains.OptionTier(tier)
	switch t {
	case curtains.OptionEconomy, curtains.OptionStandard, curtains.OptionPremium:
	default:
		return "option_tier economy, standard yoki premium bo'lishi kerak"
	}
	if st.SelectedOption == t && (st.LastQuote != nil || st.LastCalc != nil) {
		return ""
	}
	snap, ok := st.LastOptions[t]
	if !ok {
		return "bu paket hali tuzilmagan — avval build_options chaqir"
	}
	res := snap.Result
	st.LastCalc = &res
	// Oldingi taklif boshqa hisobga tegishli — tanlangan paket ustida qaytadan quriladi.
	st.LastQuote = nil
	st.SelectedOption = t
	st.Stage = curtains.StageChoose
	return ""
}
