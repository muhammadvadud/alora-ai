package curtains

import (
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shopspring/decimal"
)

const (
	EntityProduct        = "product"
	EntityVariant        = "variant"
	EntityCategory       = "category"
	EntityCalcRule       = "calc_rule"
	EntityKnowledgeRule  = "knowledge_rule"
	EntityServicePrice   = "service_price"
	EntityDiscountTier   = "discount_tier"
	EntityDiscountPolicy = "discount_policy"
	EntityOrderDefaults  = "order_defaults"
	EntityFason          = "fason"
	EntityPrompt         = "prompt"

	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"
)

var (
	ErrVersionGap    = errors.New("katalog delta versiyasi uzildi")
	ErrUnknownEntity = errors.New("noma'lum katalog entitysi")
	ErrUnknownAction = errors.New("noma'lum katalog amali")
)

// VariantDeleteRef delta o'chirilgan variantni mahsulot indeksidan ham olib tashlashga
// yordam beradi: o'chirish payload'ida variantning o'zi kelmasligi mumkin.
type VariantDeleteRef struct {
	ProductID string
}

// View — katalogning o'zgarmas ko'rinishi. Pointer map'lar faqat yangi View
// qurilayotganda almashtiriladi, shu sabab o'qiyotgan tool eski snapshotni yo'qotmaydi.
type View struct {
	Version    int64
	SyncedAt   time.Time
	Stale      bool
	products   map[string]Product
	variants   map[string]Variant
	byProduct  map[string][]string
	categories map[string]Category
	calcRules  map[string]decimal.Decimal
	services   map[string]Service
	tiers      []DiscountTier
	policy     *DiscountPolicy
	defaults   *OrderDefaults
	fasons     []Fason
	knowledge  []KnowledgeRule
	prompt     *PromptVersion
}

var _ PricingSource = (*View)(nil)

// Catalog yozuvlarni ketma-ket qiladi, o'qishlarni esa atomic snapshot orqali
// lock'siz bajaradi. Bu uzoq tool chaqiruvi davomida katalogning yarim holatini
// ko'rib qolish xavfini yo'q qiladi.
type Catalog struct {
	mu         sync.Mutex
	clock      Clock
	staleAfter time.Duration
	view       atomic.Pointer[View]
}

func NewCatalog(clock Clock, staleAfter time.Duration) *Catalog {
	if clock == nil {
		clock = RealClock{}
	}
	c := &Catalog{clock: clock, staleAfter: staleAfter}
	c.view.Store(&View{})
	return c
}

// Snapshot — Django'dan olingan bitta to'liq katalog holati.
type Snapshot struct {
	Products   []Product
	Variants   []Variant
	Categories []Category
	CalcRules  []CalcRule
	Services   []Service
	Tiers      []DiscountTier
	Policy     *DiscountPolicy
	Defaults   *OrderDefaults
	Fasons     []Fason
	Knowledge  []KnowledgeRule
	Prompt     *PromptVersion
	Version    int64
}

// Delta — Watch oqimidan keladigan bitta ketma-ket katalog o'zgarishi.
type Delta struct {
	Version  int64
	Entity   string
	EntityID string
	Action   string
	Payload  any
}

func (c *Catalog) ApplySnapshot(s Snapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.view.Store(buildView(s, c.clock.Now(), c.staleAfter, false))
}

func (c *Catalog) ApplyDelta(d Delta) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	old := c.view.Load()
	if d.Version <= old.Version {
		return nil
	}
	if d.Version != old.Version+1 {
		return ErrVersionGap
	}
	next := cloneView(old)
	if err := applyDelta(next, d); err != nil {
		return err
	}
	next.Version = d.Version
	next.SyncedAt = c.clock.Now()
	next.Stale = false
	c.view.Store(next)
	return nil
}

func (c *Catalog) Version() int64 { return c.view.Load().Version }

func (c *Catalog) SyncedAt() time.Time { return c.view.Load().SyncedAt }

// View stale qiymatini chaqiruv vaqtida hisoblaydi, lekin atomic snapshotning
// o'zini o'zgartirmaydi — parallel o'qishlar race'ga tushmasligi uchun shunday.
func (c *Catalog) View() *View {
	v := c.view.Load()
	copy := *v
	copy.Stale = v.Ready() && c.staleAfter >= 0 && c.clock.Now().Sub(v.SyncedAt) > c.staleAfter
	return &copy
}

func (v *View) Ready() bool { return !v.SyncedAt.IsZero() }

func (v *View) Product(id string) (Product, bool) {
	p, ok := v.products[id]
	return cloneProduct(p), ok
}
func (v *View) Variant(id string) (Variant, bool) {
	x, ok := v.variants[id]
	return cloneVariant(x), ok
}

func (v *View) VariantsOf(productID string) []Variant {
	ids := v.byProduct[productID]
	result := make([]Variant, 0, len(ids))
	for _, id := range ids {
		if x, ok := v.variants[id]; ok {
			result = append(result, cloneVariant(x))
		}
	}
	return result
}

func (v *View) Products() []Product {
	result := make([]Product, 0, len(v.products))
	for _, p := range v.products {
		result = append(result, cloneProduct(p))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		return result[i].ID < result[j].ID
	})
	return result
}

func (v *View) Category(id string) (Category, bool) {
	x, ok := v.categories[id]
	return cloneCategory(x), ok
}
func (v *View) CalcRule(key string) (decimal.Decimal, bool) { x, ok := v.calcRules[key]; return x, ok }
func (v *View) Service(key string) (Service, bool) {
	x, ok := v.services[key]
	return cloneService(x), ok
}
func (v *View) DiscountTiers() []DiscountTier { return cloneTiers(v.tiers) }
func (v *View) DiscountPolicy() (DiscountPolicy, bool) {
	if v.policy == nil {
		return DiscountPolicy{}, false
	}
	return *v.policy, true
}
func (v *View) OrderDefaults() (OrderDefaults, bool) {
	if v.defaults == nil {
		return OrderDefaults{}, false
	}
	return *v.defaults, true
}
func (v *View) Fasons() []Fason            { return cloneFasons(v.fasons) }
func (v *View) Knowledge() []KnowledgeRule { return cloneKnowledge(v.knowledge) }
func (v *View) Prompt() (PromptVersion, bool) {
	if v.prompt == nil {
		return PromptVersion{}, false
	}
	return *v.prompt, true
}

func buildView(s Snapshot, now time.Time, _ time.Duration, stale bool) *View {
	v := &View{Version: s.Version, SyncedAt: now, Stale: stale, products: map[string]Product{}, variants: map[string]Variant{}, byProduct: map[string][]string{}, categories: map[string]Category{}, calcRules: map[string]decimal.Decimal{}, services: map[string]Service{}}
	for _, p := range s.Products {
		v.products[p.ID] = cloneProduct(p)
	}
	for _, x := range s.Variants {
		v.variants[x.ID] = cloneVariant(x)
		v.byProduct[x.ProductID] = append(v.byProduct[x.ProductID], x.ID)
	}
	for id := range v.byProduct {
		sort.Strings(v.byProduct[id])
	}
	for _, x := range s.Categories {
		v.categories[x.ID] = cloneCategory(x)
	}
	for _, x := range s.CalcRules {
		v.calcRules[x.Key] = x.Value
	}
	for _, x := range s.Services {
		v.services[x.Key] = cloneService(x)
	}
	v.tiers, v.fasons, v.knowledge = cloneTiers(s.Tiers), cloneFasons(s.Fasons), cloneKnowledge(s.Knowledge)
	if s.Policy != nil {
		x := *s.Policy
		v.policy = &x
	}
	if s.Defaults != nil {
		x := *s.Defaults
		v.defaults = &x
	}
	if s.Prompt != nil {
		x := *s.Prompt
		v.prompt = &x
	}
	return v
}

func cloneView(v *View) *View {
	n := &View{Version: v.Version, SyncedAt: v.SyncedAt, Stale: v.Stale, products: make(map[string]Product, len(v.products)), variants: make(map[string]Variant, len(v.variants)), byProduct: make(map[string][]string, len(v.byProduct)), categories: make(map[string]Category, len(v.categories)), calcRules: make(map[string]decimal.Decimal, len(v.calcRules)), services: make(map[string]Service, len(v.services)), tiers: cloneTiers(v.tiers), fasons: cloneFasons(v.fasons), knowledge: cloneKnowledge(v.knowledge)}
	for id, x := range v.products {
		n.products[id] = cloneProduct(x)
	}
	for id, x := range v.variants {
		n.variants[id] = x
	}
	for id, x := range v.byProduct {
		n.byProduct[id] = append([]string(nil), x...)
	}
	for id, x := range v.categories {
		n.categories[id] = x
	}
	for id, x := range v.calcRules {
		n.calcRules[id] = x
	}
	for id, x := range v.services {
		n.services[id] = x
	}
	if v.policy != nil {
		x := *v.policy
		n.policy = &x
	}
	if v.defaults != nil {
		x := *v.defaults
		n.defaults = &x
	}
	if v.prompt != nil {
		x := *v.prompt
		n.prompt = &x
	}
	return n
}

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

func cloneProduct(x Product) Product {
	x.RecommendedRooms = append([]string(nil), x.RecommendedRooms...)
	x.VariantIDs = append([]string(nil), x.VariantIDs...)
	x.ImageURLs = append([]string(nil), x.ImageURLs...)
	if x.Tags != nil {
		x.Tags = cloneAnyMap(x.Tags)
	}
	return x
}
func cloneVariant(x Variant) Variant             { return x }
func cloneCategory(x Category) Category          { return x }
func cloneService(x Service) Service             { return x }
func cloneTiers(x []DiscountTier) []DiscountTier { return append([]DiscountTier(nil), x...) }
func cloneFason(x Fason) Fason {
	x.Rooms = append([]string(nil), x.Rooms...)
	x.Parts = append([]FasonPart(nil), x.Parts...)
	x.Accessories = append([]FasonAccessory(nil), x.Accessories...)
	return x
}
func cloneFasons(x []Fason) []Fason {
	r := make([]Fason, len(x))
	for i := range x {
		r[i] = cloneFason(x[i])
	}
	return r
}
func cloneKnowledgeRule(x KnowledgeRule) KnowledgeRule {
	x.Condition = cloneAnyMap(x.Condition)
	return x
}
func cloneKnowledge(x []KnowledgeRule) []KnowledgeRule {
	r := make([]KnowledgeRule, len(x))
	for i := range x {
		r[i] = cloneKnowledgeRule(x[i])
	}
	return r
}
func cloneAnyMap(x map[string]any) map[string]any {
	if x == nil {
		return nil
	}
	r := make(map[string]any, len(x))
	for k, v := range x {
		r[k] = v
	}
	return r
}
