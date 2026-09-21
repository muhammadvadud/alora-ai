package backend

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/muhammadvadud/alora-ai/gen/alorav1"
	"github.com/muhammadvadud/alora-ai/internal/curtains"
	"github.com/shopspring/decimal"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func decimalText(s string) (decimal.Decimal, error) {
	if s == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(s)
}

func decimalFloat(v float64) (decimal.Decimal, error) {
	return decimal.NewFromString(strconv.FormatFloat(v, 'f', -1, 64))
}

func mapJSON(s string) (map[string]any, error) {
	if s == "" {
		return nil, nil
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func productFromPB(p *alorav1.CatalogProduct) (curtains.Product, error) {
	density, err := decimalFloat(p.GetDensity())
	if err != nil {
		return curtains.Product{}, fmt.Errorf("product density: %w", err)
	}
	coef, err := decimalFloat(p.GetRecommendedCoef())
	if err != nil {
		return curtains.Product{}, fmt.Errorf("product recommended_coef: %w", err)
	}
	width, err := decimalFloat(p.GetDefaultWidthM())
	if err != nil {
		return curtains.Product{}, fmt.Errorf("product default_width_m: %w", err)
	}
	tags, err := tagsJSON(p.GetTagsJson())
	if err != nil {
		return curtains.Product{}, fmt.Errorf("product tags_json: %w", err)
	}
	variantIDs := make([]string, 0, len(p.GetVariants()))
	for _, v := range p.GetVariants() {
		if v != nil {
			variantIDs = append(variantIDs, v.GetId())
		}
	}
	return curtains.Product{
		ID: p.GetId(), TenantID: p.GetTenantId(), CategoryID: p.GetCategoryId(), Name: p.GetName(), Code: p.GetCode(),
		Description: p.GetDescription(), FabricType: p.GetFabricType(), Pattern: p.GetPattern(),
		Style: p.GetStyle(), Transparency: p.GetTransparency(), Density: density,
		Composition: p.GetComposition(), Care: p.GetCare(), RecommendedRooms: append([]string(nil), p.GetRecommendedRooms()...),
		RecommendedCoef: coef, DefaultWidthM: width, IsSewingRequired: p.GetIsSewingRequired(),
		Segment: curtains.Segment(p.GetSegment()), Tags: tags, VariantIDs: variantIDs,
		ImageURLs: append([]string(nil), p.GetImageUrls()...),
	}, nil
}

func variantFromPB(v *alorav1.CatalogVariant) (curtains.Variant, error) {
	width, err := decimalFloat(v.GetWidthM())
	if err != nil {
		return curtains.Variant{}, fmt.Errorf("variant width_m: %w", err)
	}
	price, err := decimalFloat(v.GetPrice())
	if err != nil {
		return curtains.Variant{}, fmt.Errorf("variant price: %w", err)
	}
	oldPrice, err := decimalFloat(v.GetOldPrice())
	if err != nil {
		return curtains.Variant{}, fmt.Errorf("variant old_price: %w", err)
	}
	return curtains.Variant{ID: v.GetId(), TenantID: v.GetTenantId(), ProductID: v.GetProductId(), SKU: v.GetSku(), ColorName: v.GetColorName(), ColorHex: v.GetColorHex(), WidthM: width, Price: price, PriceUnit: curtains.PriceUnit(v.GetPriceUnit()), OldPrice: oldPrice, IsActive: v.GetIsActive()}, nil
}

func categoryFromPB(c *alorav1.CatalogCategory) curtains.Category {
	return curtains.Category{ID: c.GetId(), TenantID: c.GetTenantId(), Name: c.GetName(), Slug: c.GetSlug(), ParentID: c.GetParentId(), SortOrder: int(c.GetSortOrder())}
}

func calcRuleFromPB(r *alorav1.CatalogCalcRule) (curtains.CalcRule, error) {
	v, err := decimalFloat(r.GetValue())
	if err != nil {
		return curtains.CalcRule{}, fmt.Errorf("calc_rule value: %w", err)
	}
	applies, err := mapJSON(r.GetAppliesToJson())
	if err != nil {
		return curtains.CalcRule{}, fmt.Errorf("calc_rule applies_to_json: %w", err)
	}
	return curtains.CalcRule{Key: r.GetKey(), Value: v, Unit: r.GetUnit(), Description: r.GetDescription(), AppliesTo: applies}, nil
}

func serviceFromPB(s *alorav1.CatalogServicePrice) (curtains.Service, error) {
	price, err := decimalText(s.GetPrice())
	if err != nil {
		return curtains.Service{}, fmt.Errorf("service price: %w", err)
	}
	return curtains.Service{Key: s.GetKey(), Name: s.GetName(), Price: price, Unit: curtains.PriceUnit(s.GetUnit()), IsActive: s.GetIsActive()}, nil
}

func tierFromPB(t *alorav1.CatalogDiscountTier) (curtains.DiscountTier, error) {
	min, err := decimalText(t.GetMinAmount())
	if err != nil {
		return curtains.DiscountTier{}, fmt.Errorf("discount tier min_amount: %w", err)
	}
	pct, err := decimalText(t.GetPercent())
	if err != nil {
		return curtains.DiscountTier{}, fmt.Errorf("discount tier percent: %w", err)
	}
	return curtains.DiscountTier{ID: t.GetId(), MinAmount: min, Percent: pct, IsActive: t.GetIsActive()}, nil
}

func policyFromPB(p *alorav1.CatalogDiscountPolicy) (curtains.DiscountPolicy, error) {
	employee, err := decimalText(p.GetManualMaxPercentEmployee())
	if err != nil {
		return curtains.DiscountPolicy{}, fmt.Errorf("discount policy employee: %w", err)
	}
	manager, err := decimalText(p.GetManualMaxPercentManager())
	if err != nil {
		return curtains.DiscountPolicy{}, fmt.Errorf("discount policy manager: %w", err)
	}
	return curtains.DiscountPolicy{ManualMaxPercentEmployee: employee, ManualMaxPercentManager: manager, StackWithTier: p.GetStackWithTier()}, nil
}

func defaultsFromPB(d *alorav1.CatalogOrderDefaults) (curtains.OrderDefaults, error) {
	prepayment, err := decimalText(d.GetPrepaymentPercent())
	if err != nil {
		return curtains.OrderDefaults{}, fmt.Errorf("order defaults prepayment_percent: %w", err)
	}
	delivery, err := decimalText(d.GetDeliveryAmount())
	if err != nil {
		return curtains.OrderDefaults{}, fmt.Errorf("order defaults delivery_amount: %w", err)
	}
	return curtains.OrderDefaults{PrepaymentPercent: prepayment, DeliveryAmount: delivery, DeadlineDays: int(d.GetDeadlineDays())}, nil
}

func fasonFromPB(f *alorav1.CatalogFason) (curtains.Fason, error) {
	result := curtains.Fason{ID: f.GetId(), TenantID: f.GetTenantId(), Name: f.GetName(), Style: f.GetStyle(), Rooms: append([]string(nil), f.GetRooms()...), ImageURL: f.GetImageUrl(), PartCount: int(f.GetPartCount())}
	for _, p := range f.GetParts() {
		width, err := decimalText(p.GetWidthShare())
		if err != nil {
			return curtains.Fason{}, fmt.Errorf("fason part width_share: %w", err)
		}
		height, err := decimalText(p.GetHeightShare())
		if err != nil {
			return curtains.Fason{}, fmt.Errorf("fason part height_share: %w", err)
		}
		coef, err := decimalText(p.GetDefaultCoef())
		if err != nil {
			return curtains.Fason{}, fmt.Errorf("fason part default_coef: %w", err)
		}
		result.Parts = append(result.Parts, curtains.FasonPart{ID: p.GetId(), Role: curtains.PartRole(p.GetRole()), Name: p.GetName(), WidthShare: width, HeightShare: height, DefaultCoef: coef, SortOrder: int(p.GetSortOrder())})
	}
	for _, a := range f.GetAccessories() {
		qty, err := decimalText(a.GetDefaultQty())
		if err != nil {
			return curtains.Fason{}, fmt.Errorf("fason accessory default_qty: %w", err)
		}
		result.Accessories = append(result.Accessories, curtains.FasonAccessory{ID: a.GetId(), Category: a.GetCategory(), DefaultQty: qty, QtyFormula: a.GetQtyFormula()})
	}
	return result, nil
}

func knowledgeFromPB(k *alorav1.CatalogKnowledgeRule) (curtains.KnowledgeRule, error) {
	condition, err := mapJSON(k.GetConditionJson())
	if err != nil {
		return curtains.KnowledgeRule{}, fmt.Errorf("knowledge condition_json: %w", err)
	}
	return curtains.KnowledgeRule{ID: k.GetId(), Kind: k.GetKind(), Condition: condition, Recommendation: k.GetRecommendation(), Priority: int(k.GetPriority())}, nil
}

func promptFromPB(p *alorav1.CatalogPrompt) (curtains.PromptVersion, error) {
	updated, err := time.Parse(time.RFC3339, p.GetUpdatedAt())
	if err != nil {
		return curtains.PromptVersion{}, fmt.Errorf("prompt updated_at: %w", err)
	}
	return curtains.PromptVersion{Version: int(p.GetVersion()), Content: p.GetContent(), UpdatedAt: updated}, nil
}

func snapshotFromPages(pages []*alorav1.CatalogSnapshot) (curtains.Snapshot, error) {
	if len(pages) == 0 {
		return curtains.Snapshot{}, errors.New("katalog snapshot sahifalari bo'sh")
	}
	result := curtains.Snapshot{Version: pages[0].GetVersion()}
	for pageIndex, page := range pages {
		for _, p := range page.GetProducts() {
			product, err := productFromPB(p)
			if err != nil {
				return curtains.Snapshot{}, err
			}
			for _, v := range p.GetVariants() {
				variant, err := variantFromPB(v)
				if err != nil {
					return curtains.Snapshot{}, err
				}
				result.Variants = append(result.Variants, variant)
				product.VariantIDs = append(product.VariantIDs, variant.ID)
			}
			result.Products = append(result.Products, product)
		}
		if pageIndex != 0 {
			continue
		}
		for _, c := range page.GetCategories() {
			result.Categories = append(result.Categories, categoryFromPB(c))
		}
		for _, r := range page.GetCalcRules() {
			x, err := calcRuleFromPB(r)
			if err != nil {
				return curtains.Snapshot{}, err
			}
			result.CalcRules = append(result.CalcRules, x)
		}
		for _, k := range page.GetKnowledgeRules() {
			x, err := knowledgeFromPB(k)
			if err != nil {
				return curtains.Snapshot{}, err
			}
			result.Knowledge = append(result.Knowledge, x)
		}
		for _, s := range page.GetServicePrices() {
			x, err := serviceFromPB(s)
			if err != nil {
				return curtains.Snapshot{}, err
			}
			result.Services = append(result.Services, x)
		}
		for _, t := range page.GetDiscountTiers() {
			x, err := tierFromPB(t)
			if err != nil {
				return curtains.Snapshot{}, err
			}
			result.Tiers = append(result.Tiers, x)
		}
		if page.GetDiscountPolicy() != nil {
			x, err := policyFromPB(page.GetDiscountPolicy())
			if err != nil {
				return curtains.Snapshot{}, err
			}
			result.Policy = &x
		}
		if page.GetOrderDefaults() != nil {
			x, err := defaultsFromPB(page.GetOrderDefaults())
			if err != nil {
				return curtains.Snapshot{}, err
			}
			result.Defaults = &x
		}
		for _, f := range page.GetFasons() {
			x, err := fasonFromPB(f)
			if err != nil {
				return curtains.Snapshot{}, err
			}
			result.Fasons = append(result.Fasons, x)
		}
		if page.GetPrompt() != nil {
			x, err := promptFromPB(page.GetPrompt())
			if err != nil {
				return curtains.Snapshot{}, err
			}
			result.Prompt = &x
		}
	}
	return result, nil
}

func unmarshalPayload(s string, message proto.Message) error {
	if err := protojson.Unmarshal([]byte(s), message); err != nil {
		return fmt.Errorf("delta payload: %w", err)
	}
	return nil
}

func deltaFromPB(d *alorav1.CatalogDelta) (curtains.Delta, error) {
	result := curtains.Delta{Version: d.GetVersion(), Entity: d.GetEntity(), EntityID: d.GetEntityId(), Action: d.GetAction()}
	if d.GetEntity() == "catalog" && d.GetAction() == "FULL_RESYNC" {
		return result, nil
	}
	if d.GetEntity() == "catalog" {
		return curtains.Delta{}, curtains.ErrUnknownEntity
	}
	var message proto.Message
	switch d.GetEntity() {
	case curtains.EntityProduct:
		message = &alorav1.CatalogProduct{}
	case curtains.EntityVariant:
		if d.GetAction() == curtains.ActionDelete {
			var raw struct {
				ProductID string `json:"product_id"`
			}
			if err := json.Unmarshal([]byte(d.GetPayloadJson()), &raw); err != nil {
				return curtains.Delta{}, fmt.Errorf("variant delete payload: %w", err)
			}
			result.Payload = curtains.VariantDeleteRef{ProductID: raw.ProductID}
			return result, nil
		}
		message = &alorav1.CatalogVariant{}
	case curtains.EntityCategory:
		message = &alorav1.CatalogCategory{}
	case curtains.EntityCalcRule:
		message = &alorav1.CatalogCalcRule{}
	case curtains.EntityKnowledgeRule:
		message = &alorav1.CatalogKnowledgeRule{}
	case curtains.EntityServicePrice:
		message = &alorav1.CatalogServicePrice{}
	case curtains.EntityDiscountTier:
		message = &alorav1.CatalogDiscountTier{}
	case curtains.EntityDiscountPolicy:
		message = &alorav1.CatalogDiscountPolicy{}
	case curtains.EntityOrderDefaults:
		message = &alorav1.CatalogOrderDefaults{}
	case curtains.EntityFason:
		message = &alorav1.CatalogFason{}
	case curtains.EntityPrompt:
		message = &alorav1.CatalogPrompt{}
	default:
		return curtains.Delta{}, curtains.ErrUnknownEntity
	}
	if d.GetAction() == curtains.ActionDelete {
		return result, nil
	}
	if d.GetAction() != curtains.ActionCreate && d.GetAction() != curtains.ActionUpdate {
		return curtains.Delta{}, curtains.ErrUnknownEntity
	}
	if err := unmarshalPayload(d.GetPayloadJson(), message); err != nil {
		return curtains.Delta{}, err
	}
	var err error
	switch d.GetEntity() {
	case curtains.EntityProduct:
		result.Payload, err = productFromPB(message.(*alorav1.CatalogProduct))
	case curtains.EntityVariant:
		result.Payload, err = variantFromPB(message.(*alorav1.CatalogVariant))
	case curtains.EntityCategory:
		result.Payload = categoryFromPB(message.(*alorav1.CatalogCategory))
	case curtains.EntityCalcRule:
		result.Payload, err = calcRuleFromPB(message.(*alorav1.CatalogCalcRule))
	case curtains.EntityKnowledgeRule:
		result.Payload, err = knowledgeFromPB(message.(*alorav1.CatalogKnowledgeRule))
	case curtains.EntityServicePrice:
		result.Payload, err = serviceFromPB(message.(*alorav1.CatalogServicePrice))
	case curtains.EntityDiscountTier:
		result.Payload, err = tierFromPB(message.(*alorav1.CatalogDiscountTier))
	case curtains.EntityDiscountPolicy:
		result.Payload, err = policyFromPB(message.(*alorav1.CatalogDiscountPolicy))
	case curtains.EntityOrderDefaults:
		result.Payload, err = defaultsFromPB(message.(*alorav1.CatalogOrderDefaults))
	case curtains.EntityFason:
		result.Payload, err = fasonFromPB(message.(*alorav1.CatalogFason))
	case curtains.EntityPrompt:
		result.Payload, err = promptFromPB(message.(*alorav1.CatalogPrompt))
	}
	if err != nil {
		return curtains.Delta{}, err
	}
	return result, nil
}

// tagsJSON — kontrakt bo'yicha teglar RO'YXAT (`["rim","rulon"]`, Django JSONField
// default=list); eski yozuvlarda obyekt ham uchraydi. Ikkalasi ham map ko'rinishiga
// keltiriladi (ro'yxat elementi → kalit=true), chunki bitta noto'g'ri mahsulot
// butun snapshot'ni yiqitmasligi kerak (2026-09-14 live: katalog umuman yuklanmadi).
func tagsJSON(raw string) (map[string]any, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return nil, nil
	}
	if strings.HasPrefix(raw, "[") {
		var list []any
		if err := json.Unmarshal([]byte(raw), &list); err != nil {
			return nil, err
		}
		out := make(map[string]any, len(list))
		for _, it := range list {
			if s, ok := it.(string); ok && s != "" {
				out[s] = true
			}
		}
		return out, nil
	}
	return mapJSON(raw)
}
