package curtains

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// Bu fayl — Django `apps/pricing/services/calculator.py` ning AYNAN nusxasi
// (rules.md §0.3, §4.2). Funksiya nomlari va tartibi ataylab Django'niki bilan
// bir xil — ikki kodni yonma-yon solishtirish oson bo'lsin. Yagona haqiqat —
// tests/fixtures/calc_cases.json (backend bilan umumiy). Formulani o'zgartirish
// faqat etalon fayl bilan birga, backend jamoasi bilan kelishib.

var (
	zero    = decimal.Zero
	one     = decimal.NewFromInt(1)
	hundred = decimal.NewFromInt(100)
)

// CalcRule kalitlari — calc_cases.json `pricing_seed.calc_rules` bilan bir xil.
const (
	RuleCoefDefault      = "coef_default"
	RuleSeamAllowance    = "seam_allowance"
	RuleHemAllowance     = "hem_allowance"
	RuleHookStep         = "hook_step"
	RuleRoundAreaTo      = "round_area_to"
	RuleRoundTotalTo     = "round_total_to"
	RuleMinOrderAmount   = "min_order_amount"
	RuleWasteWarnPercent = "waste_warn_percent"
)

// ServiceDelivery — yetkazish xizmatining kaliti (yo'q bo'lsa OrderDefaults ishlaydi).
const ServiceDelivery = "delivery"

// roundToStep — Django `_round_to_step`: (value/step) ni butunga HALF_UP, keyin × step.
func roundToStep(value, step decimal.Decimal) decimal.Decimal {
	if step.LessThanOrEqual(zero) {
		return value
	}
	return value.Div(step).Round(0).Mul(step)
}

// quantizeMoney — Django `_quantize_money`: 2 kasr, HALF_UP.
func quantizeMoney(value decimal.Decimal) decimal.Decimal {
	return value.Round(2)
}

// ruleValue — Django `_rule_value`: kalit topilmasa xato, default o'ylab topilmaydi (rules §4.2.1).
func ruleValue(src PricingSource, key string) (decimal.Decimal, error) {
	v, ok := src.CalcRule(key)
	if !ok {
		return zero, validationf("rules", "'%s' hisob qoidasi topilmadi", key)
	}
	return v, nil
}

// resolveDiscountPolicy — Django `_resolve_discount_policy`: employee → o'z chegarasi,
// qolgan rollar (manager, admin, noma'lum) → manager chegarasi.
func resolveDiscountPolicy(policy DiscountPolicy, role UserRole) decimal.Decimal {
	if role == RoleEmployee {
		return policy.ManualMaxPercentEmployee
	}
	return policy.ManualMaxPercentManager
}

// matchTier — Django `_match_tier`: subtotal ≥ min_amount bo'lgan eng yuqori foiz.
func matchTier(tiers []DiscountTier, subtotal decimal.Decimal) decimal.Decimal {
	matched := zero
	for _, t := range tiers {
		if !t.IsActive {
			continue
		}
		if subtotal.GreaterThanOrEqual(t.MinAmount) && t.Percent.GreaterThanOrEqual(matched) {
			matched = t.Percent
		}
	}
	return matched
}

func variantByID(src PricingSource, id string) (Variant, error) {
	v, ok := src.Variant(id)
	if !ok {
		return Variant{}, validationf("variant_id", "Variant topilmadi")
	}
	return v, nil
}

// partWidth/partHeight/partCoef — bo'lakda berilmagan bo'lsa o'lchov/qoidadan (Django `_part_*`).
func partWidth(p PartInput, m MeasurementInput) decimal.Decimal {
	if p.WidthM != nil {
		return *p.WidthM
	}
	return m.Length
}

func partHeight(p PartInput, m MeasurementInput) decimal.Decimal {
	if p.HeightM != nil {
		return *p.HeightM
	}
	return m.Height
}

func partCoef(p PartInput, src PricingSource) (decimal.Decimal, error) {
	if p.Coef != nil {
		return *p.Coef, nil
	}
	return ruleValue(src, RuleCoefDefault)
}

// partVisibleArea — ko'rinadigan yuza: width × coef × height.
func partVisibleArea(width, height, coef decimal.Decimal) decimal.Decimal {
	return width.Mul(coef).Mul(height)
}

// partRawArea — zapaslar bilan xom yuza: width × coef × (height + seam + hem).
func partRawArea(width, height, coef, seam, hem decimal.Decimal) decimal.Decimal {
	return width.Mul(coef).Mul(height.Add(seam).Add(hem))
}

// variantLineTotal — Django `_variant_line_total`: birlikdan qat'i nazar qty × price, 2 kasr.
func variantLineTotal(v Variant, qty decimal.Decimal) decimal.Decimal {
	return quantizeMoney(qty.Mul(v.Price))
}

// serviceQuantity — Django `_service_quantity`: hook → ceil(Σlength / hook_step);
// per_sqm → Σarea; per_meter → Σlength; per_piece → bo'laklar soni.
func serviceQuantity(svc Service, src PricingSource, totalArea, totalLength decimal.Decimal, partCount int) (decimal.Decimal, error) {
	if svc.Key == "hook" {
		step, err := ruleValue(src, RuleHookStep)
		if err != nil {
			return zero, err
		}
		if step.LessThanOrEqual(zero) {
			return zero, nil
		}
		return totalLength.Div(step).Ceil(), nil
	}
	switch svc.Unit {
	case PerSqm:
		return totalArea, nil
	case PerMeter:
		return totalLength, nil
	default:
		return decimal.NewFromInt(int64(partCount)), nil
	}
}

// buildMeasurement — Django `_build_measurement_preview`: bitta deraza bo'yicha bo'laklar va bezaklar.
func buildMeasurement(m MeasurementInput, src PricingSource) (MeasurementResult, error) {
	seam, err := ruleValue(src, RuleSeamAllowance)
	if err != nil {
		return MeasurementResult{}, err
	}
	hem, err := ruleValue(src, RuleHemAllowance)
	if err != nil {
		return MeasurementResult{}, err
	}
	areaStep, err := ruleValue(src, RuleRoundAreaTo)
	if err != nil {
		return MeasurementResult{}, err
	}

	res := MeasurementResult{
		Length:      m.Length,
		Height:      m.Height,
		Parts:       make([]PartResult, 0, len(m.Parts)),
		Accessories: make([]AccessoryResult, 0, len(m.Accessories)),
	}
	totalRaw, totalArea := zero, zero

	for _, p := range m.Parts {
		v, err := variantByID(src, p.VariantID)
		if err != nil {
			return MeasurementResult{}, err
		}
		width := partWidth(p, m)
		height := partHeight(p, m)
		coef, err := partCoef(p, src)
		if err != nil {
			return MeasurementResult{}, err
		}
		visible := partVisibleArea(width, height, coef)
		raw := partRawArea(width, height, coef, seam, hem)
		rounded := roundToStep(raw, areaStep)
		waste := raw.Sub(visible)
		wastePct := zero
		if visible.GreaterThan(zero) {
			// 12 kasr — Python Decimal (28 raqam) bilan yaxlitlash chegarasida farq bo'lmasin.
			wastePct = waste.Mul(hundred).DivRound(visible, 12).Round(2)
		}
		res.Parts = append(res.Parts, PartResult{
			VariantID:     v.ID,
			SKU:           v.SKU,
			Role:          p.Role,
			WidthM:        width,
			HeightM:       height,
			Coef:          coef,
			VisibleAreaM2: visible,
			RawAreaM2:     raw,
			AreaM2:        rounded,
			WasteAreaM2:   waste,
			WastePercent:  wastePct,
			PriceUnit:     v.PriceUnit,
			UnitPrice:     v.Price,
			LineTotal:     variantLineTotal(v, rounded),
		})
		totalRaw = totalRaw.Add(raw)
		totalArea = totalArea.Add(rounded)
	}

	accTotal := zero
	for _, a := range m.Accessories {
		v, err := variantByID(src, a.VariantID)
		if err != nil {
			return MeasurementResult{}, err
		}
		line := variantLineTotal(v, a.Qty)
		res.Accessories = append(res.Accessories, AccessoryResult{
			VariantID: v.ID,
			SKU:       v.SKU,
			Qty:       a.Qty,
			PriceUnit: v.PriceUnit,
			UnitPrice: v.Price,
			LineTotal: line,
		})
		accTotal = accTotal.Add(line)
	}

	res.RawAreaM2 = totalRaw
	res.AreaM2 = totalArea
	res.AccessoriesTotal = quantizeMoney(accTotal)
	return res, nil
}

// buildServiceLines — Django `_build_service_lines`: xizmat qatorlari, ularning yig'indisi va yetkazish.
func buildServiceLines(measurements []MeasurementResult, src PricingSource, requested []string, delivery bool) ([]ServiceLine, decimal.Decimal, decimal.Decimal, error) {
	totalArea, totalLength := zero, zero
	totalParts := 0
	for _, m := range measurements {
		totalLength = totalLength.Add(m.Length)
		totalParts += len(m.Parts)
		totalArea = totalArea.Add(m.AreaM2)
	}

	lines := make([]ServiceLine, 0, len(requested)+1)
	serviceSum, deliveryAmount := zero, zero

	for _, key := range requested {
		svc, ok := src.Service(key)
		if !ok || !svc.IsActive {
			return nil, zero, zero, validationf("services", "'%s' xizmati topilmadi", key)
		}
		qty, err := serviceQuantity(svc, src, totalArea, totalLength, totalParts)
		if err != nil {
			return nil, zero, zero, err
		}
		line := quantizeMoney(qty.Mul(svc.Price))
		lines = append(lines, ServiceLine{
			Key: svc.Key, Name: svc.Name, Qty: qty, PriceUnit: svc.Unit,
			UnitPrice: svc.Price, LineTotal: line, IsDelivery: false,
		})
		serviceSum = serviceSum.Add(line)
	}

	if delivery {
		if svc, ok := src.Service(ServiceDelivery); ok && svc.IsActive {
			deliveryAmount = quantizeMoney(svc.Price)
			lines = append(lines, ServiceLine{
				Key: svc.Key, Name: svc.Name, Qty: one, PriceUnit: svc.Unit,
				UnitPrice: svc.Price, LineTotal: deliveryAmount, IsDelivery: true,
			})
		} else {
			defaults, ok := src.OrderDefaults()
			if !ok {
				return nil, zero, zero, validationf("orderdefaults", "Sozlama topilmadi")
			}
			deliveryAmount = quantizeMoney(defaults.DeliveryAmount)
			lines = append(lines, ServiceLine{
				Key: ServiceDelivery, Name: "Yetkazish", Qty: one, PriceUnit: PerPiece,
				UnitPrice: deliveryAmount, LineTotal: deliveryAmount, IsDelivery: true,
			})
		}
	}

	return lines, serviceSum, deliveryAmount, nil
}

// Calculate — Django `calculate_preview` nusxasi. Validatsiya (chegaralar) bu yerda
// EMAS — tool adapter avval Validate() chaqiradi; etalon holatlar faqat hisobni tekshiradi.
func Calculate(src PricingSource, in CalcInput) (CalcResult, error) {
	measurements := make([]MeasurementResult, 0, len(in.Measurements))
	for _, m := range in.Measurements {
		res, err := buildMeasurement(m, src)
		if err != nil {
			return CalcResult{}, err
		}
		measurements = append(measurements, res)
	}

	services, serviceSum, deliveryAmount, err := buildServiceLines(measurements, src, in.Services, in.Delivery)
	if err != nil {
		return CalcResult{}, err
	}

	partsSum, accessoriesSum := zero, zero
	for _, m := range measurements {
		for _, p := range m.Parts {
			partsSum = partsSum.Add(p.LineTotal)
		}
		accessoriesSum = accessoriesSum.Add(m.AccessoriesTotal)
	}

	subtotal := quantizeMoney(partsSum.Add(accessoriesSum).Add(serviceSum))
	tierDiscount := matchTier(src.DiscountTiers(), subtotal)

	// Chegirma siyosati yo'q bo'lsa (Django'da bu xato): qo'lda chegirma so'ralmagan
	// bo'lsa natija bir xil — 0 bilan davom etamiz; so'ralgan bo'lsa aniq xato.
	manualApplied := zero
	stack := false
	if policy, ok := src.DiscountPolicy(); ok {
		manualMax := resolveDiscountPolicy(policy, in.Role)
		manualApplied = decimal.Min(in.DiscountPercent, manualMax)
		stack = policy.StackWithTier
	} else if in.DiscountPercent.GreaterThan(zero) {
		return CalcResult{}, validationf("discountpolicy", "Sozlama topilmadi")
	}

	var appliedPercent decimal.Decimal
	if stack {
		appliedPercent = tierDiscount.Add(manualApplied)
	} else {
		appliedPercent = decimal.Max(tierDiscount, manualApplied)
	}

	discountAmount := quantizeMoney(subtotal.Mul(appliedPercent).Div(hundred))
	totalBefore := subtotal.Sub(discountAmount).Add(deliveryAmount)
	minOrder, err := ruleValue(src, RuleMinOrderAmount)
	if err != nil {
		return CalcResult{}, err
	}
	totalBefore = decimal.Max(totalBefore, minOrder)
	roundTotalTo, err := ruleValue(src, RuleRoundTotalTo)
	if err != nil {
		return CalcResult{}, err
	}
	total := quantizeMoney(roundToStep(totalBefore, roundTotalTo))

	warnings := []string{}
	wasteWarn, err := ruleValue(src, RuleWasteWarnPercent)
	if err != nil {
		return CalcResult{}, err
	}
	for _, m := range measurements {
		for _, p := range m.Parts {
			if p.WastePercent.GreaterThan(wasteWarn) {
				// Matn Django bilan bir xil — etalon `warnings` string sifatida solishtiriladi.
				warnings = append(warnings, fmt.Sprintf("%s uchun chiqindi %s%% dan oshdi", p.SKU, p.WastePercent.StringFixed(2)))
			}
		}
	}

	return CalcResult{
		Measurements:          measurements,
		Services:              services,
		Subtotal:              subtotal,
		TierDiscountPercent:   tierDiscount,
		ManualDiscountPercent: manualApplied,
		DiscountPercent:       appliedPercent,
		DiscountAmount:        discountAmount,
		DeliveryAmount:        quantizeMoney(deliveryAmount),
		Total:                 total,
		Warnings:              warnings,
	}, nil
}
