package tools

// DTO'lar: LLM ko'radigan kirish/chiqish. jsonschema teglari MAJBURIY (Eino sxema yasaydi).
// Sonlar float64 — bu LLM chegarasi; ichkarida darhol decimal'ga o'tadi (rules §1.5).
// Har natijada Error maydoni — errors-as-results (rules §1.3).

// ---- calculator (shablondan) ----

type calcArgs struct {
	Op string  `json:"op" jsonschema:"description=amal,enum=add,enum=sub,enum=mul,enum=div"`
	A  float64 `json:"a" jsonschema:"description=birinchi son"`
	B  float64 `json:"b" jsonschema:"description=ikkinchi son"`
}

// ---- current_time ----

type timeArgs struct {
	Timezone string `json:"timezone" jsonschema:"description=IANA vaqt mintaqasi — masalan Asia/Tashkent"`
}

// ---- product_search ----

type productSearchArgs struct {
	Query        string   `json:"query,omitempty"         jsonschema:"description=nom/SKU bo'yicha (mijoz nom aytsa)"`
	Room         string   `json:"room,omitempty"          jsonschema:"description=xona turi,enum=living,enum=bedroom,enum=wedding,enum=kids,enum=kitchen,enum=office,enum=hall"`
	Style        string   `json:"style,omitempty"         jsonschema:"description=uslub,enum=classic,enum=modern,enum=hi_tech,enum=minimal,enum=oriental,enum=provence"`
	Pattern      string   `json:"pattern,omitempty"       jsonschema:"description=bezak,enum=plain,enum=floral,enum=geometric,enum=striped,enum=jacquard"`
	Transparency string   `json:"transparency,omitempty"  jsonschema:"description=shaffoflik,enum=transparent,enum=semi,enum=blackout"`
	Layer        string   `json:"layer,omitempty"         jsonschema:"description=qatlam,enum=tul,enum=portyera,enum=lambrekan,enum=astar,enum=kant,enum=rim,enum=rulon"`
	Layers       []string `json:"layers,omitempty"        jsonschema:"description=ko'p qatlam: bir chaqiruvda har qatlam uchun 3 variant"`
	Color        string   `json:"color,omitempty"         jsonschema:"description=istalgan rang — masalan bej yoki grafit"`
	Kind         string   `json:"kind,omitempty"          jsonschema:"description=nima qidirilyapti: mato yoki bezak/furnitura,enum=fabric,enum=accessory"`
	Segment      string   `json:"segment,omitempty"       jsonschema:"description=narx segmenti,enum=economy,enum=standard,enum=premium"`
	MaxPrice     float64  `json:"max_price,omitempty"     jsonschema:"description=birlik uchun maksimal narx so'mda (m2 — dona yoki metr)"`
	Limit        int      `json:"limit,omitempty"         jsonschema:"description=nechta natija — standart 5 — eng ko'pi 10"`
}

type productCard struct {
	VariantID    string `json:"variant_id"`
	Name         string `json:"name"`
	ColorName    string `json:"color,omitempty"`
	Price        string `json:"price"`
	PriceDisplay string `json:"price_display,omitempty"`
	PriceUnit    string `json:"unit"`
	WidthM       string `json:"width_m,omitempty"`
	Transparency string `json:"transparency,omitempty"`
	Segment      string `json:"segment,omitempty"`
	Why          string `json:"why,omitempty"`
}

type productSearchResult struct {
	Products []productCard `json:"products,omitempty"`
	// ByLayer — layers so'ralganda: qatlam → eng mos variantlar (curtain_calculate parts uchun tayyor).
	ByLayer  map[string][]productCard `json:"by_layer,omitempty"`
	Count    int                      `json:"count"`
	Stale    bool                     `json:"stale,omitempty"`
	Warnings []string                 `json:"warnings,omitempty"`
	Error    string                   `json:"error,omitempty"`
}

// ---- curtain_calculate ----

type calcPart struct {
	VariantID   string  `json:"variant_id"            jsonschema:"description=variant ID"`
	Role        string  `json:"role"                  jsonschema:"description=bo'lak roli,enum=portyera,enum=tul,enum=lambrekan,enum=kant,enum=dekor,enum=astar"`
	WidthM      float64 `json:"width_m,omitempty"     jsonschema:"description=kenglik metr (bo'sh = butun karniz)"`
	HeightM     float64 `json:"height_m,omitempty"    jsonschema:"description=balandlik metr (bo'sh = umumiy)"`
	Coefficient float64 `json:"coefficient,omitempty" jsonschema:"description=burma koeffitsienti (bo'sh = standart)"`
}

type calcAccessory struct {
	VariantID string  `json:"variant_id" jsonschema:"description=aksessuar variant ID"`
	Qty       float64 `json:"qty"        jsonschema:"description=miqdori: dona yoki metr"`
}

type curtainCalcArgs struct {
	CorniceLengthM float64         `json:"cornice_length_m"      jsonschema:"description=karniz uzunligi metrda"`
	HeightM        float64         `json:"height_m"              jsonschema:"description=pardaning umumiy balandligi metrda"`
	Parts          []calcPart      `json:"parts"                 jsonschema:"description=pardani tashkil qiluvchi mato bo'laklari — 1 tadan 10 tagacha"`
	Accessories    []calcAccessory `json:"accessories,omitempty" jsonschema:"description=matodan tashqari bezaklar: dekorativ gul — podxvat — kisti — bahroma — halqa"`
	RoomName       string          `json:"room_name,omitempty"   jsonschema:"description=xona nomi — masalan mehmonxona"`
}

// ---- ixcham hisob natijasi (curtain_calculate / price_quote) ----
// To'liq CalcResult LLM'ga katta (10 bo'lakda 4 KB+) — shuning uchun faqat
// yakuniy yig'indilar va eng kerakli qatorlar chiqariladi (rules §5: ≤ 2 KB).
// To'liq hisob SessionState'da (LastCalc/LastQuote) saqlanadi va yo'qotilmaydi.

type calcTotals struct {
	PartsTotal       string `json:"parts_total,omitempty"`       // kontrakt 1.4.1; eski backend -> bo'sh
	AccessoriesTotal string `json:"accessories_total,omitempty"` // yuqoridagi kabi
	ServicesTotal    string `json:"services_total,omitempty"`    // yuqoridagi kabi
	DeliveryAmount   string `json:"delivery_amount,omitempty"`
	DiscountPercent  string `json:"discount_percent,omitempty"`
	DiscountAmount   string `json:"discount_amount,omitempty"`
	Subtotal         string `json:"subtotal"`
	Total            string `json:"total"`
	Prepayment       string `json:"prepayment,omitempty"` // faqat price_quote
	Balance          string `json:"balance,omitempty"`    // faqat price_quote
}

type calcLine struct {
	Role      string `json:"role"`
	Name      string `json:"name"` // mahsulot nomi; katalogda bo'lmasa SKU
	AreaM2    string `json:"area_m2"`
	Coef      string `json:"coef"`
	LineTotal string `json:"line_total"`
}

type calcServiceLine struct {
	Key       string `json:"key"`
	Name      string `json:"name,omitempty"`
	LineTotal string `json:"line_total"`
}

type curtainCalcResult struct {
	Totals      *calcTotals       `json:"totals,omitempty"`
	Lines       []calcLine        `json:"lines,omitempty"`
	Services    []calcServiceLine `json:"services,omitempty"`
	Display     any               `json:"display,omitempty"` // o'qiladigan summalar: {"subtotal":"5 382 000 so'm",...}
	Warnings    []string          `json:"warnings,omitempty"`
	Explanation []string          `json:"explanation,omitempty"` // curtains.Explain — izoh/tejash tavsiyalari (1.9)
	Stale       bool              `json:"stale,omitempty"`
	Error       string            `json:"error,omitempty"`
}

// ---- build_options ----

type buildOptionsArgs struct {
	CorniceLengthM float64 `json:"cornice_length_m,omitempty" jsonschema:"description=karniz uzunligi metr"`
	HeightM        float64 `json:"height_m,omitempty"         jsonschema:"description=balandlik metr"`
	BudgetMax      float64 `json:"budget_max,omitempty" jsonschema:"description=maks budjet so'm"`
	Room           string  `json:"room,omitempty" jsonschema:"description=xona turi"`
	Style          string  `json:"style,omitempty" jsonschema:"description=uslub"`
	Pattern        string  `json:"pattern,omitempty" jsonschema:"description=bezak turi"`
	Transparency   string  `json:"transparency,omitempty" jsonschema:"description=shaffoflik"`
	Color          string  `json:"color,omitempty" jsonschema:"description=istalgan rang"`
}

type buildOptionResult struct {
	Tier     string               `json:"tier"`
	Why      string               `json:"why,omitempty"`
	Total    string               `json:"total,omitempty"` // o'qiladigan yakuniy summa
	Display  any                  `json:"display,omitempty"`
	Lines    []string             `json:"lines,omitempty"` // tarkib xulosasi, eng ko'pi 3 qator
	Variants []buildOptionVariant `json:"variants,omitempty"`
	Error    string               `json:"error,omitempty"`
}

// buildOptionVariant — paketni keyin qayta hisoblashda model ishlatishi kerak
// bo'lgan katalog identifikatori. Nom yoki SKU variant_id o'rnini bosa olmaydi.
type buildOptionVariant struct {
	Role      string `json:"role"`
	VariantID string `json:"variant_id"`
	Name      string `json:"name,omitempty"`
}

type buildOptionsResult struct {
	Options    []buildOptionResult `json:"options,omitempty"`
	Comparison []string            `json:"comparison,omitempty"`
	Warnings   []string            `json:"warnings,omitempty"`
	Stale      bool                `json:"stale,omitempty"`
	Error      string              `json:"error,omitempty"`
}

// ---- price_quote ----

type priceQuoteArgs struct {
	NeedCornice   bool     `json:"need_cornice,omitempty"   jsonschema:"description=yangi karniz kerakmi"`
	NeedInstall   bool     `json:"need_install,omitempty"   jsonschema:"description=montaj kerakmi"`
	NeedDelivery  bool     `json:"need_delivery,omitempty"  jsonschema:"description=yetkazib berish kerakmi"`
	Services      []string `json:"services,omitempty"       jsonschema:"description=xizmatlar: sewing tape hook (bo'sh = standart)"`
	DiscountPct   float64  `json:"discount_pct,omitempty"   jsonschema:"description=sotuvchi/manager qo'lda kiritgan chegirma foizi"`
	PrepaymentPct float64  `json:"prepayment_pct,omitempty" jsonschema:"description=avans %"`
}

type priceQuoteResult struct {
	Totals      *calcTotals       `json:"totals,omitempty"`
	Lines       []calcLine        `json:"lines,omitempty"`
	Services    []calcServiceLine `json:"services,omitempty"`
	Display     any               `json:"display,omitempty"` // o'qiladigan summalar, shu jumladan prepayment/balance
	Warnings    []string          `json:"warnings,omitempty"`
	Explanation []string          `json:"explanation,omitempty"` // curtains.Explain — izoh/tejash tavsiyalari (1.9)
	Stale       bool              `json:"stale,omitempty"`
	Error       string            `json:"error,omitempty"`
}

// ---- set_context ----

type setContextArgs struct {
	Room           string  `json:"room,omitempty"         jsonschema:"description=xona turi,enum=living,enum=bedroom,enum=wedding,enum=kids,enum=kitchen,enum=office,enum=hall"`
	Style          string  `json:"style,omitempty"        jsonschema:"description=uslub,enum=classic,enum=modern,enum=hi_tech,enum=minimal,enum=oriental,enum=provence"`
	Pattern        string  `json:"pattern,omitempty"      jsonschema:"description=bezak,enum=plain,enum=floral,enum=geometric,enum=striped,enum=jacquard"`
	Transparency   string  `json:"transparency,omitempty" jsonschema:"description=shaffoflik,enum=transparent,enum=semi,enum=blackout"`
	Light          string  `json:"light,omitempty"        jsonschema:"description=yorug'lik/deraza tomoni"`
	Colors         string  `json:"colors,omitempty"       jsonschema:"description=xona ranglari"`
	Construction   string  `json:"construction,omitempty" jsonschema:"description=deraza konstruksiyasi: rom/balkon eshigi/karniz"`
	CorniceLengthM float64 `json:"cornice_length_m,omitempty" jsonschema:"description=karniz uzunligi metr"`
	HeightM        float64 `json:"height_m,omitempty"         jsonschema:"description=balandlik metr"`
	BudgetMin      float64 `json:"budget_min,omitempty"   jsonschema:"description=budjet pastki chegarasi so'mda"`
	BudgetMax      float64 `json:"budget_max,omitempty"   jsonschema:"description=budjet yuqori chegarasi so'mda"`
}

type setContextResult struct {
	State        any    `json:"state,omitempty"`
	NextQuestion string `json:"next_question,omitempty"`
	Error        string `json:"error,omitempty"`
}

// ---- fason_lookup ----

type fasonLookupArgs struct {
	Room  string `json:"room,omitempty"  jsonschema:"description=xona turi,enum=living,enum=bedroom,enum=wedding,enum=kids,enum=kitchen,enum=office,enum=hall"`
	Style string `json:"style,omitempty" jsonschema:"description=uslub,enum=classic,enum=modern,enum=hi_tech,enum=minimal,enum=oriental,enum=provence"`
}

type fasonPartCard struct {
	Role        string `json:"role"`
	Name        string `json:"name"`
	WidthShare  string `json:"width_share"`
	HeightShare string `json:"height_share"`
	DefaultCoef string `json:"default_coef"`
	// Tayyor tanlov: shu rol uchun katalogdan eng mos variant (xona/uslub bo'yicha) —
	// model curtain_calculate.parts ga aynan shuni uzatadi, qatlam-qatlam qidirmaydi.
	VariantID    string `json:"variant_id,omitempty"`
	VariantName  string `json:"variant_name,omitempty"`
	PriceDisplay string `json:"price_display,omitempty"`
}

type fasonAccessoryCard struct {
	Category   string `json:"category"`
	DefaultQty string `json:"default_qty"`
	QtyFormula string `json:"qty_formula"`
}

type fasonCard struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Style       string               `json:"style"`
	Rooms       []string             `json:"rooms,omitempty"`
	PartCount   int                  `json:"part_count"`
	Parts       []fasonPartCard      `json:"parts,omitempty"`
	Accessories []fasonAccessoryCard `json:"accessories,omitempty"`
}

type fasonLookupResult struct {
	Fasons   []fasonCard `json:"fasons,omitempty"`
	Count    int         `json:"count"`
	Warnings []string    `json:"warnings,omitempty"`
	Stale    bool        `json:"stale,omitempty"`
	Error    string      `json:"error,omitempty"`
}

// ---- knowledge_lookup ----

type knowledgeLookupArgs struct {
	Topic string `json:"topic,omitempty" jsonschema:"description=qoida turi,enum=room,enum=style,enum=objection,enum=light,enum=construction,enum=color"`
	Query string `json:"query,omitempty" jsonschema:"description=tavsiya matnida qidiriladigan kalit so'z"`
}

type knowledgeRuleCard struct {
	Kind           string `json:"kind"`
	Recommendation string `json:"recommendation"`
	Priority       int    `json:"priority"`
}

type knowledgeLookupResult struct {
	Note     string              `json:"note"`
	Rules    []knowledgeRuleCard `json:"rules,omitempty"`
	Count    int                 `json:"count"`
	Warnings []string            `json:"warnings,omitempty"`
	Stale    bool                `json:"stale,omitempty"`
	Error    string              `json:"error,omitempty"`
}

// ---- order_draft ----

type orderDraftArgs struct {
	Action        string  `json:"action"                  jsonschema:"description=amal — buyurtma qoralamasini saqlash,enum=quote,enum=draft,enum=confirm"`
	CustomerName  string  `json:"customer_name,omitempty" jsonschema:"description=mijoz ismi — SOTUVCHI kiritadi — mijozdan so'ralmaydi"`
	CustomerPhone string  `json:"customer_phone,omitempty" jsonschema:"description=mijoz telefoni — masalan +998901234567 — SOTUVCHI kiritadi"`
	Address       string  `json:"address,omitempty"       jsonschema:"description=yetkazib berish manzili — SOTUVCHI kiritadi"`
	Note          string  `json:"note,omitempty"          jsonschema:"description=buyurtmaga qo'shimcha eslatma"`
	DeadlineDays  int     `json:"deadline_days,omitempty" jsonschema:"description=bajarish muddati kunlarda; bo'sh bo'lsa do'kon standarti"`
	PrepaymentPct float64 `json:"prepayment_pct,omitempty" jsonschema:"description=avans %"`
}

type orderDraftResult struct {
	Quote        any      `json:"quote,omitempty"`         // Django taklifi (LastQuote) — action=quote
	Display      any      `json:"display,omitempty"`       // o'qiladigan summalar: {"subtotal":"...","total":"..."}
	Missing      []string `json:"missing,omitempty"`       // yetishmayotgan mijoz maydonlari — action=draft
	Hint         string   `json:"hint,omitempty"`          // AI'ga nega missing borligini tushuntiradi
	Ready        bool     `json:"ready,omitempty"`         // hamma maydon to'ldi, confirm mumkin
	Summary      any      `json:"summary,omitempty"`       // action=draft tayyor bo'lganda qisqa referens
	OrderID      string   `json:"order_id,omitempty"`      // Django buyurtma UUID si — action=confirm
	OrderNumber  string   `json:"order_number,omitempty"`  // ALR-2026-000123 — action=confirm
	CustomerRef  string   `json:"customer_ref,omitempty"`  // "mijoz #1234" — ism/telefon EMAS (rules §1.6)
	TotalDisplay string   `json:"total_display,omitempty"` // yakuniy summa — action=confirm
	Warnings     []string `json:"warnings,omitempty"`
	Error        string   `json:"error,omitempty"`
}
