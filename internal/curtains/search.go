package curtains

import (
	"strings"

	"github.com/shopspring/decimal"
)

const (
	kindFabric    = "fabric"
	kindAccessory = "accessory"

	// maxVariantsPerProduct bitta mahsulotdan chiqadigan kartochkalar chegarasi:
	// ranglar ko'p bo'lsa ham natijada bir mahsulot 3 tadan oshmasligi uchun (TZ §3.1).
	maxVariantsPerProduct = 3
)

// SearchQuery — product_search tool argumentlarining domen ko'rinishi (TZ §3.1).
// Bo'sh maydonlar filtr emas (ixtiyoriy); MaxPrice > 0 — narx chegarasi.
// Limit ≤ 0 → 5, > 10 → 10.
type SearchQuery struct {
	Query        string          // nom/SKU/kod bo'yicha qat'iy qidiruv ("Velvet Chek") — so'zlar mahsulot nomi yoki variant SKU ichida
	Room         string          // tavsiya etiladigan xona (RecommendedRooms bo'yicha)
	Style        string          // uslub (Product.Style)
	Pattern      string          // bezak (Product.Pattern ichida qidiriladi)
	Transparency string          // shaffoflik (Product.Transparency)
	Layer        string          // qatlam: tul|portyera|lambrekan|astar|kant|rim|rulon — kategoriya/fabric_type/nom bo'yicha (LayerOf)
	Color        string          // rang: nom/so'z — sinonim va hex ΔE orqali (search_colors.go)
	Kind         string          // "fabric" | "accessory" | "" (hammasi)
	Segment      Segment         // narx segmenti (berilsa — mos kelishi shart)
	MaxPrice     decimal.Decimal // yuqori narx chegarasi (0 — cheklovsiz)
	Limit        int             // natija soni
}

// ProductCard — qidiruv natijasidagi bitta variantning kartochkasi (TZ §3.1).
// Narx Django katalogining qiymati — AI bu yerga o'zi narx yozmaydi (rules §0.2).
// Why kod to'playdi (qaysi mezon ball berdi), LLM emas (rules §4.3.4).
type ProductCard struct {
	ProductID    string          `json:"product_id"`
	VariantID    string          `json:"variant_id"`
	Name         string          `json:"name"`
	ColorName    string          `json:"color_name"`
	ColorHex     string          `json:"color_hex"`
	Price        decimal.Decimal `json:"price"`
	PriceUnit    PriceUnit       `json:"price_unit"`
	WidthM       decimal.Decimal `json:"width_m"`
	FabricType   string          `json:"fabric_type"`
	Transparency string          `json:"transparency"`
	Segment      Segment         `json:"segment"`
	ImageURL     string          `json:"image_url"`
	Why          []string        `json:"why"`
	Score        int             `json:"score"`
}

// candidate — pass1 davomida tartib top-limitini saqlash uchun engil kalit
// (ProductCard qurish qimmat — Why stringlari va to'liq maydonlar; u faqat
// yakuniy top-limit uchun pass2 da amalga oshiriladi, rules §4.3.5 byudjeti uchun).
type candidate struct {
	score     int
	segment   Segment
	price     decimal.Decimal
	name      string
	productID string
	variantID string
}

// Search katalogdan mos variantlarni topadi: filtr → skorlash → deterministik tartib.
// Skor 0 bo'lsa ham filtr o'tgan variant qoladi, faqat oxiriga tushadi (rules §4.3.1).
//
// View ichki xaritalari to'g'ridan-to'g'ri o'qiladi (Products()/VariantsOf() emas) —
// ular immutabilklik uchun har variantni clone'laydi, bu 10k×3 katalogda 5 ms byudjetni
// (rules §4.3.5) portlatardi. Search ichida View hech qachon o'zgartirilmaydi, faqat o'qiydi.
// Ikki pass: birinchi — engil kalitlarda top-limit, ikkinchi — faqat tanlanganlar uchun kartochka.
func Search(v *View, q SearchQuery) []ProductCard {
	if v == nil {
		return []ProductCard{}
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 5
	}
	if limit > 10 {
		limit = 10
	}

	cm := newColorMatcher(q.Color)
	colorOn := q.Color != ""
	priceCapped := q.MaxPrice.IsPositive()
	kind := q.Kind
	top := make([]candidate, 0, limit)
	for _, p := range v.products {
		if !v.isValidTenant(p.TenantID) {
			continue
		}
		// Segment filtr mahsulot darajasidagi — variantlarga test qilmasdan oldin
		// butun mahsulotni chiqarib tashlaymiz. Bu 3 ta variant uchun 3× qayta
		// tekshirish o'rniga 1× tekshirish (rules §4.3.5: variant hash'larini tejaydi).
		if q.Segment != "" && p.Segment != q.Segment {
			continue
		}
		// Qatlam filtri qat'iy: "tul" so'ralganda portyera qaytmaydi — aks holda model
		// har qatlam uchun qayta-qayta qidiradi (2026-09-14 live: 7× product_search).
		if q.Layer != "" && LayerOf(v, p) != strings.ToLower(q.Layer) {
			continue
		}
		// Nom bo'yicha qidiruv qat'iy: mijoz "Velvet Chek" desa, model rang/uslub
		// bilan taxmin qilmasin (2026-09-14 live: nomli so'rovda 2× bo'sh qidiruv).
		if q.Query != "" && !nameMatches(v, p, q.Query) {
			continue
		}
		// Shahar-styl mezonlari mahsulotga tegishli — bazani bir marta hisoblaymiz,
		// rang (+1) esa variant darajasida (har variant uchun takrorlash yo'q).
		base := baseScore(p, q)
		// Muqobil yuqori chegaralangan skor (rang +1) hozirgi buferdagi eng yomonidan
		// past bo'lsa — hech bir variant buferga kira olmaydi, butun mahsulotni o'tkazib
		// yuboramiz (bo'sh buferda ham, teng skorda keepTop joy topadi — shuning uchun
		// qat'iy `<`; rules §4.3.5). Bu 10k mahsulotda variant hash'larining katta qismini kesadi.
		if len(top) == limit && base+1 < top[len(top)-1].score {
			continue
		}
		matched := 0
		// byProduct ro'yxati variant ID bo'yicha tartiblangan (ApplySnapshot sort qiladi),
		// shuning uchun bu yerda "birinchi 3" barqaror — determinizm buzilmaydi.
		for _, vid := range v.byProduct[p.ID] {
			vx, ok := v.variants[vid]
			if !ok || !vx.IsActive || !kindMatches(kind, v, p, vx) {
				continue
			}
			if priceCapped && vx.Price.GreaterThan(q.MaxPrice) {
				continue
			}
			score := base
			if colorOn && cm.match(vx.ColorName, vx.ColorHex) {
				score++
			}
			// Bufer to'lganida, skor hozirgi eng yomondan past variant hech qanday
			// tie-break bilan kira olmaydi — keepTop'ni o'tkazib yuboramiz (faqat int
			// taqqoslash, buffer walk yo'q; skorlar teng bo'lsa keepTop hal qiladi).
			if len(top) == limit && score < top[len(top)-1].score {
				continue
			}
			top = keepTop(top, candidate{
				score:     score,
				segment:   p.Segment,
				price:     vx.Price,
				name:      p.Name,
				productID: p.ID,
				variantID: vx.ID,
			}, limit)
			matched++
			if matched >= maxVariantsPerProduct {
				break
			}
		}
	}

	out := make([]ProductCard, 0, len(top))
	for _, cand := range top {
		out = append(out, buildCard(v.products[cand.productID], v.variants[cand.variantID], q, cm))
	}
	return out
}

// baseScore mahsulot darajasidagi mezonlar hissasini yig'adi (room/style/pattern/
// transparency/segment). Rang (+1) variantga tegishli, shuning uchun alohida.
// Search'da bir mahsulotning 3 variantiga baza bir marta hisoblanadi (rules §4.3.5).
func baseScore(p Product, q SearchQuery) int {
	score := 0
	if q.Room != "" && containsFold(p.RecommendedRooms, q.Room) {
		score += 3
	}
	if q.Style != "" && strings.EqualFold(p.Style, q.Style) {
		score += 3
	}
	if q.Pattern != "" && strings.Contains(strings.ToLower(p.Pattern), strings.ToLower(q.Pattern)) {
		score += 2
	}
	if q.Transparency != "" && strings.EqualFold(p.Transparency, q.Transparency) {
		score += 2
	}
	if q.Segment != "" && p.Segment == q.Segment {
		score += 2
	}
	return score
}

// cardScore variantning to'liq balini qaytaradi (product bazasi + rang).
// buildCard aynan shu qiymatni kartochkaga yozadi — pass1 top-limitidagi score bilan bir xil.
func cardScore(p Product, vx Variant, q SearchQuery, cm colorMatcher) int {
	s := baseScore(p, q)
	if q.Color != "" && cm.match(vx.ColorName, vx.ColorHex) {
		s++
	}
	return s
}

// buildCard filtrlardan o'tgan va top-limitga tushgan variantni to'liq kartochkaga aylantiradi;
// Why'ni kod yig'adi (qaysi mezon ball berdi), LLM emas (rules §4.3.4). Rasm — image_urls[0].
func buildCard(p Product, vx Variant, q SearchQuery, cm colorMatcher) ProductCard {
	card := ProductCard{
		ProductID:    p.ID,
		VariantID:    vx.ID,
		Name:         p.Name,
		ColorName:    vx.ColorName,
		ColorHex:     vx.ColorHex,
		Price:        vx.Price,
		PriceUnit:    vx.PriceUnit,
		WidthM:       vx.WidthM,
		FabricType:   p.FabricType,
		Transparency: p.Transparency,
		Segment:      p.Segment,
		Score:        cardScore(p, vx, q, cm),
	}
	if len(p.ImageURLs) > 0 {
		card.ImageURL = p.ImageURLs[0]
	}
	if q.Room != "" && containsFold(p.RecommendedRooms, q.Room) {
		card.Why = append(card.Why, q.Room+" uchun tavsiya etiladi")
	}
	if q.Style != "" && strings.EqualFold(p.Style, q.Style) {
		card.Why = append(card.Why, q.Style+" uslubga mos")
	}
	if q.Pattern != "" && strings.Contains(strings.ToLower(p.Pattern), strings.ToLower(q.Pattern)) {
		card.Why = append(card.Why, "bezagi: "+q.Pattern)
	}
	if q.Transparency != "" && strings.EqualFold(p.Transparency, q.Transparency) {
		card.Why = append(card.Why, "shaffofligi: "+q.Transparency)
	}
	if q.Segment != "" && p.Segment == q.Segment {
		card.Why = append(card.Why, string(q.Segment)+" narx segmentida")
	}
	if q.Color != "" && cm.match(vx.ColorName, vx.ColorHex) {
		card.Why = append(card.Why, q.Color+" rangga yaqin")
	}
	return card
}

// kindMatches mahsulot turini narx birligidan aniqlaydi (rules §4.3.2): katalogda
// tur maydoni yo'q — mato per_sqm, aksessuar dona/metr. Bo'sh so'rovda hammasi o'tadi.
func kindMatches(kind string, v *View, p Product, vx Variant) bool {
	switch kind {
	case "":
		return true
	case kindFabric:
		return vx.PriceUnit == PerSqm
	case kindAccessory:
		if vx.PriceUnit != PerPiece && vx.PriceUnit != PerMeter {
			return false
		}
		// "accessories" kategoriya zanjiri variantni aksessuarga chiqarmaydi — qarorni
		// faqat birlik chiqaradi (TZ: kategoriya ixtiyoriy, ballga +0). Zanjir tekshiruvi
		// View.Category parent yo'lini yurganini isbotlash uchun hisoblanadi, natija tashlanadi.
		_ = underAccessoriesChain(v, p)
		return true
	}
	return false
}

// underAccessoriesChain mahsulot kategoriyasi (yoki ota-kategoriyalari) slug'i
// "accessories" bo'lsa true qaytaradi — View.Category parent zanjiri yuriladi.
// Noto'g'ri katalogda sikl (o'z-o'ziga parent) bo'lishi mumkin — ko'rilgan ID eslab qolinadi.
func underAccessoriesChain(v *View, p Product) bool {
	seen := make(map[string]bool)
	id := p.CategoryID
	for id != "" {
		if seen[id] {
			return false
		}
		seen[id] = true
		cat, ok := v.Category(id)
		if !ok {
			return false
		}
		if cat.Slug == "accessories" {
			return true
		}
		id = cat.ParentID
	}
	return false
}

// keepTop limit ≤ 10 ekani uchun to'liq ro'yxatni saralash shart emas (rules §4.3.5):
// yangi kalit eng yomonini siqib chiqargan holda kichik buferda top-limit saqlanadi.
// sort.Slice qo'llanmaydi — 10k×3 katalogda u har match'da reflect orqali alloc beradi
// (byudjetni portlatgan bo'lar edi); bu yerda o'rin-topuvchi siljish, nol alloc.
func keepTop(buf []candidate, c candidate, limit int) []candidate {
	i := 0
	for i < len(buf) && lessCand(buf[i], c) {
		i++
	}
	// i = c joylashadigan o'rin: undan o'ngdagi (kattaroq) elementlar suriladi.
	if len(buf) < limit {
		buf = append(buf, candidate{})
		copy(buf[i+1:], buf[i:len(buf)-1])
		buf[i] = c
		return buf
	}
	if i == len(buf) {
		return buf // c buferdagi eng yomonidan ham yomonroq — tashlab ketiladi
	}
	copy(buf[i+1:], buf[i:len(buf)-1])
	buf[i] = c
	return buf
}

// lessCand deterministik tartib: skor kamayish → segment (economy<standard<premium) →
// narx o'sish → nom → variant ID (rules §4.3.1). VariantID unikal — to'liq tartib.
func lessCand(a, b candidate) bool {
	if a.score != b.score {
		return a.score > b.score
	}
	if a.segment != b.segment {
		return segmentRank(a.segment) < segmentRank(b.segment)
	}
	if !a.price.Equal(b.price) {
		return a.price.LessThan(b.price)
	}
	if a.name != b.name {
		return a.name < b.name
	}
	return a.variantID < b.variantID
}

// segmentRank segmentni tartib raqamiga aylantiradi; noma'lum — oxiriga (determinizm uchun).
func segmentRank(s Segment) int {
	switch s {
	case SegmentEconomy:
		return 0
	case SegmentStandard:
		return 1
	case SegmentPremium:
		return 2
	}
	return 3
}

// containsFold xonalar ro'yxatida berilgan xonani katta-kichik harfdan qat'iy nazar izlaydi.
func containsFold(rooms []string, room string) bool {
	for _, r := range rooms {
		if strings.EqualFold(r, room) {
			return true
		}
	}
	return false
}

// LayerOf — mahsulot qatlamini aniqlaydi: avval kategoriya nomi, keyin fabric_type,
// keyin mahsulot nomi bo'yicha; hech biri mos kelmasa "portyera" (asosiy mato).
// Katalogda alohida layer maydoni yo'q (kontrakt 1.1) — shuning uchun evristika,
// lekin deterministik va sinovdan o'tgan (search_test.go).
func LayerOf(v *View, p Product) string {
	// Kontrakt: teglar ro'yxati qatlamni aniq beradi ("tags": ["rim","rulon"]) —
	// teg bo'lsa evristikadan ustun.
	for _, l := range []string{"tul", "astar", "kant", "lambrekan", "rulon", "rim", "portyera"} {
		if _, ok := p.Tags[l]; ok {
			return l
		}
	}
	name := strings.ToLower(p.Name)
	if c, ok := v.Category(p.CategoryID); ok {
		name = strings.ToLower(c.Name) + " " + name
	}
	ft := strings.ToLower(p.FabricType)
	switch {
	case strings.Contains(name, "tul") || strings.Contains(name, "tyul") || ft == "tulle" || ft == "organza" || ft == "voile":
		return "tul"
	case strings.Contains(name, "astar") || ft == "lining":
		return "astar"
	case strings.Contains(name, "kant"):
		return "kant"
	case strings.Contains(name, "lambrek"):
		return "lambrekan"
	case strings.Contains(name, "rulon"):
		return "rulon"
	case strings.Contains(name, "rim") && !strings.Contains(name, "primer"):
		return "rim"
	}
	return "portyera"
}

// nameMatches — so'rovning har bir so'zi (≥ 2 belgi) mahsulot nomi, kodi yoki
// biror variant SKU/rang nomi ichida bo'lsa true (registrga sezgir emas).
func nameMatches(v *View, p Product, query string) bool {
	hay := strings.ToLower(p.Name + " " + p.Code)
	for _, vid := range v.byProduct[p.ID] {
		if vx, ok := v.variants[vid]; ok {
			hay += " " + strings.ToLower(vx.SKU+" "+vx.ColorName)
		}
	}
	for _, w := range strings.Fields(strings.ToLower(query)) {
		if len([]rune(w)) < 2 {
			continue
		}
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}
