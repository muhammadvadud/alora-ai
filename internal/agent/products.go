package agent

import "context"

// ProductRef — ilovada kartochka chizish uchun mahsulot havolasi (proto ProductRef bilan bir xil).
// agent domenni bilmaydi (rules §2), shuning uchun bu tur curtains.ProductCard emas — yassi nusxa.
type ProductRef struct {
	VariantID   string  `json:"variant_id"`
	ProductName string  `json:"product_name"`
	ColorName   string  `json:"color_name,omitempty"`
	ColorHex    string  `json:"color_hex,omitempty"`
	Price       float64 `json:"price"`
	ImageURL    string  `json:"image_url,omitempty"`
	SKU         string  `json:"sku,omitempty"`
}

type productCollectorKey struct{}

// ProductCollector — bitta so'rov davomida tool'lar topgan mahsulotlarni yig'adi.
// Nega kontekst orqali: tool handler'lari Event kanalini bilmaydi va bilmasligi kerak;
// Chat.Stream har tool natijasidan keyin yig'ilganini KindProducts event'iga aylantiradi.
type ProductCollector struct{ refs []ProductRef }

// WithProductCollector — so'rov kontekstiga yangi yig'uvchi qo'shadi (Chat.Stream chaqiradi).
func WithProductCollector(ctx context.Context) (context.Context, *ProductCollector) {
	c := &ProductCollector{}
	return context.WithValue(ctx, productCollectorKey{}, c), c
}

// AttachProducts — tool o'z natijasidagi mahsulotlarni ilovaga ko'rsatish uchun belgilaydi.
// Yig'uvchi bo'lmasa (CLI'dan tashqari test) jimgina o'tkazib yuboriladi.
func AttachProducts(ctx context.Context, refs []ProductRef) {
	if c, ok := ctx.Value(productCollectorKey{}).(*ProductCollector); ok && c != nil {
		c.refs = append(c.refs, refs...)
	}
}

// Drain — yig'ilganlarni qaytaradi va yig'uvchini bo'shatadi (har tool natijasidan keyin).
func (c *ProductCollector) Drain() []ProductRef {
	if c == nil || len(c.refs) == 0 {
		return nil
	}
	out := c.refs
	c.refs = nil
	return out
}
