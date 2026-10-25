package curtains

import "slices"

// invalidateChangedQuote — yangi talab eski paket/qoralamaga yashirin o'tmasin.
// Budjet o'zgarsa material hisobi saqlanadi, lekin taklif qayta tekshiriladi.
func (s *SessionState) invalidateChangedQuote(before SessionState) {
	dimensions := !s.CorniceLengthM.Equal(before.CorniceLengthM) || !s.HeightM.Equal(before.HeightM)
	materialChanged := dimensions ||
		s.Room != before.Room || s.Style != before.Style || s.Pattern != before.Pattern ||
		s.Transparency != before.Transparency || !slices.Equal(s.Colors, before.Colors) ||
		s.Construction != before.Construction || s.LightPreference != before.LightPreference
	changed := materialChanged || !s.BudgetMin.Equal(before.BudgetMin) || !s.BudgetMax.Equal(before.BudgetMax)
	if !changed || (before.LastCalc == nil && before.LastQuote == nil && len(before.LastOptions) == 0) {
		return
	}
	s.LastOptions = nil
	s.SelectedOption = ""
	s.LastQuote = nil
	s.QuoteNeedsReview = true
	s.Stage = StageMeasure
	if materialChanged {
		s.LastCalc = nil
		s.Windows = nil
		s.Parts = nil
	}
}
