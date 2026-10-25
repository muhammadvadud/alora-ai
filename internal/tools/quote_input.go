package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

// prepareQuoteInput — hisobni takrorlamasdan Django kirishini tayyorlaydi.
func prepareQuoteInput(ctx context.Context, v *curtains.View, st curtains.SessionState, a priceQuoteArgs) (curtains.CalcInput, []string, string) {
	// Fail-closed: rol noma'lum yoki bo'sh bo'lsa chegirma yo'q — employee kabi
	// qabul qilinsa ham baribir, lekin taxmin qilish o'rniga aniq aytiladi.
	// (userRole emas — u noma'lumni employee'ga tushiradi, bu yerda Valid kerak.)
	u, ok := curtains.UserFromContext(ctx)
	if !ok || !u.Role.Valid() {
		return curtains.CalcInput{}, nil, errUnknownRole
	}
	role := u.Role
	discount := dec(a.DiscountPct)
	if policy, ok := v.DiscountPolicy(); ok && a.DiscountPct > 0 {
		max := policy.ManualMaxPercentManager
		if role == curtains.RoleEmployee {
			max = policy.ManualMaxPercentEmployee
		}
		if discount.GreaterThan(max) {
			// Django jim kesadi — AI esa sotuvchiga chegarani aniq aytishi kerak.
			return curtains.CalcInput{}, nil, fmt.Sprintf("chegirma chegarasi %s%% — %s%% berib bo'lmaydi",
				max.StringFixed(0), discount.StringFixed(0))
		}
	}

	in := rebuildInput(st.LastCalc, role)
	in.Delivery = a.NeedDelivery
	in.DiscountPercent = discount

	// Xizmatlar faqat katalogdan (ServicePrice): model bergan yoki standart kalit
	// katalogda bo'lmasa u hisobga yuborilmaydi — ogohlantirish bilan. Aks holda
	// Django "'x' xizmati topilmadi" deb butun hisobni rad etadi.
	requested := services(a)
	if a.Services == nil {
		base := st.LastQuote
		if base == nil {
			base = st.LastCalc
		}
		for _, service := range base.Services {
			if !service.IsDelivery {
				requested = append(requested, service.Key)
			}
		}
	}
	if a.NeedCornice {
		requested = append(requested, "cornice")
	}
	requested = normalizeServiceKeys(requested)
	var warnings []string
	in.Services = []string{}
	var missing []string
	for _, key := range requested {
		if _, ok := v.Service(key); ok {
			in.Services = append(in.Services, key)
		} else {
			missing = append(missing, key)
		}
	}
	warnings = append(warnings, missingServiceWarnings(missing)...)

	// Avans foizi payload bilan Django'ga ketadi — summani Django hisoblaydi (rules §0.2).
	if a.PrepaymentPct > 0 {
		in.PrepaymentPercent = dec(a.PrepaymentPct)
	}
	return in, warnings, ""
}

// services — aniq ro'yxat; bo'sh ro'yxat xizmatlarni olib tashlaydi.
func services(a priceQuoteArgs) []string {
	s := normalizeServiceKeys(a.Services)

	if a.NeedInstall {
		s = append(s, "install")
	}
	return s
}

// rebuildInput — oxirgi hisob natijasidan qayta so'rov (Django o'sha sonlarni oladi).
func rebuildInput(last *curtains.CalcResult, role curtains.UserRole) curtains.CalcInput {
	in := curtains.CalcInput{Role: role, Services: []string{}}
	for _, m := range last.Measurements {
		mi := curtains.MeasurementInput{Length: m.Length, Height: m.Height}
		for _, p := range m.Parts {
			w, hgt, c := p.WidthM, p.HeightM, p.Coef
			mi.Parts = append(mi.Parts, curtains.PartInput{VariantID: p.VariantID, Role: p.Role, WidthM: &w, HeightM: &hgt, Coef: &c})
		}
		for _, x := range m.Accessories {
			mi.Accessories = append(mi.Accessories, curtains.AccessoryInput{VariantID: x.VariantID, Qty: x.Qty})
		}
		in.Measurements = append(in.Measurements, mi)
	}
	return in
}

// normalizeServiceKeys — LLM ba'zan bir nechta kalitni bitta satrga yozadi
// ("sewing tape hook" yoki "sewing, tape"): bo'sh joy/vergul bo'yicha bo'linadi,
// trim+lowercase, dublikat olib tashlanadi. Aks holda Django "'sewing tape hook'
// xizmati topilmadi" deb butun hisobni rad etadi.
func normalizeServiceKeys(raw []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range raw {
		for _, key := range strings.FieldsFunc(item, func(r rune) bool {
			return r == ' ' || r == ',' || r == ';' || r == '\t' || r == '\n'
		}) {
			key = strings.ToLower(strings.TrimSpace(key))
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, key)
		}
	}
	return out
}
