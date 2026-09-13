// Package curtains — Alora parda domeni: katalog turlari, hisob, qidiruv, holat.
//
// Bu paket LLM, gRPC va Eino'dan MUSTAQIL (rules.md §2): faqat stdlib va decimal.
// Django — haqiqat manbai, bu yerdagi turlar uning nusxasi (proto Catalog* xabarlari).
package curtains

import (
	"time"

	"github.com/shopspring/decimal"
)

// PriceUnit — narx birligi. Django `PriceUnit` bilan bir xil qiymatlar.
type PriceUnit string

const (
	PerSqm   PriceUnit = "per_sqm"   // mato — m² bo'yicha
	PerMeter PriceUnit = "per_meter" // bahroma, tasma — metr bo'yicha
	PerPiece PriceUnit = "per_piece" // gul, podxvat, montaj — dona bo'yicha
)

// PartRole — parda bo'lagining roli. Django `CurtainPartRole` bilan bir xil.
type PartRole string

const (
	RolePortyera  PartRole = "portyera"
	RoleTul       PartRole = "tul"
	RoleLambrekan PartRole = "lambrekan"
	RoleKant      PartRole = "kant"
	RoleDekor     PartRole = "dekor"
	RoleAstar     PartRole = "astar"
)

// Segment — narx segmenti (product_search va build_options uchun).
type Segment string

const (
	SegmentEconomy  Segment = "economy"
	SegmentStandard Segment = "standard"
	SegmentPremium  Segment = "premium"
)

// Variant — mahsulotning rang/narx varianti (proto CatalogVariant).
// Narx decimal — float64 pul uchun taqiqlangan (rules §1.5).
type Variant struct {
	ID        string
	ProductID string
	SKU       string
	ColorName string
	ColorHex  string
	WidthM    decimal.Decimal
	Price     decimal.Decimal
	PriceUnit PriceUnit
	OldPrice  decimal.Decimal
	IsActive  bool
}

// Product — katalog mahsuloti (proto CatalogProduct); variantlar alohida saqlanadi.
type Product struct {
	ID               string
	CategoryID       string
	Name             string
	Code             string
	Description      string
	FabricType       string
	Pattern          string
	Style            string
	Transparency     string
	Density          decimal.Decimal
	Composition      string
	Care             string
	RecommendedRooms []string
	RecommendedCoef  decimal.Decimal
	DefaultWidthM    decimal.Decimal
	IsSewingRequired bool
	Segment          Segment
	Tags             map[string]any
	VariantIDs       []string
	ImageURLs        []string
}

// Category — mahsulot kategoriyasi (proto CatalogCategory).
type Category struct {
	ID        string
	Name      string
	Slug      string
	ParentID  string
	SortOrder int
}

// CalcRule — hisob koeffitsienti/qoidasi (Django CalcRule). Kalitlar calc_cases.json bilan bir xil.
type CalcRule struct {
	Key         string
	Value       decimal.Decimal
	Unit        string
	Description string
	AppliesTo   map[string]any
}

// Service — xizmat narxi: sewing, tape, hook, install, delivery, cornice (Django ServicePrice).
type Service struct {
	Key      string
	Name     string
	Price    decimal.Decimal
	Unit     PriceUnit
	IsActive bool
}

// DiscountTier — summaga qarab o'sadigan chegirma bosqichi.
type DiscountTier struct {
	ID        string
	MinAmount decimal.Decimal
	Percent   decimal.Decimal
	IsActive  bool
}

// DiscountPolicy — qo'lda chegirma chegaralari (rol bo'yicha).
type DiscountPolicy struct {
	ManualMaxPercentEmployee decimal.Decimal
	ManualMaxPercentManager  decimal.Decimal
	StackWithTier            bool
}

// OrderDefaults — avans, yetkazish va muddat defaultlari (manager kiritmasa ishlaydi).
type OrderDefaults struct {
	PrepaymentPercent decimal.Decimal
	DeliveryAmount    decimal.Decimal
	DeadlineDays      int
}

// FasonPart — fason shablonidagi bo'lak: karniz/balandlikdan ulush va koeffitsient.
type FasonPart struct {
	ID          string
	Role        PartRole
	Name        string
	WidthShare  decimal.Decimal
	HeightShare decimal.Decimal
	DefaultCoef decimal.Decimal
	SortOrder   int
}

// FasonAccessory — fasonga biriktirilgan standart bezak (kategoriya bo'yicha).
type FasonAccessory struct {
	ID         string
	Category   string
	DefaultQty decimal.Decimal
	QtyFormula string // "" | per_side | per_meter
}

// Fason — tayyor fason shabloni (Django Fason + parts + accessories).
type Fason struct {
	ID          string
	Name        string
	Style       string
	Rooms       []string
	ImageURL    string
	PartCount   int
	Parts       []FasonPart
	Accessories []FasonAccessory
}

// KnowledgeRule — sotuv bilimi qoidasi (Django KnowledgeRule).
type KnowledgeRule struct {
	ID             string
	Kind           string
	Condition      map[string]any
	Recommendation string
	Priority       int
}

// PromptVersion — Django'da versiyalangan system prompt.
type PromptVersion struct {
	Version   int
	Content   string
	UpdatedAt time.Time
}

// UserRole — chegirma chegarasi uchun rol (Django UserRole).
type UserRole string

const (
	RoleEmployee UserRole = "employee"
	RoleManager  UserRole = "manager"
	RoleAdmin    UserRole = "admin"
)

// Valid — chegirma berishga ruxsatli rol (employee|manager). RoleAdmin yoki
// noma'lum qiymat fail-closed hisoblanadi: ai hech qachon tasksiz yuqori
// chegirma bermasligi uchun (rules §5.6) — bosh rol manager limitiga tushmaydi.
func (r UserRole) Valid() bool {
	switch r {
	case RoleEmployee, RoleManager:
		return true
	}
	return false
}

// ParseRole — Django'dan kelgan rol qiymatini tekshirilgan konstanta qiladi.
// Aniq kichik harf talab qilinadi: "MANAGER" yoki "admin" qabul qilinmaydi —
// ularni Valid() ko'rmaydi va price_quote rad qiladi.
func ParseRole(s string) (UserRole, bool) {
	r := UserRole(s)
	return r, r.Valid()
}
