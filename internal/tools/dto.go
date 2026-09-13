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
	Timezone string `json:"timezone" jsonschema:"description=IANA vaqt mintaqasi, masalan Asia/Tashkent"`
}

// ---- product_search ----

type productSearchArgs struct {
	Room         string  `json:"room,omitempty"          jsonschema:"description=xona turi,enum=living,enum=bedroom,enum=wedding,enum=kids,enum=kitchen,enum=office,enum=hall"`
	Style        string  `json:"style,omitempty"         jsonschema:"description=uslub,enum=classic,enum=modern,enum=hi_tech,enum=minimal,enum=oriental,enum=provence"`
	Pattern      string  `json:"pattern,omitempty"       jsonschema:"description=bezak,enum=plain,enum=floral,enum=geometric,enum=striped,enum=jacquard"`
	Transparency string  `json:"transparency,omitempty"  jsonschema:"description=shaffoflik,enum=transparent,enum=semi,enum=blackout"`
	Layer        string  `json:"layer,omitempty"         jsonschema:"description=qatlam,enum=tul,enum=portyera,enum=lambrekan,enum=rim,enum=rulon"`
	Color        string  `json:"color,omitempty"         jsonschema:"description=istalgan rang, masalan bej yoki grafit"`
	Kind         string  `json:"kind,omitempty"          jsonschema:"description=nima qidirilyapti: mato yoki bezak/furnitura,enum=fabric,enum=accessory"`
	Segment      string  `json:"segment,omitempty"       jsonschema:"description=narx segmenti,enum=economy,enum=standard,enum=premium"`
	MaxPrice     float64 `json:"max_price,omitempty"     jsonschema:"description=birlik uchun maksimal narx so'mda (m2, dona yoki metr)"`
	Limit        int     `json:"limit,omitempty"         jsonschema:"description=nechta natija, standart 5, eng ko'pi 10"`
}

type productCard struct {
	ProductID    string   `json:"product_id"`
	VariantID    string   `json:"variant_id"`
	Name         string   `json:"name"`
	ColorName    string   `json:"color_name,omitempty"`
	ColorHex     string   `json:"color_hex,omitempty"`
	Price        string   `json:"price"`
	PriceDisplay string   `json:"price_display"`
	PriceUnit    string   `json:"price_unit"`
	WidthM       string   `json:"width_m,omitempty"`
	FabricType   string   `json:"fabric_type,omitempty"`
	Transparency string   `json:"transparency,omitempty"`
	Segment      string   `json:"segment,omitempty"`
	ImageURL     string   `json:"image_url,omitempty"`
	Why          []string `json:"why,omitempty"`
}

type productSearchResult struct {
	Products []productCard `json:"products,omitempty"`
	Count    int           `json:"count"`
	Stale    bool          `json:"stale,omitempty"`
	Warnings []string      `json:"warnings,omitempty"`
	Error    string        `json:"error,omitempty"`
}

// ---- curtain_calculate ----

type calcPart struct {
	VariantID   string  `json:"variant_id"            jsonschema:"description=shu bo'lakdagi mato varianti ID (product_search dan)"`
	Role        string  `json:"role"                  jsonschema:"description=bo'lak roli,enum=portyera,enum=tul,enum=lambrekan,enum=kant,enum=dekor,enum=astar"`
	WidthM      float64 `json:"width_m,omitempty"     jsonschema:"description=shu bo'lak egallaydigan karniz kengligi metrda; bo'sh bo'lsa butun karniz"`
	HeightM     float64 `json:"height_m,omitempty"    jsonschema:"description=shu bo'lakning balandligi metrda; bo'sh bo'lsa umumiy balandlik"`
	Coefficient float64 `json:"coefficient,omitempty" jsonschema:"description=burma koeffitsienti 1.5 2.0 2.5 3.0; bo'sh bo'lsa standart"`
}

type calcAccessory struct {
	VariantID string  `json:"variant_id" jsonschema:"description=bezak yoki furnitura varianti ID: gul, broshka, podxvat, kisti, bahroma, halqa"`
	Qty       float64 `json:"qty"        jsonschema:"description=miqdori: dona yoki metr"`
}

type curtainCalcArgs struct {
	CorniceLengthM float64         `json:"cornice_length_m"      jsonschema:"description=karniz uzunligi metrda"`
	HeightM        float64         `json:"height_m"              jsonschema:"description=pardaning umumiy balandligi metrda"`
	Parts          []calcPart      `json:"parts"                 jsonschema:"description=pardani tashkil qiluvchi mato bo'laklari, 1 tadan 10 tagacha"`
	Accessories    []calcAccessory `json:"accessories,omitempty" jsonschema:"description=matodan tashqari bezaklar: dekorativ gul, podxvat, kisti, bahroma, halqa"`
	RoomName       string          `json:"room_name,omitempty"   jsonschema:"description=xona nomi, masalan mehmonxona"`
}

type curtainCalcResult struct {
	RoomName    string   `json:"room_name,omitempty"`
	Calc        any      `json:"calc,omitempty"`    // Django preview natijasi (measurements, subtotal, total...) o'zgarishsiz
	Display     any      `json:"display,omitempty"` // o'qiladigan summalar: {"subtotal":"5 382 000 so'm",...}
	Explanation string   `json:"explanation,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
	Stale       bool     `json:"stale,omitempty"`
	Error       string   `json:"error,omitempty"`
}

// ---- price_quote ----

type priceQuoteArgs struct {
	NeedCornice   bool     `json:"need_cornice,omitempty"   jsonschema:"description=yangi karniz kerakmi"`
	NeedInstall   bool     `json:"need_install,omitempty"   jsonschema:"description=montaj kerakmi"`
	NeedDelivery  bool     `json:"need_delivery,omitempty"  jsonschema:"description=yetkazib berish kerakmi"`
	Services      []string `json:"services,omitempty"       jsonschema:"description=qo'shimcha xizmat kalitlari: sewing tape hook (bo'sh bo'lsa tikuv+tasma+ilgich avtomatik)"`
	DiscountPct   float64  `json:"discount_pct,omitempty"   jsonschema:"description=sotuvchi/manager qo'lda kiritgan chegirma foizi"`
	PrepaymentPct float64  `json:"prepayment_pct,omitempty" jsonschema:"description=avans foizi; bo'sh bo'lsa do'kon standarti"`
}

type priceQuoteResult struct {
	Quote      any      `json:"quote,omitempty"` // Django preview: services, subtotal, discount, total
	Display    any      `json:"display,omitempty"`
	Prepayment string   `json:"prepayment,omitempty"`
	Balance    string   `json:"balance,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
	Stale      bool     `json:"stale,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// ---- set_context ----

type setContextArgs struct {
	Room         string  `json:"room,omitempty"         jsonschema:"description=xona turi,enum=living,enum=bedroom,enum=wedding,enum=kids,enum=kitchen,enum=office,enum=hall"`
	Style        string  `json:"style,omitempty"        jsonschema:"description=uslub,enum=classic,enum=modern,enum=hi_tech,enum=minimal,enum=oriental,enum=provence"`
	Pattern      string  `json:"pattern,omitempty"      jsonschema:"description=bezak,enum=plain,enum=floral,enum=geometric,enum=striped,enum=jacquard"`
	Transparency string  `json:"transparency,omitempty" jsonschema:"description=shaffoflik,enum=transparent,enum=semi,enum=blackout"`
	Light        string  `json:"light,omitempty"        jsonschema:"description=yorug'lik istagi va deraza tomoni, masalan 'qorong'iroq, janubga qaraydi'"`
	Colors       string  `json:"colors,omitempty"       jsonschema:"description=xona ranglari vergul bilan: devor, mebel, pol"`
	Construction string  `json:"construction,omitempty" jsonschema:"description=deraza konstruksiyasi: rom soni, balkon eshigi, karniz bormi, shift balandligi"`
	BudgetMin    float64 `json:"budget_min,omitempty"   jsonschema:"description=budjet pastki chegarasi so'mda"`
	BudgetMax    float64 `json:"budget_max,omitempty"   jsonschema:"description=budjet yuqori chegarasi so'mda"`
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
