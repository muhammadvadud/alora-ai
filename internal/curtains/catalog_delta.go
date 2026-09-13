package curtains

// catalog_delta.go — Watch delta'larini View nusxasiga qo'llash (upsert/delete)
// va indeks yordamchilari. catalog.go dan ajratilgan (rules §2: ≤ 500 qator).

import (
	"errors"
	"sort"
)

func applyDelta(v *View, d Delta) error {
	if d.Action != ActionCreate && d.Action != ActionUpdate && d.Action != ActionDelete {
		return ErrUnknownAction
	}
	if d.Action == ActionDelete {
		return deleteDelta(v, d)
	}
	switch d.Entity {
	case EntityProduct:
		x, ok := payload[Product](d.Payload)
		if !ok {
			return errors.New("product payload noto'g'ri")
		}
		delete(v.byProduct, x.ID)
		v.products[x.ID] = cloneProduct(x)
		ensureProductIndex(v, x)
	case EntityVariant:
		x, ok := payload[Variant](d.Payload)
		if !ok {
			return errors.New("variant payload noto'g'ri")
		}
		oldProductID := ""
		if old, exists := v.variants[x.ID]; exists {
			oldProductID = old.ProductID
		}
		removeVariantIndex(v, x.ID)
		if oldProductID != "" && oldProductID != x.ProductID {
			removeProductVariant(v, oldProductID, x.ID)
		}
		v.variants[x.ID] = cloneVariant(x)
		v.byProduct[x.ProductID] = append(v.byProduct[x.ProductID], x.ID)
		addProductVariant(v, x.ProductID, x.ID)
		sort.Strings(v.byProduct[x.ProductID])
	case EntityCategory:
		x, ok := payload[Category](d.Payload)
		if !ok {
			return errors.New("category payload noto'g'ri")
		}
		v.categories[x.ID] = cloneCategory(x)
	case EntityCalcRule:
		x, ok := payload[CalcRule](d.Payload)
		if !ok {
			return errors.New("calc_rule payload noto'g'ri")
		}
		v.calcRules[x.Key] = x.Value
	case EntityServicePrice:
		x, ok := payload[Service](d.Payload)
		if !ok {
			return errors.New("service payload noto'g'ri")
		}
		v.services[x.Key] = cloneService(x)
	case EntityDiscountTier:
		x, ok := payload[DiscountTier](d.Payload)
		if !ok {
			return errors.New("discount_tier payload noto'g'ri")
		}
		replaceTier(v, x)
	case EntityDiscountPolicy:
		x, ok := payload[DiscountPolicy](d.Payload)
		if !ok {
			return errors.New("discount_policy payload noto'g'ri")
		}
		v.policy = ptr(x)
	case EntityOrderDefaults:
		x, ok := payload[OrderDefaults](d.Payload)
		if !ok {
			return errors.New("order_defaults payload noto'g'ri")
		}
		v.defaults = ptr(x)
	case EntityFason:
		x, ok := payload[Fason](d.Payload)
		if !ok {
			return errors.New("fason payload noto'g'ri")
		}
		replaceFason(v, x)
	case EntityKnowledgeRule:
		x, ok := payload[KnowledgeRule](d.Payload)
		if !ok {
			return errors.New("knowledge_rule payload noto'g'ri")
		}
		replaceKnowledge(v, x)
	case EntityPrompt:
		x, ok := payload[PromptVersion](d.Payload)
		if !ok {
			return errors.New("prompt payload noto'g'ri")
		}
		v.prompt = ptr(x)
	default:
		return ErrUnknownEntity
	}
	return nil
}

func deleteDelta(v *View, d Delta) error {
	switch d.Entity {
	case EntityProduct:
		delete(v.products, d.EntityID)
		for pid := range v.byProduct {
			if pid == d.EntityID {
				delete(v.byProduct, pid)
			}
		}
	case EntityVariant:
		productID := ""
		if x, ok := v.variants[d.EntityID]; ok {
			productID = x.ProductID
		}
		delete(v.variants, d.EntityID)
		removeVariantIndex(v, d.EntityID)
		if ref, ok := payload[VariantDeleteRef](d.Payload); ok && ref.ProductID != "" {
			productID = ref.ProductID
		}
		if productID != "" {
			removeProductVariant(v, productID, d.EntityID)
		}
	case EntityCategory:
		delete(v.categories, d.EntityID)
	case EntityCalcRule:
		delete(v.calcRules, d.EntityID)
	case EntityServicePrice:
		delete(v.services, d.EntityID)
	case EntityDiscountTier:
		removeTier(v, d.EntityID)
	case EntityDiscountPolicy:
		v.policy = nil
	case EntityOrderDefaults:
		v.defaults = nil
	case EntityFason:
		removeFason(v, d.EntityID)
	case EntityKnowledgeRule:
		removeKnowledge(v, d.EntityID)
	case EntityPrompt:
		v.prompt = nil
	default:
		return ErrUnknownEntity
	}
	return nil
}

func ensureProductIndex(v *View, p Product) {
	for _, id := range p.VariantIDs {
		if _, ok := v.variants[id]; ok && !contains(v.byProduct[p.ID], id) {
			v.byProduct[p.ID] = append(v.byProduct[p.ID], id)
		}
	}
	sort.Strings(v.byProduct[p.ID])
}

func addProductVariant(v *View, productID, variantID string) {
	if p, ok := v.products[productID]; ok && !contains(p.VariantIDs, variantID) {
		p.VariantIDs = append(p.VariantIDs, variantID)
		sort.Strings(p.VariantIDs)
		v.products[productID] = p
	}
}

func removeProductVariant(v *View, productID, variantID string) {
	p, ok := v.products[productID]
	if !ok {
		return
	}
	out := p.VariantIDs[:0]
	for _, id := range p.VariantIDs {
		if id != variantID {
			out = append(out, id)
		}
	}
	p.VariantIDs = out
	v.products[productID] = p
}
func removeVariantIndex(v *View, id string) {
	for pid := range v.byProduct {
		removeVariantFromProduct(v, pid, id)
	}
}
func removeVariantFromProduct(v *View, pid, id string) {
	ids := v.byProduct[pid]
	out := ids[:0]
	for _, x := range ids {
		if x != id {
			out = append(out, x)
		}
	}
	if len(out) == 0 {
		delete(v.byProduct, pid)
	} else {
		v.byProduct[pid] = out
	}
}
func contains(xs []string, x string) bool {
	for _, y := range xs {
		if x == y {
			return true
		}
	}
	return false
}
func payload[T any](x any) (T, bool) {
	if y, ok := x.(T); ok {
		return y, true
	}
	if y, ok := x.(*T); ok && y != nil {
		return *y, true
	}
	var zero T
	return zero, false
}
func ptr[T any](x T) *T { return &x }

func replaceTier(v *View, x DiscountTier) { removeTier(v, x.ID); v.tiers = append(v.tiers, x) }
func removeTier(v *View, id string) {
	out := v.tiers[:0]
	for _, x := range v.tiers {
		if x.ID != id {
			out = append(out, x)
		}
	}
	v.tiers = out
}
func replaceFason(v *View, x Fason) { removeFason(v, x.ID); v.fasons = append(v.fasons, cloneFason(x)) }
func removeFason(v *View, id string) {
	out := v.fasons[:0]
	for _, x := range v.fasons {
		if x.ID != id {
			out = append(out, x)
		}
	}
	v.fasons = out
}
func replaceKnowledge(v *View, x KnowledgeRule) {
	removeKnowledge(v, x.ID)
	v.knowledge = append(v.knowledge, cloneKnowledgeRule(x))
}
func removeKnowledge(v *View, id string) {
	out := v.knowledge[:0]
	for _, x := range v.knowledge {
		if x.ID != id {
			out = append(out, x)
		}
	}
	v.knowledge = out
}
