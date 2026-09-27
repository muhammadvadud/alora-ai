package curtains

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// dec — testlarda decimal qiymat (float'dan emas — rules §1.5).
func dec(s string) decimal.Decimal {
	return decimal.RequireFromString(s)
}

// searchView kichik katalog snapshotidan o'zgarmas View qaytaradi (test faqat ma'lumotlari).
func searchView(s Snapshot) *View {
	c := NewCatalog(NewFakeClock(time.Now()), time.Hour)
	c.ApplySnapshot(s)
	return c.View()
}

// orderingFixture — tartib qoidalarini (skor → segment → narx → nom) tekshiradigan katalog.
func orderingFixture() Snapshot {
	return Snapshot{
		Version: 1,
		Products: []Product{
			{ID: "p-alfa", Name: "Alfa", Style: "klassik", RecommendedRooms: []string{"mehmonxona"}, Segment: SegmentStandard, ImageURLs: []string{"img/alfa.jpg"}, FabricType: "dubles", VariantIDs: []string{"v-a1", "v-a2"}},
			{ID: "p-beta", Name: "Beta", Style: "klassik", RecommendedRooms: []string{"mehmonxona"}, Segment: SegmentPremium, VariantIDs: []string{"v-b1"}},
			{ID: "p-gamma", Name: "Gamma", Style: "klassik", RecommendedRooms: []string{"mehmonxona"}, Segment: SegmentEconomy, VariantIDs: []string{"v-g1"}},
			{ID: "p-delta", Name: "Delta", Style: "provans", RecommendedRooms: []string{"yotoqxona"}, Segment: SegmentStandard, VariantIDs: []string{"v-d1"}},
		},
		Variants: []Variant{
			{ID: "v-a1", ProductID: "p-alfa", ColorName: "qora", ColorHex: "#000000", Price: dec("300000"), PriceUnit: PerSqm, IsActive: true, WidthM: dec("1.4")},
			{ID: "v-a2", ProductID: "p-alfa", ColorName: "oq", ColorHex: "#FFFFFF", Price: dec("200000"), PriceUnit: PerSqm, IsActive: true, WidthM: dec("1.4")},
			{ID: "v-b1", ProductID: "p-beta", ColorName: "oq", ColorHex: "#FFFFFF", Price: dec("400000"), PriceUnit: PerSqm, IsActive: true, WidthM: dec("1.4")},
			{ID: "v-g1", ProductID: "p-gamma", ColorName: "oq", ColorHex: "#FFFFFF", Price: dec("500000"), PriceUnit: PerSqm, IsActive: true, WidthM: dec("1.4")},
			{ID: "v-d1", ProductID: "p-delta", ColorName: "oq", ColorHex: "#FFFFFF", Price: dec("100000"), PriceUnit: PerSqm, IsActive: true, WidthM: dec("1.4")},
		},
	}
}

func TestSearch_Ordering_ScoreDescSegmentPriceName(t *testing.T) {
	v := searchView(orderingFixture())

	got := Search(v, SearchQuery{Room: "mehmonxona", Style: "klassik"})

	wantIDs := []string{"v-g1", "v-a2", "v-a1", "v-b1", "v-d1"}
	wantScores := []int{6, 6, 6, 6, 0}
	if len(got) != len(wantIDs) {
		t.Fatalf("natija soni %d, kutilgan %d", len(got), len(wantIDs))
	}
	for i := range wantIDs {
		if got[i].VariantID != wantIDs[i] {
			t.Fatalf("[%d] variant %q, kutilgan %q", i, got[i].VariantID, wantIDs[i])
		}
		if got[i].Score != wantScores[i] {
			t.Fatalf("[%d] skor %d, kutilgan %d", i, got[i].Score, wantScores[i])
		}
	}

	// Gamma skorda eng baland, uning why'si ikkala mos mezonni o'z ichiga oladi.
	g1 := got[0]
	if len(g1.Why) != 2 || g1.Why[0] != "mehmonxona uchun tavsiya etiladi" || g1.Why[1] != "klassik uslubga mos" {
		t.Fatalf("why noto'g'ri: %#v", g1.Why)
	}
	// Alfa kartochkasida rasm URL Django image_urls[0] dan olinadi.
	if got[1].ImageURL != "img/alfa.jpg" {
		t.Fatalf("image_url: %q", got[1].ImageURL)
	}
	// Delta skori 0 — lekin filtr o'tgani uchun ro'yxatda, eng oxirida.
	d1 := got[len(got)-1]
	if d1.Score != 0 || len(d1.Why) != 0 {
		t.Fatalf("delta skor 0 bo'lishi va why'si bo'sh bo'lishi kerak: %#v", d1)
	}
}

func TestSearch_DeterministicEqualInputs(t *testing.T) {
	v := searchView(orderingFixture())
	q := SearchQuery{Room: "mehmonxona", Style: "klassik"}

	first := Search(v, q)
	second := Search(v, q)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("bir xil kirish ikki marta chaqirilganda natija bir xil bo'lishi shart (determinizm)")
	}
}

// kindFixture — kind (mato/aksessuar) birlik bo'yicha ajratiladigan katalog.
// p-under mahsuloti "accessories" zanjiri ostida — kategoriya birlik qaroriga ta'sir etmasligi kerak.
func kindFixture() Snapshot {
	return Snapshot{
		Version: 1,
		Categories: []Category{
			{ID: "cat-root", Slug: "parda"},
			{ID: "cat-acc", Slug: "accessories", ParentID: "cat-root"},
			{ID: "cat-sub", Slug: "lesa", ParentID: "cat-acc"},
		},
		Products: []Product{
			{ID: "p-fab", Name: "Fabrika mato", CategoryID: "cat-root", VariantIDs: []string{"f1", "f2"}},
			{ID: "p-acc", Name: "Aksessuar aralash", CategoryID: "cat-root", VariantIDs: []string{"a1", "a2", "a3"}},
			{ID: "p-under", Name: "Aksessuar ostidagi", CategoryID: "cat-sub", VariantIDs: []string{"u1", "u2"}},
		},
		Variants: []Variant{
			{ID: "f1", ProductID: "p-fab", Price: dec("100000"), PriceUnit: PerSqm, IsActive: true},
			{ID: "f2", ProductID: "p-fab", Price: dec("15000"), PriceUnit: PerPiece, IsActive: true},
			{ID: "a1", ProductID: "p-acc", Price: dec("8000"), PriceUnit: PerMeter, IsActive: true},
			{ID: "a2", ProductID: "p-acc", Price: dec("12000"), PriceUnit: PerPiece, IsActive: true},
			{ID: "a3", ProductID: "p-acc", Price: dec("90000"), PriceUnit: PerSqm, IsActive: true},
			{ID: "u1", ProductID: "p-under", Price: dec("20000"), PriceUnit: PerPiece, IsActive: true},
			{ID: "u2", ProductID: "p-under", Price: dec("110000"), PriceUnit: PerSqm, IsActive: true},
		},
	}
}

func TestSearch_KindFabric_OnlyPerSqm(t *testing.T) {
	v := searchView(kindFixture())
	got := Search(v, SearchQuery{Kind: kindFabric})
	assertVariantIDs(t, got, []string{"a3", "f1", "u2"})
}

func TestSearch_KindAccessory_OnlyPieceOrMeter(t *testing.T) {
	v := searchView(kindFixture())
	got := Search(v, SearchQuery{Kind: kindAccessory})
	// u2 per_sqm ("accessories" zanjirida bo'lsa ham) chiqmaydi — faqat birlik hal qiladi.
	assertVariantIDs(t, got, []string{"a1", "a2", "f2", "u1"})
}

func TestSearch_Kind_EmptyMeansAll(t *testing.T) {
	v := searchView(kindFixture())
	got := Search(v, SearchQuery{Kind: "", Limit: 10})
	assertVariantIDs(t, got, []string{"a1", "a2", "f2", "u1", "a3", "f1", "u2"})
}

func TestSearch_UnderAccessoriesChain(t *testing.T) {
	v := searchView(kindFixture())

	under, _ := v.Product("p-under")
	if !underAccessoriesChain(v, under) {
		t.Fatal("p-under parent zanjiri (cat-sub → cat-acc) accessories slug'ini topishi kerak")
	}
	plain, _ := v.Product("p-fab")
	if underAccessoriesChain(v, plain) {
		t.Fatal("p-fab accessories zanjirida emas")
	}
}

func TestSearch_CategoryCycleDoesNotHang(t *testing.T) {
	s := Snapshot{
		Version:    1,
		Categories: []Category{{ID: "self", Slug: "node", ParentID: "self"}},
		Products:   []Product{{ID: "p", CategoryID: "self", VariantIDs: []string{"v1"}}},
		Variants:   []Variant{{ID: "v1", ProductID: "p", PriceUnit: PerSqm, IsActive: true}},
	}
	v := searchView(s)
	p, _ := v.Product("p")
	if underAccessoriesChain(v, p) {
		t.Fatal("o'z-o'ziga parent bo'lgan siklda false qaytishi kerak")
	}
}

// manyProducts Snapshot'i — limit chegaralarini tekshiradigan n xil mahsulot.
func manyProducts(n int) Snapshot {
	s := Snapshot{Version: 1}
	for i := 0; i < n; i++ {
		num := fmt.Sprintf("%02d", i)
		id := "p-" + num
		vid := "v-" + num
		s.Products = append(s.Products, Product{
			ID: id, Name: "Mahsulot " + num, Style: "klassik",
			RecommendedRooms: []string{"mehmonxona"}, Segment: SegmentEconomy, VariantIDs: []string{vid},
		})
		s.Variants = append(s.Variants, Variant{ID: vid, ProductID: id, Price: dec("100000"), PriceUnit: PerSqm, IsActive: true})
	}
	return s
}

func TestSearch_LimitBounds(t *testing.T) {
	v := searchView(manyProducts(12))
	q := SearchQuery{Room: "mehmonxona", Style: "klassik", Limit: 0}

	q.Limit = 0
	default5 := Search(v, q)
	if len(default5) != 5 {
		t.Fatalf("Limit=0 → 5 farz qilinishi kerak, keldi: %d", len(default5))
	}
	q.Limit = -3
	if got := Search(v, q); len(got) != 5 {
		t.Fatalf("Limit<0 → 5 farz, keldi: %d", len(got))
	}
	q.Limit = 5
	explicit5 := Search(v, q)
	if !reflect.DeepEqual(default5, explicit5) {
		t.Fatal("Limit=0 va Limit=5 natijalari bir xil bo'lishi kerak")
	}
	q.Limit = 10
	if got := Search(v, q); len(got) != 10 {
		t.Fatalf("Limit=10 → 10 ta, keldi: %d", len(got))
	}
	q.Limit = 50
	if got := Search(v, q); len(got) != 10 {
		t.Fatalf("Limit>10 → 10 gacha kesiladi, keldi: %d", len(got))
	}
}

func TestSearch_MaxPrice(t *testing.T) {
	s := Snapshot{Version: 1}
	prices := []string{"90000", "250000", "250001", "500000"}
	for i, price := range prices {
		id := fmt.Sprintf("p-%d", i)
		vid := fmt.Sprintf("v-%d", i)
		s.Products = append(s.Products, Product{ID: id, Name: id, VariantIDs: []string{vid}})
		s.Variants = append(s.Variants, Variant{ID: vid, ProductID: id, Price: dec(price), PriceUnit: PerSqm, IsActive: true})
	}
	v := searchView(s)

	if got := Search(v, SearchQuery{MaxPrice: dec("250000")}); len(got) != 2 {
		t.Fatalf("250000 chegarada 2 ta variant qolishi kerak, keldi: %d", len(got))
	}
	// MaxPrice nol (bo'sh) — cheklov yo'q.
	if got := Search(v, SearchQuery{}); len(got) != 4 {
		t.Fatalf("bo'sh MaxPrice hammasini berishi kerak, keldi: %d", len(got))
	}
}

func TestSearch_MaxThreeVariantsPerProduct(t *testing.T) {
	s := Snapshot{Version: 1}
	p := Product{ID: "p-multi", Name: "Ko'p rangli", VariantIDs: []string{"v1", "v2", "v3", "v4"}}
	s.Products = []Product{p}
	for i := 1; i <= 4; i++ {
		s.Variants = append(s.Variants, Variant{ID: fmt.Sprintf("v%d", i), ProductID: "p-multi", Price: dec("100000"), PriceUnit: PerSqm, IsActive: true})
	}
	v := searchView(s)

	got := Search(v, SearchQuery{Limit: 10})
	if len(got) != 3 {
		t.Fatalf("bitta mahsulotdan ko'pi bilan 3 ta variant, keldi: %d", len(got))
	}
	assertVariantIDs(t, got, []string{"v1", "v2", "v3"})
}

func TestSearch_InactiveVariantExcluded(t *testing.T) {
	s := Snapshot{
		Version:  1,
		Products: []Product{{ID: "p", Name: "Mato", VariantIDs: []string{"active", "dead"}}},
		Variants: []Variant{
			{ID: "active", ProductID: "p", PriceUnit: PerSqm, IsActive: true},
			{ID: "dead", ProductID: "p", PriceUnit: PerSqm, IsActive: false},
		},
	}
	v := searchView(s)
	assertVariantIDs(t, Search(v, SearchQuery{}), []string{"active"})
}

func TestSearch_EmptyResult(t *testing.T) {
	v := searchView(orderingFixture())
	// Xona skorlovchi mezon, filtr emas — bo'sh natijani MaxPrice orqali tekshiramiz.
	if got := Search(v, SearchQuery{MaxPrice: dec("1")}); len(got) != 0 {
		t.Fatalf("hech bir narx chegara ichida emas — bo'sh natija berishi kerak, keldi: %d", len(got))
	}

	empty := NewCatalog(NewFakeClock(time.Now()), time.Hour).View()
	if got := Search(empty, SearchQuery{}); len(got) != 0 {
		t.Fatalf("bo'sh katalog bo'sh natija berishi kerak, keldi: %d", len(got))
	}
	if got := Search(nil, SearchQuery{}); got == nil || len(got) != 0 {
		t.Fatal("nil View bo'sh, nil emas ro'yxat qaytarishi kerak")
	}
}

// segmentFixture — q.Segment filtri va segment skori (+2) uchun katalog.
func segmentFixture() Snapshot {
	s := Snapshot{Version: 1}
	for i, seg := range []Segment{SegmentEconomy, SegmentStandard, SegmentPremium} {
		id := fmt.Sprintf("p-%d", i)
		vid := fmt.Sprintf("v-%d", i)
		s.Products = append(s.Products, Product{ID: id, Name: id, Segment: seg, VariantIDs: []string{vid}})
		s.Variants = append(s.Variants, Variant{ID: vid, ProductID: id, Price: dec("150000"), PriceUnit: PerSqm, IsActive: true})
	}
	return s
}

func TestSearch_SegmentFilterAndScore(t *testing.T) {
	v := searchView(segmentFixture())

	got := Search(v, SearchQuery{Segment: SegmentStandard})
	if len(got) != 1 {
		t.Fatalf("faqat standard segment qolishi kerak, keldi: %d", len(got))
	}
	if got[0].Score != 2 {
		t.Fatalf("segment mosligi +2, keldi: %d", got[0].Score)
	}
	if len(got[0].Why) != 1 || got[0].Why[0] != "standard narx segmentida" {
		t.Fatalf("segment why noto'g'ri: %#v", got[0].Why)
	}

	if gotAll := Search(v, SearchQuery{}); len(gotAll) != 3 {
		t.Fatalf("bo'sh segment filtri hammasini berishi kerak, keldi: %d", len(gotAll))
	}
}

func TestSearch_ColorScoreAndWhy(t *testing.T) {
	s := Snapshot{
		Version:  1,
		Products: []Product{{ID: "p", Name: "Ipak", VariantIDs: []string{"v-grafit", "v-oq"}}},
		Variants: []Variant{
			{ID: "v-grafit", ProductID: "p", ColorName: "Grafit", ColorHex: "#404040", PriceUnit: PerSqm, IsActive: true},
			{ID: "v-oq", ProductID: "p", ColorName: "Oq", ColorHex: "#FFFFFF", PriceUnit: PerSqm, IsActive: true},
		},
	}
	v := searchView(s)

	got := Search(v, SearchQuery{Color: "grafit"})
	if len(got) != 2 {
		t.Fatalf("2 ta variant ham filtrdan o'tishi kerak, keldi: %d", len(got))
	}
	if got[0].VariantID != "v-grafit" || got[0].Score != 1 {
		t.Fatalf("grafit varianti +1 olib birinchi bo'lishi kerak: %#v", got[0])
	}
	if len(got[0].Why) != 1 || got[0].Why[0] != "grafit rangga yaqin" {
		t.Fatalf("rang why noto'g'ri: %#v", got[0].Why)
	}
	if got[1].VariantID != "v-oq" || got[1].Score != 0 || len(got[1].Why) != 0 {
		t.Fatalf("oq varianti skor olmasligi kerak: %#v", got[1])
	}
}

func TestSearch_ProductCardJSONTags(t *testing.T) {
	want := map[string]string{
		"ProductID": "product_id", "VariantID": "variant_id", "Name": "name",
		"ColorName": "color_name", "ColorHex": "color_hex", "Price": "price",
		"PriceUnit": "price_unit", "WidthM": "width_m", "FabricType": "fabric_type",
		"Transparency": "transparency", "Segment": "segment", "ImageURL": "image_url",
		"Why": "why", "Score": "score",
	}
	typ := reflect.TypeOf(ProductCard{})
	for field, tag := range want {
		f, ok := typ.FieldByName(field)
		if !ok {
			t.Fatalf("ProductCard.%s maydoni yo'q", field)
		}
		if got := f.Tag.Get("json"); got != tag {
			t.Errorf("ProductCard.%s json tegi %q, kutilgan %q", field, got, tag)
		}
	}
}

func assertVariantIDs(t *testing.T, got []ProductCard, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("natija soni %d, kutilgan %d (natija: %#v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i].VariantID != want[i] {
			t.Fatalf("variant[%d] = %q, kutilgan %q", i, got[i].VariantID, want[i])
		}
	}
}

// TestSearch_ExcludesReferenceProducts — etalon (REF-*) mahsulot tavsiyaga chiqmaydi,
// lekin View.Variant orqali hisob uchun topiladi (ADR-012; 2026-09-27 live).
func TestSearch_ExcludesReferenceProducts(t *testing.T) {
	v := searchView(Snapshot{
		Version: 1,
		Categories: []Category{
			{ID: "c-ref", Name: "Etalon", Slug: "etalon"},
			{ID: "c-ref-sub", Name: "Etalon mato", Slug: "etalon-mato", ParentID: "c-ref"},
			{ID: "c-mato", Name: "Matolar", Slug: "matolar"},
		},
		Products: []Product{
			{ID: "p-ref", Name: "REF-PORTYERA", CategoryID: "c-ref", VariantIDs: []string{"v-ref"}},
			{ID: "p-ref2", Name: "REF-TUL", CategoryID: "c-ref-sub", VariantIDs: []string{"v-ref2"}},
			{ID: "p-real", Name: "Blackout Duo", CategoryID: "c-mato", VariantIDs: []string{"v-real"}},
		},
		Variants: []Variant{
			{ID: "v-ref", ProductID: "p-ref", Price: dec("25000"), PriceUnit: PerSqm, IsActive: true},
			{ID: "v-ref2", ProductID: "p-ref2", Price: dec("8000"), PriceUnit: PerSqm, IsActive: true},
			{ID: "v-real", ProductID: "p-real", Price: dec("95000"), PriceUnit: PerSqm, IsActive: true},
		},
	})

	got := Search(v, SearchQuery{Kind: "fabric", Limit: 10})
	if len(got) != 1 || got[0].VariantID != "v-real" {
		t.Fatalf("faqat v-real kutilgan, olindi %+v", got)
	}
	if _, ok := v.Variant("v-ref"); !ok {
		t.Fatal("etalon variant hisob uchun View.Variant orqali topilishi kerak")
	}
}
