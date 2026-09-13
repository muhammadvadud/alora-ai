package tools

import (
	"context"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/muhammadvadud/alora-agent/internal/agent"
	"github.com/muhammadvadud/alora-agent/internal/curtains"
)

// BuildOptions bir chaqiruvda uchta to'liq paketni tayyorlaydi. Har bir narx
// alohida CalcPreview'dan keladi, shuning uchun uch so'rov parallel yuboriladi.
func BuildOptions(d Deps) tool.BaseTool {
	h := &buildOptionsHandler{d: d}
	return must(utils.InferTool("build_options",
		"Yig'ilgan talab va o'lchov asosida arzon / o'rtacha / premium — 3 ta to'liq taklifni bir chaqiruvda tuzadi (mahsulot tanlash + hisob). Faqat o'lcham ma'lum bo'lgach chaqir.",
		h.do))
}

type buildOptionsHandler struct{ d Deps }

func (h *buildOptionsHandler) do(ctx context.Context, a buildOptionsArgs) (buildOptionsResult, error) {
	v, errMsg, stale := readyView(h.d)
	if errMsg != "" {
		return buildOptionsResult{Error: errMsg}, nil
	}
	if h.d.Calc == nil {
		return buildOptionsResult{Error: errCalcUnavailable}, nil
	}
	st, sid := loadState(ctx, h.d)
	if st.CorniceLengthM.IsZero() || st.HeightM.IsZero() {
		return buildOptionsResult{Error: "avval karniz uzunligi va balandlikni curtain_calculate ga bering yoki set_context bilan yozing"}, nil
	}
	q := curtains.SearchQuery{Room: first(a.Room, st.Room), Style: first(a.Style, st.Style), Pattern: first(a.Pattern, st.Pattern), Transparency: first(a.Transparency, st.Transparency), Color: first(a.Color, "")}
	plans := curtains.PlanOptions(v, st, q)
	outs := make([]buildOptionResult, 3)
	results := [3]*curtains.CalcResult{}
	errs := make([]error, 3)
	var wg sync.WaitGroup
	for i := range plans {
		outs[i].Tier = string(plans[i].Tier)
		outs[i].Why = plans[i].Why
		if plans[i].Parts == nil {
			outs[i].Error = plans[i].Why
			continue
		}
		outs[i].Parts = optionParts(plans[i].Parts)
		outs[i].Accessories = optionAccessories(plans[i].Accessories)
		outs[i].Services = append([]string(nil), plans[i].Services...)
		in := planInput(plans[i], st, userRole(ctx))
		wg.Add(1)
		go func(i int, in curtains.CalcInput) {
			defer wg.Done()
			res, err := h.d.Calc.Calculate(ctx, in)
			if err != nil {
				errs[i] = err
				return
			}
			results[i] = &res
		}(i, in)
	}
	wg.Wait()

	success := 0
	for i := range outs {
		if errs[i] != nil {
			outs[i].Error = calcErrorText(errs[i])
			continue
		}
		if results[i] == nil {
			continue
		}
		success++
		outs[i].Calc = toAny(*results[i])
		outs[i].Display = displayTotals(*results[i])
		outs[i].Total = displayMoney(results[i].Total)
		attachPlanProducts(ctx, v, plans[i])
	}
	if success == 0 {
		return buildOptionsResult{Options: outs, Stale: stale, Error: errCalcUnavailable}, nil
	}
	warnings := append([]string{}, optionWarnings(results)...)
	if stale {
		warnings = append(warnings, warnStale)
	}
	st.Apply("build_options", argsMap(a), now(h.d))
	saveState(ctx, h.d, sid, st)
	return buildOptionsResult{Options: outs, Comparison: CompareOptions(results), Warnings: warnings, Stale: stale}, nil
}

func first(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func planInput(p curtains.OptionPlan, st curtains.SessionState, role curtains.UserRole) curtains.CalcInput {
	m := curtains.MeasurementInput{Length: st.CorniceLengthM, Height: st.HeightM, Accessories: append([]curtains.AccessoryInput(nil), p.Accessories...)}
	for _, x := range p.Parts {
		w, h, c := x.WidthM, x.HeightM, x.Coef
		m.Parts = append(m.Parts, curtains.PartInput{VariantID: x.VariantID, Role: x.Role, WidthM: &w, HeightM: &h, Coef: &c})
	}
	services := append([]string(nil), p.Services...)
	if p.NeedCornice {
		services = append(services, "cornice")
	}
	return curtains.CalcInput{Measurements: []curtains.MeasurementInput{m}, Services: services, Role: role}
}

func optionParts(parts []curtains.PartSpec) []curtainOptionPart {
	out := make([]curtainOptionPart, 0, len(parts))
	for _, p := range parts {
		out = append(out, curtainOptionPart{Role: string(p.Role), VariantID: p.VariantID, WidthM: p.WidthM.String(), HeightM: p.HeightM.String(), Coef: p.Coef.String()})
	}
	return out
}

func optionAccessories(xs []curtains.AccessoryInput) []calcAccessory {
	out := make([]calcAccessory, 0, len(xs))
	for _, x := range xs {
		out = append(out, calcAccessory{VariantID: x.VariantID, Qty: x.Qty.InexactFloat64()})
	}
	return out
}

func attachPlanProducts(ctx context.Context, v *curtains.View, p curtains.OptionPlan) {
	refs := make([]agent.ProductRef, 0, len(p.Parts)+len(p.Accessories))
	for _, id := range append(partIDs(p.Parts), accessoryIDs(p.Accessories)...) {
		vx, ok := v.Variant(id)
		if !ok {
			continue
		}
		prod, ok := v.Product(vx.ProductID)
		if !ok {
			continue
		}
		price, _ := vx.Price.Float64()
		ref := agent.ProductRef{VariantID: id, ProductName: prod.Name, ColorName: vx.ColorName, Price: price, SKU: vx.SKU}
		if len(prod.ImageURLs) > 0 {
			ref.ImageURL = prod.ImageURLs[0]
		}
		refs = append(refs, ref)
	}
	agent.AttachProducts(ctx, refs)
}

func partIDs(xs []curtains.PartSpec) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		out = append(out, x.VariantID)
	}
	return out
}
func accessoryIDs(xs []curtains.AccessoryInput) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		out = append(out, x.VariantID)
	}
	return out
}

func optionWarnings(results [3]*curtains.CalcResult) []string {
	if results[0] == nil || results[1] == nil || results[2] == nil {
		return nil
	}
	if results[0].Total.GreaterThan(results[1].Total) || results[1].Total.GreaterThan(results[2].Total) {
		return []string{"paketlar narxi kutilgan tartibda emas: economy ≤ standard ≤ premium emas"}
	}
	return nil
}

// CompareOptions faqat Django qaytargan jami summalarni taqqoslaydi.
func CompareOptions(results [3]*curtains.CalcResult) []string {
	labels := []string{"Economy", "O'rtacha", "Premium"}
	why := []string{"asosiy portyera", "tul qo'shilgan", "astar + lambrekan + montaj"}
	var out []string
	for i := 1; i < len(results); i++ {
		if results[i] == nil || results[i-1] == nil || !results[i].Total.GreaterThan(results[i-1].Total) {
			continue
		}
		diff := results[i].Total.Sub(results[i-1].Total)
		out = append(out, labels[i]+" "+stringsLower(labels[i-1])+" "+displayMoney(diff)+" qimmat: "+why[i])
	}
	return out
}

func stringsLower(s string) string {
	if s == "O'rtacha" {
		return "o'rtachadan"
	}
	return "economy'dan"
}
