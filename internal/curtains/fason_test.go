package curtains

import (
	"testing"
)

func TestLookupFason_MatchByRoomAndStyle(t *testing.T) {
	v := fasonView(
		Fason{ID: "f-gen", Name: "Zebra", Rooms: nil, Style: ""},
		Fason{ID: "f-mod", Name: "Alfa", Rooms: []string{"living"}, Style: "modern"},
		Fason{ID: "f-any", Name: "Beta", Rooms: []string{"living"}, Style: ""},
		Fason{ID: "f-cla", Name: "Classica", Rooms: []string{"living"}, Style: "classic"},
		Fason{ID: "f-kt", Name: "Katya", Rooms: []string{"kitchen"}, Style: "classic"},
	)
	got := LookupFason(v, "living", "classic")
	ids := ids(got)
	// f-cla: xona+uslub aniq mos (2); f-any: faqat xona (1); f-gen: umumiy (0).
	want := []string{"f-cla", "f-any", "f-gen"}
	if len(ids) != len(want) {
		t.Fatalf("natija soni %d, kutilgan %d: %v", len(ids), len(want), ids)
	}
	for i := range ids {
		if ids[i] != want[i] {
			t.Fatalf("[%d] %q, kutilgan %q; natija: %v", i, ids[i], want[i], ids)
		}
	}
}

func TestLookupFason_DeterministicNameOrdering(t *testing.T) {
	v := fasonView(
		Fason{ID: "g2", Name: "Zebra", Rooms: nil, Style: ""},
		Fason{ID: "g1", Name: "Alpha", Rooms: nil, Style: ""},
		Fason{ID: "r1", Name: "LivingZ", Rooms: []string{"living"}, Style: ""},
	)
	got := LookupFason(v, "living", "classic")
	// Umumiylar (r1 aniqroq emas, chunki faqat room). Bizning exactness: r1 room=1, g1/g2 room=0.
	// r1 birinchi; keyin nom bo'yicha: Alpha, Zebra.
	ids := ids(got)
	want := []string{"r1", "g1", "g2"}
	if len(ids) != len(want) {
		t.Fatalf("son: %d vs %d: %v vs %v", len(ids), len(want), ids, want)
	}
	for i := range ids {
		if ids[i] != want[i] {
			t.Fatalf("[%d] %q != %q: %v", i, ids[i], want[i], ids)
		}
	}
}

func TestLookupFason_NoMatchReturnsEmpty(t *testing.T) {
	v := fasonView(Fason{ID: "f1", Name: "Katya", Rooms: []string{"kitchen"}, Style: "classic"})
	if got := LookupFason(v, "bedroom", "modern"); len(got) != 0 {
		t.Fatalf("mos fason yo'q; kutib turgan bo'sh: %+v", got)
	}
}

func TestLookupFason_EmptyFiltersReturnsAll(t *testing.T) {
	v := fasonView(
		Fason{ID: "a", Name: "A"},
		Fason{ID: "b", Name: "B"},
	)
	if got := LookupFason(v, "", ""); len(got) != 2 {
		t.Fatalf("filter yo'qda hammasi: %d", len(got))
	}
}

func TestFasonToParts_SharesAndCoefficients(t *testing.T) {
	f := Fason{Parts: []FasonPart{
		{Role: RolePortyera, Name: "Portyera", WidthShare: dec("0.68"), HeightShare: dec("1"), DefaultCoef: dec("2.5"), SortOrder: 0},
		{Role: RoleTul, Name: "Tul", WidthShare: dec("0.32"), HeightShare: dec("1"), DefaultCoef: dec("2"), SortOrder: 1},
	}}
	parts := FasonToParts(f, dec("4.2"), dec("2.6"), map[PartRole]string{RolePortyera: "v1"})
	if len(parts) != 2 {
		t.Fatalf("bo'lak soni: %d", len(parts))
	}
	if !parts[0].WidthM.Equal(dec("2.856")) || !parts[0].HeightM.Equal(dec("2.6")) {
		t.Fatalf("portyera o'lchov: w=%s h=%s", parts[0].WidthM, parts[0].HeightM)
	}
	if !parts[1].WidthM.Equal(dec("1.344")) {
		t.Fatalf("tul kenglik: %s", parts[1].WidthM)
	}
	if parts[0].VariantID != "v1" || parts[1].VariantID != "" {
		t.Fatalf("variantlar: %s %s", parts[0].VariantID, parts[1].VariantID)
	}
	if !parts[0].Coef.Equal(dec("2.5")) || !parts[1].Coef.Equal(dec("2")) {
		t.Fatalf("koeffitsientlar: %s %s", parts[0].Coef, parts[1].Coef)
	}
	// Ulushlar yig'indisi butun karnizga teng (3 kasr).
	sum := parts[0].WidthM.Add(parts[1].WidthM)
	if !sum.Equal(dec("4.2")) {
		t.Fatalf("kenglik yig'indisi %s, kutilgan 4.2", sum)
	}
}

func TestFasonToParts_RoundingThreeDecimals(t *testing.T) {
	f := Fason{Parts: []FasonPart{
		{Role: RolePortyera, Name: "P", WidthShare: dec("0.3334"), HeightShare: dec("0.3334"), DefaultCoef: dec("2")},
	}}
	parts := FasonToParts(f, dec("1"), dec("1"), nil)
	// 1×0.3334=0.3334 → round(3)=0.333
	if !parts[0].WidthM.Equal(dec("0.333")) || !parts[0].HeightM.Equal(dec("0.333")) {
		t.Fatalf("round: w=%s h=%s", parts[0].WidthM, parts[0].HeightM)
	}
}

func TestFasonToParts_EmptyLayers(t *testing.T) {
	f := Fason{Parts: []FasonPart{
		{Role: RolePortyera, Name: "P", WidthShare: dec("0.6"), HeightShare: dec("1"), DefaultCoef: dec("2.5")},
	}}
	parts := FasonToParts(f, dec("4"), dec("2.6"), nil)
	if parts[0].VariantID != "" {
		t.Fatalf("variant bo'sh bo'lishi kerak: %q", parts[0].VariantID)
	}
}

func TestFasonAccessoryQty_Formulas(t *testing.T) {
	perSide := FasonAccessory{Category: "podxvat", DefaultQty: dec("1"), QtyFormula: "per_side"}
	perMeter := FasonAccessory{Category: "bahroma", DefaultQty: dec("1.5"), QtyFormula: "per_meter"}
	fixed := FasonAccessory{Category: "gul", DefaultQty: dec("3"), QtyFormula: ""}
	unknown := FasonAccessory{Category: "x", DefaultQty: dec("0"), QtyFormula: "boshqa"}

	if !FasonAccessoryQty(perSide, dec("4.2")).Equal(dec("2")) {
		t.Fatal("per_side → 2")
	}
	if !FasonAccessoryQty(perMeter, dec("4.2")).Equal(dec("6.3")) {
		t.Fatalf("per_meter: %s", FasonAccessoryQty(perMeter, dec("4.2")))
	}
	if !FasonAccessoryQty(fixed, dec("4.2")).Equal(dec("3")) {
		t.Fatal("default formula → DefaultQty")
	}
	// Noma'lum formula → DefaultQty (qiymati 0).
	if !FasonAccessoryQty(unknown, dec("4.2")).Equal(dec("0")) {
		t.Fatal("noma'lum formula → DefaultQty")
	}
}

func TestFasonAccessoryQty_PerMeterRounding(t *testing.T) {
	a := FasonAccessory{Category: "x", DefaultQty: dec("1.333"), QtyFormula: "per_meter"}
	got := FasonAccessoryQty(a, dec("1"))
	// 1×1.333=1.333 → round(3)=1.333
	if !got.Equal(dec("1.333")) {
		t.Fatalf("per_meter round: %s", got)
	}
}

func fasonView(fs ...Fason) *View {
	return searchView(Snapshot{Version: 1, Fasons: fs})
}

func ids(fs []Fason) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.ID
	}
	return out
}
