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
	FabricType   string `json:"fabric_type,omitempty"`
	Composition  string `json:"composition,omitempty"`
	Care         string `json:"care,omitempty"`
	Description  string `json:"description,omitempty"`
	Segment      string `json:"segment,omitempty"`
	// CalcSupported — faqat false bo'lganda chiqadi (rulon/rim: AI hisoblamaydi,
	// sotuvchi narxni qo'lda belgilaydi — ADR-014). Boshqa mahsulotlarda maydon yo'q.
	CalcSupported *bool  `json:"calc_supported,omitempty"`
	Why           string `json:"why,omitempty"`
}

type productSearchResult struct {
	Products []productCard `json:"products,omitempty"`
	// ByLayer — layers so'ralganda: qatlam → eng mos variantlar (curtain_calculate parts uchun tayyor).
	ByLayer  map[string][]productCard `json:"by_layer,omitempty"`
	Count    int                      `json:"count"`
	Stale    bool                     `json:"stale,omitempty"`
	Stop     bool                     `json:"stop,omitempty"` // bo'sh qidiruv sikli tugdi — qayta qidirma (ADR-020)
	Hint     string                   `json:"hint,omitempty"` // stop bilan birga: nima qilish kerak
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
	CorniceLengthM float64         `json:"cornice_length_m"      jsonschema:"description=karniz uzunligi metrda — windows bo'lmasa"`
	HeightM        float64         `json:"height_m"              jsonschema:"description=pardaning umumiy balandligi metrda — windows bo'lmasa"`
	Parts          []calcPart      `json:"parts,omitempty"       jsonschema:"description=pardani tashkil qiluvchi mato bo'laklari — 1 tadan 10 tagacha"`
	Accessories    []calcAccessory `json:"accessories,omitempty" jsonschema:"description=matodan tashqari bezaklar: dekorativ gul — podxvat — kisti — bahroma — halqa"`
	RoomName       string          `json:"room_name,omitempty"   jsonschema:"description=xona nomi — masalan mehmonxona"`
	// Windows — bir nechta o'ynaning o'lchovi. qty bo'yicha Django'ga alohida
	// o'lchov yozuvlari ketadi (Go summani ko'paytirmaydi, rules §0.2).
	Windows []calcWindow `json:"windows,omitempty" jsonschema:"description=bir nechta oyna — turli karniz/balandlik; bir xil o'lchovli bir nechta deraza uchun qty"`
}

// calcWindow — bitta oyna (karniz kesimi) o'lchovi va unga tegishli bo'laklar.
type calcWindow struct {
	CorniceLengthM float64         `json:"cornice_length_m"           jsonschema:"description=shu oynaning karniz uzunligi metrda"`
	HeightM        float64         `json:"height_m"                   jsonschema:"description=shu oynaning parda balandligi metrda"`
	Qty            int             `json:"qty,omitempty"              jsonschema:"description=shu o'lchamdagi derazalar soni — 1 dan 20 gacha, bo'sh = 1"`
	Parts          []calcPart      `json:"parts"                      jsonschema:"description=bu oynaning mato bo'laklari"`
	Accessories    []calcAccessory `json:"accessories,omitempty"      jsonschema:"description=faqat shu oynaga tegishli bezaklar"`
	RoomName       string          `json:"room_name,omitempty"        jsonschema:"description=xona nomi — masalan mehmonxona"`
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
	VariantID string `json:"variant_id,omitempty"`
	Name      string `json:"name"` // mahsulot nomi; katalogda bo'lmasa SKU
	WidthM    string `json:"width_m,omitempty"`
	HeightM   string `json:"height_m,omitempty"`
	Coef      string `json:"coef,omitempty"`
	AreaM2    string `json:"area_m2,omitempty"`
	PriceUnit string `json:"price_unit,omitempty"`
	UnitPrice string `json:"unit_price,omitempty"`
	LineTotal string `json:"line_total"`
}

type calcServiceLine struct {
	Key       string `json:"key"`
	Name      string `json:"name,omitempty"`
	LineTotal string `json:"line_total"`
}

// calcWindowLine — bitta oynaning ixcham natija qatori: xona, o'lcham, nechta deraza
// va Django qatorlaridan yig'indi. Ko'p oynali hisobda `lines` 8 ta bilan
// chegaralanadi, shuning uchun oyna bo'yicha yig'indi shu yerda kerak (rules §5.5).
type calcWindowLine struct {
	RoomName string `json:"room_name,omitempty"`
	LengthM  string `json:"length_m,omitempty"`
	HeightM  string `json:"height_m,omitempty"`
	Qty      int    `json:"qty,omitempty"`
	Parts    int    `json:"parts,omitempty"`
	Total    string `json:"total"` // Django line_total + accessories_total
}

// calcDiscount — chegirma TARKIBI (ADR-020). Avval natijada faqat yalpi foiz
// bor edi; mijoz "12% qayerdа?" deb so'raganda model qo'lda va bosqichli
// chegirmani chalkashtirdi. Barcha sonlar Django CalcResult dan olinadi.
type calcDiscount struct {
	TierPercent    string `json:"tier_percent,omitempty"`    // bosqichli (mijoz miqdori) chegirma
	ManualPercent  string `json:"manual_percent,omitempty"`  // sotuvchi qo'lda bergan chegirma
	AppliedPercent string `json:"applied_percent,omitempty"` // jami qo'llangan foiz
	// Rule — chegirma qoidasi matni (stack_with_tier dan), mijozga tushuntirish uchun.
	Rule string `json:"rule,omitempty"`
	// ManualMaxPercent — joriy rol uchun qo'lda chegirma chegarasi. Mijoz rejimida
	// (customer_mode) HECH QACHON chiqmaydi (rules §1.6).
	ManualMaxPercent string `json:"manual_max_percent,omitempty"`
}

type curtainCalcResult struct {
	Totals      *calcTotals       `json:"totals,omitempty"`
	Windows     []calcWindowLine  `json:"windows,omitempty"`
	Lines       []calcLine        `json:"lines,omitempty"`
	Services    []calcServiceLine `json:"services,omitempty"`
	Discount    *calcDiscount     `json:"discount,omitempty"`
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
	OptionTier    string   `json:"option_tier,omitempty"    jsonschema:"description=mijoz tanlagan paket: economy|standard|premium — bo'sh bo'lsa oxirgi curtain_calculate"`
	NeedCornice   bool     `json:"need_cornice,omitempty"   jsonschema:"description=yangi karniz kerakmi"`
	NeedInstall   bool     `json:"need_install,omitempty"   jsonschema:"description=montaj kerakmi"`
	NeedDelivery  bool     `json:"need_delivery,omitempty"  jsonschema:"description=yetkazib berish kerakmi"`
	Services      []string `json:"services,omitempty"       jsonschema:"description=xizmat kalitlari — har biri ALOHIDA element: sewing / tape / hook / install / cornice; bo'sh = standart (sewing+tape+hook)"`
	DiscountPct   float64  `json:"discount_pct,omitempty"   jsonschema:"description=sotuvchi/manager qo'lda kiritgan chegirma foizi"`
	PrepaymentPct float64  `json:"prepayment_pct,omitempty" jsonschema:"description=avans %"`
}

type priceQuoteResult struct {
	Totals      *calcTotals       `json:"totals,omitempty"`
	OptionTier  string            `json:"option_tier,omitempty"` // hisoblangan paket darajasi
	Windows     []calcWindowLine  `json:"windows,omitempty"`
	Lines       []calcLine        `json:"lines,omitempty"`
	Services    []calcServiceLine `json:"services,omitempty"`
	Discount    *calcDiscount     `json:"discount,omitempty"`
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
	WidthM      string `json:"width_m,omitempty"`
	HeightM     string `json:"height_m,omitempty"`
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
	Topic string `json:"topic,omitempty" jsonschema:"description=so'rov mavzusi,enum=room_style,enum=light,enum=color,enum=construction,enum=objection,enum=general"`
	Query string `json:"query,omitempty" jsonschema:"description=so'rov matnidan asosiy so'zlar — masalan 'bolalar xonasi xavfsiz'"`
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

// orderDraftArgs — PII (ism/telefon/manzil) BU YERDA YO'Q (ADR-020). Mijoz
// ma'lumotlari LLM kontekstiga kirmaydi: ularni ilova formasiga sotuvchi
// kiritadi. `confirm` ham olib tashlandi — AI buyurtmani tasdiqlamaydi.
type orderDraftArgs struct {
	Action        string  `json:"action"                  jsonschema:"description=amal,enum=quote,enum=draft"`
	OptionTier    string  `json:"option_tier,omitempty"   jsonschema:"description=mijoz tanlagan paket: economy|standard|premium — bo'sh bo'lsa oxirgi price_quote"`
	Note          string  `json:"note,omitempty"          jsonschema:"description=buyurtmaga qo'shimcha eslatma"`
	DeadlineDays  int     `json:"deadline_days,omitempty" jsonschema:"description=bajarish muddati kunlarda; bo'sh bo'lsa do'kon standarti"`
	PrepaymentPct float64 `json:"prepayment_pct,omitempty" jsonschema:"description=avans %"`
}

type orderDraftResult struct {
	OptionTier string `json:"option_tier,omitempty"` // hisoblanadigan paket darajasi
	Quote      any    `json:"quote,omitempty"`       // Django taklifi (LastQuote)
	Display    any    `json:"display,omitempty"`     // o'qiladigan summalar: {"subtotal":"...","total":"..."}
	// CustomerFormRequired — mijoz ma'lumotlari (ism/telefon/manzil) AI orqali
	// yigilmaydi: sotuvchi ilova formasida kiritadi. Shu bayroq modelga "endi
	// savol berib tugat, forma ochilsin" signalini beradi (ADR-020).
	CustomerFormRequired bool     `json:"customer_form_required,omitempty"`
	Hint                 string   `json:"hint,omitempty"`          // AI'ga keyingi qadamni aytadi
	Summary              any      `json:"summary,omitempty"`       // qisqa referens: summa, muddat, avans (PII yo'q)
	TotalDisplay         string   `json:"total_display,omitempty"` // yakuniy summa
	Warnings             []string `json:"warnings,omitempty"`
	Error                string   `json:"error,omitempty"`
}
