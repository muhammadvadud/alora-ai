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
	tenantID   string
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

// Touch — Watch heartbeat'i kelganda chaqiriladi: katalog o'zgarmagan, lekin aloqa
// tirik — SyncedAt yangilanadi, aks holda tinch katalog stale_after'dan keyin
// noto'g'ri "stale" bo'lardi (rules §4.1.5: stale = aloqa yo'qligi belgisi).
func (c *Catalog) Touch() {
	old := c.view.Load()
	if !old.Ready() {
		return
	}
	next := cloneView(old)
	next.SyncedAt = c.clock.Now()
	next.Stale = false
	c.view.Store(next)
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

func (v *View) ForTenant(tenantID string) *View {
	if tenantID == "" {
		return v
	}
	copy := *v
	copy.tenantID = tenantID
	return &copy
}

func (v *View) isValidTenant(itemTenantID string) bool {
	if v.tenantID == "" || itemTenantID == "" {
		return true
	}
	return v.tenantID == itemTenantID
}

func (v *View) Product(id string) (Product, bool) {
	p, ok := v.products[id]
	if ok && !v.isValidTenant(p.TenantID) {
		return Product{}, false
	}
	return cloneProduct(p), ok
}
func (v *View) Variant(id string) (Variant, bool) {
	x, ok := v.variants[id]
	if ok && !v.isValidTenant(x.TenantID) {
		return Variant{}, false
	}
	return cloneVariant(x), ok
}

func (v *View) VariantsOf(productID string) []Variant {
	ids := v.byProduct[productID]
	result := make([]Variant, 0, len(ids))
	for _, id := range ids {
		if x, ok := v.variants[id]; ok {
			if !v.isValidTenant(x.TenantID) {
				continue
			}
			result = append(result, cloneVariant(x))
		}
	}
	return result
}

func (v *View) Products() []Product {
	result := make([]Product, 0, len(v.products))
	for _, p := range v.products {
		if !v.isValidTenant(p.TenantID) {
			continue
		}
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
	if ok && !v.isValidTenant(x.TenantID) {
		return Category{}, false
	}
	return cloneCategory(x), ok
}
func (v *View) CalcRule(key string) (decimal.Decimal, bool) { x, ok := v.calcRules[key]; return x, ok }

func (v *View) Limits() Limits {
	l := DefaultLimits()

	if val, ok := v.CalcRule("min_cornice_length_m"); ok && val.GreaterThan(decimal.Zero) {
		l.MinLength = val
	}
	if val, ok := v.CalcRule("max_cornice_length_m"); ok && val.GreaterThan(decimal.Zero) {
		l.MaxLength = val
	}
	if val, ok := v.CalcRule("min_height_m"); ok && val.GreaterThan(decimal.Zero) {
		l.MinHeight = val
	}
	if val, ok := v.CalcRule("max_height_m"); ok && val.GreaterThan(decimal.Zero) {
		l.MaxHeight = val
	}
	if val, ok := v.CalcRule("min_coef"); ok && val.GreaterThan(decimal.Zero) {
		l.MinCoef = val
	}
	if val, ok := v.CalcRule("max_coef"); ok && val.GreaterThan(decimal.Zero) {
		l.MaxCoef = val
	}

	if l.MinLength.GreaterThan(l.MaxLength) {
		l.MinLength = DefaultLimits().MinLength
		l.MaxLength = DefaultLimits().MaxLength
	}
	if l.MinHeight.GreaterThan(l.MaxHeight) {
		l.MinHeight = DefaultLimits().MinHeight
		l.MaxHeight = DefaultLimits().MaxHeight
	}
	if l.MinCoef.GreaterThan(l.MaxCoef) {
		l.MinCoef = DefaultLimits().MinCoef
		l.MaxCoef = DefaultLimits().MaxCoef
	}

	return l
}
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
func (v *View) Fasons() []Fason {
	fasons := cloneFasons(v.fasons)
	if v.tenantID == "" {
		return fasons
	}
	result := make([]Fason, 0, len(fasons))
	for _, f := range fasons {
		if v.isValidTenant(f.TenantID) {
			result = append(result, f)
		}
	}
	return result
}
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
