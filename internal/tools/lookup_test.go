package tools

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

// lookupCatalog — fason_lookup va knowledge_lookup uchun o'z katalogi
// (alora_test.go dagi testCatalog faqat mahsulotlar bilan — bu yerda Fason+Knowledge).
func lookupCatalog() *curtains.Catalog {
	c := curtains.NewCatalog(curtains.RealClock{}, time.Hour)
	c.ApplySnapshot(curtains.Snapshot{
		Version: 2,
		Fasons: []curtains.Fason{
			{ID: "f1", Name: "Alfa", Style: "classic", Rooms: []string{"living"}, PartCount: 2, Parts: []curtains.FasonPart{
				{Role: curtains.RolePortyera, Name: "Portyera", WidthShare: d("0.68"), HeightShare: d("1"), DefaultCoef: d("2.5")},
				{Role: curtains.RoleTul, Name: "Tul", WidthShare: d("0.32"), HeightShare: d("1"), DefaultCoef: d("2")},
			}, Accessories: []curtains.FasonAccessory{{Category: "podxvat", DefaultQty: d("1"), QtyFormula: "per_side"}}},
			{ID: "f2", Name: "Beta", Style: "modern", Rooms: []string{"living", "hall"}, PartCount: 1, Parts: []curtains.FasonPart{
				{Role: curtains.RolePortyera, Name: "Portyera", WidthShare: d("1"), HeightShare: d("1"), DefaultCoef: d("2")},
			}},
			{ID: "f3", Name: "Gamma", Style: "", Rooms: []string{"kitchen"}, PartCount: 1, Parts: []curtains.FasonPart{
				{Role: curtains.RoleTul, Name: "Tul", WidthShare: d("1"), HeightShare: d("1"), DefaultCoef: d("2")},
			}},
		},
		Knowledge: append(
			[]curtains.KnowledgeRule{
				{ID: "k1", Kind: "room", Priority: 2, Recommendation: "Kam yorug' xonada yengil tul tavsiya etiladi.", Condition: map[string]any{"room": "living"}},
				{ID: "k2", Kind: "style", Priority: 5, Recommendation: "<script>Modern uslub uchun minimal chiziqlar.</script>", Condition: map[string]any{"style": "modern"}},
				{ID: "k3", Kind: "objection", Priority: 1, Recommendation: "Narx e'tiroziga qadamli javob beriladi.", Condition: nil},
				{ID: "k4", Kind: "light", Priority: 3, Recommendation: "Shimol tomonda qorong'i mato tanlanadi.", Condition: map[string]any{"light": "qorong'iroq"}},
				{ID: "k5", Kind: "room", Priority: 9, Recommendation: "Yotoqxona uchun jumboq mato.", Condition: map[string]any{"room": "bedroom"}},
			},
			generalRules()...,
		),
	})
	return c
}

// generalRules — inline_max dan oshish holatini tekshirish uchun ko'p umumiy qoida:
// ular ham state'ga mos keladi (condition bo'sh), limit 10 ularni kesishi kerak.
func generalRules() []curtains.KnowledgeRule {
	out := make([]curtains.KnowledgeRule, 0, 12)
	for i := 10; i <= 21; i++ {
		out = append(out, curtains.KnowledgeRule{
			ID: "k" + strconv.Itoa(i), Kind: "room", Priority: 1,
			Recommendation: "Umumiy maslahat, martaba tartibi.",
			Condition:      nil,
		})
	}
	return out
}

func lookupDeps() Deps {
	return Deps{Catalog: lookupCatalog(), Calc: nil, State: &memState{}, Clock: curtains.RealClock{}}
}

func TestAloraLookups_Names(t *testing.T) {
	dp := lookupDeps()
	got := AloraLookups(dp)
	if len(got) != 2 {
		t.Fatalf("tool soni: %d", len(got))
	}
	want := map[string]string{"fason_lookup": "fason", "knowledge_lookup": "bilim"}
	for _, tl := range got {
		info, err := tl.Info(context.Background())
		if err != nil {
			t.Fatalf("Info: %v", err)
		}
		if want[info.Name] == "" {
			t.Fatalf("kutilmagan tool: %s", info.Name)
		}
		delete(want, info.Name)
	}
	if len(want) != 0 {
		t.Fatalf("yetishmayotgan: %v", want)
	}
}

func TestFasonLookup_OrderAndContents(t *testing.T) {
	dp := lookupDeps()
	h := &fasonLookupHandler{d: dp}
	res, _ := h.do(context.Background(), fasonLookupArgs{Room: "living"})
	if res.Error != "" {
		t.Fatalf("xato: %s", res.Error)
	}
	// Xona mos ikkitasi (f1, f2 — ikkalasi ham room aniq); nom: Alfa, Beta.
	if res.Count != 2 || res.Fasons[0].ID != "f1" || res.Fasons[1].ID != "f2" {
		t.Fatalf("tartib: %+v", res.Fasons)
	}
	f1 := res.Fasons[0]
	if f1.PartCount != 2 || len(f1.Parts) != 2 || f1.Parts[0].Role != "portyera" {
		t.Fatalf("f1 parts: %+v", f1)
	}
	if f1.Parts[0].WidthShare != "0.68" || f1.Parts[0].DefaultCoef != "2.5" {
		t.Fatalf("share/coef: %+v", f1.Parts[0])
	}
	if len(f1.Accessories) != 1 || f1.Accessories[0].QtyFormula != "per_side" || f1.Accessories[0].DefaultQty != "1" {
		t.Fatalf("accessories: %+v", f1.Accessories)
	}
}

func TestFasonLookup_StyleFilterAndEmpty(t *testing.T) {
	dp := lookupDeps()
	h := &fasonLookupHandler{d: dp}

	// style classic → faqat f1.
	res, _ := h.do(context.Background(), fasonLookupArgs{Room: "living", Style: "classic"})
	if res.Count != 1 || res.Fasons[0].ID != "f1" {
		t.Fatalf("classic: %+v", res.Fasons)
	}
	// Mos fason yo'q → bo'sh ro'yxat, xato emas.
	res, _ = h.do(context.Background(), fasonLookupArgs{Room: "wedding"})
	if res.Error != "" || res.Count != 0 || len(res.Fasons) != 0 {
		t.Fatalf("bo'sh natija: %+v", res)
	}
}

func TestFasonLookup_CatalogNotReady(t *testing.T) {
	dp := Deps{Catalog: curtains.NewCatalog(curtains.RealClock{}, time.Hour)}
	res, _ := (&fasonLookupHandler{d: dp}).do(context.Background(), fasonLookupArgs{})
	if res.Error != errCatalogNotReady {
		t.Fatalf("katalog bo'sh: %q", res.Error)
	}
}

func TestFasonLookup_StaleWarns(t *testing.T) {
	clk := curtains.NewFakeClock(time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC))
	c := curtains.NewCatalog(clk, time.Hour)
	c.ApplySnapshot(curtains.Snapshot{Version: 1, Fasons: []curtains.Fason{
		{ID: "f1", Name: "Alfa", Rooms: []string{"living"}},
	}})
	clk.Advance(2 * time.Hour)
	res, _ := (&fasonLookupHandler{d: Deps{Catalog: c}}).do(context.Background(), fasonLookupArgs{Room: "living"})
	if !res.Stale || len(res.Warnings) == 0 || !strings.Contains(res.Warnings[0], "eski") {
		t.Fatalf("stale bayroq: %+v", res)
	}
}

func TestKnowledgeLookup_StateFilter(t *testing.T) {
	dp := lookupDeps()
	ctx, _ := ctxFor(curtains.RoleEmployee)
	// state'ga living + modern yozamiz — k1, k2, umumiy qoidalar mos.
	if err := dp.State.SetState(ctx, "s1", curtains.SessionState{Room: "living", Style: "modern"}); err != nil {
		t.Fatal(err)
	}
	res, _ := (&knowledgeLookupHandler{d: dp}).do(ctx, knowledgeLookupArgs{})
	if res.Error != "" {
		t.Fatalf("xato: %s", res.Error)
	}
	// k2(p5), k1(p2), keyin umumiy (12 + k3) — limit 6 tashqaridagilarni kesadi.
	if res.Count != 6 || res.Rules[0].Kind != "style" || res.Rules[1].Kind != "room" {
		t.Fatalf("rules boshida: count=%d first=%s second=%s", res.Count, res.Rules[0].Kind, res.Rules[1].Kind)
	}
	if res.Note != knowledgeNote {
		t.Fatalf("note: %q", res.Note)
	}
}

func TestKnowledgeLookup_TopicAndQueryFilter(t *testing.T) {
	dp := lookupDeps()
	ctx, _ := ctxFor(curtains.RoleEmployee)
	_ = dp.State.SetState(ctx, "s1", curtains.SessionState{Room: "living", Style: "modern"})
	h := &knowledgeLookupHandler{d: dp}

	// topic=objection → faqat umumiy k3 (kind bo'yicha).
	res, _ := h.do(ctx, knowledgeLookupArgs{Topic: "objection"})
	if res.Count != 1 || res.Rules[0].Kind != "objection" {
		t.Fatalf("topic: %+v", res.Rules)
	}

	// query registrar sezgir emas: "TUL" k1 ni topadi (k1 priority 2 — yagona mos).
	res, _ = h.do(ctx, knowledgeLookupArgs{Query: "TUL"})
	if res.Count != 1 || res.Rules[0].Kind != "room" {
		t.Fatalf("query: %+v", res.Rules)
	}
}

func TestKnowledgeLookup_Sanitize(t *testing.T) {
	dp := lookupDeps()
	ctx, _ := ctxFor(curtains.RoleEmployee)
	_ = dp.State.SetState(ctx, "s1", curtains.SessionState{Style: "modern"})
	res, _ := (&knowledgeLookupHandler{d: dp}).do(ctx, knowledgeLookupArgs{Topic: "style"})
	// B7 (ADR-020): topic lug'ati bitta — `style` va `room` ikkalasi `room_style`
	// ga xarilanadi, shuning uchun topic=style umumiy (kind=room) qoidalarni ham
	// qaytaradi. Sanitize tekshiruvi shu qoidalardan birida bajariladi.
	if res.Count == 0 {
		t.Fatalf("style rules: %+v", res.Rules)
	}
	rec := ""
	for _, r := range res.Rules {
		if r.Kind == "style" {
			rec = r.Recommendation
			break
		}
	}
	if rec == "" {
		t.Fatalf("style qoidasi topilmadi: %+v", res.Rules)
	}
	if strings.ContainsAny(rec, "<>") {
		t.Fatalf("HTML tozalanmagan: %q", rec)
	}
	// `<script>`/`</script>` belgilari o'chadi, "script" qoldiqlari esa matnda qoladi:
	// maqsad — HTML belgilarini olib tashlash (rules §16.2), so'zlar emas.
	want := "scriptModern uslub uchun minimal chiziqlar./script"
	if rec != want {
		t.Fatalf("sanitize natija: %q != %q", rec, want)
	}
}

func TestKnowledgeLookup_StateExcludesNonMatch(t *testing.T) {
	dp := lookupDeps()
	ctx, _ := ctxFor(curtains.RoleEmployee)
	// state'ga room=bedroom → k5 priority 9 birinchi; living qoidalari o'tmaydi.
	_ = dp.State.SetState(ctx, "s1", curtains.SessionState{Room: "bedroom"})
	res, _ := (&knowledgeLookupHandler{d: dp}).do(ctx, knowledgeLookupArgs{})
	if res.Count == 0 || !strings.Contains(res.Rules[0].Recommendation, "jumboq") {
		t.Fatalf("bedroom: %+v", res.Rules)
	}
}

func TestKnowledgeLookup_CatalogNotReady(t *testing.T) {
	dp := Deps{Catalog: curtains.NewCatalog(curtains.RealClock{}, time.Hour)}
	res, _ := (&knowledgeLookupHandler{d: dp}).do(context.Background(), knowledgeLookupArgs{})
	if res.Error != errCatalogNotReady {
		t.Fatalf("katalog bo'sh: %q", res.Error)
	}
}
