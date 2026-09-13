// Package telegram — agentni Telegram bot sifatida ishlatuvchi transport.
// Boshqa transportlar kabi faqat agent.Chat.Stream(...) <-chan Event ni render qiladi.
// Har suhbat (chat) alohida sessiya — xotira chat bo'yicha saqlanadi.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/muhammadvadud/alora-agent/internal/agent"
	"github.com/muhammadvadud/alora-agent/internal/rocketpark"
)

const apiBase = "https://api.telegram.org/bot"
const defaultAdminChatID int64 = 1769851684

// Serve long-polling orqali Telegram yangilanishlarini o'qiydi va agentga uzatadi.
// ctx bekor bo'lganda (SIGINT/SIGTERM) toza to'xtaydi.
// Options — transport sozlamalari. Nol maydonlar standart qiymat oladi.
type Options struct {
	Token       string
	AdminChatID int64
	// EditInterval — stream paytida xabarni yangilash oralig'i. 0 = 800ms.
	EditInterval time.Duration
}

func Serve(ctx context.Context, chat *agent.Chat, opts Options) error {
	if opts.EditInterval <= 0 {
		opts.EditInterval = defaultEditInterval
	}
	if opts.AdminChatID == 0 {
		opts.AdminChatID = defaultAdminChatID
	}
	b := &bot{
		token:        opts.Token,
		chat:         chat,
		adminChatID:  opts.AdminChatID,
		editInterval: opts.EditInterval,
		http:         &http.Client{Timeout: longPoll + 10*time.Second},
		notified:     make(map[string]string),
	}
	me, err := b.getMe(ctx)
	if err != nil {
		return fmt.Errorf("telegram bot tokenini tekshirish: %w", err)
	}
	// Webhook o'rnatilgan bo'lsa getUpdates (long-polling) 409 beradi — o'chiramiz.
	if err := b.call(ctx, "deleteWebhook", map[string]any{"drop_pending_updates": false}, nil); err != nil {
		slog.Warn("deleteWebhook xatosi", "err", err)
	}
	slog.Info("telegram bot ishga tushdi", "username", me)

	var offset int
	for {
		if ctx.Err() != nil {
			return nil
		}
		updates, err := b.getUpdates(ctx, offset)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.Warn("telegram getUpdates xatosi", "err", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(3 * time.Second):
			}
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			switch {
			case u.Message != nil && strings.TrimSpace(u.Message.Text) != "":
				go b.handleMessage(ctx, u.Message)
			case u.CallbackQuery != nil:
				go b.handleCallback(ctx, u.CallbackQuery)
				continue
			}
		}
	}
}

const longPoll = 30 * time.Second

type bot struct {
	token        string
	chat         *agent.Chat
	adminChatID  int64
	editInterval time.Duration
	http         *http.Client
	mu           sync.Mutex
	notified     map[string]string
}

func (b *bot) handleMessage(ctx context.Context, m *message) {
	chatID := m.Chat.ID
	text := strings.TrimSpace(m.Text)
	sessionID := strconv.FormatInt(chatID, 10)

	if text == "/start" {
		_ = b.chat.Reset(ctx, sessionID)
		_ = b.clearOrderState(sessionID)
		_ = b.sendMessageWithMarkup(ctx, chatID, startText(), startKeyboard())
		return
	}
	if text == "/reset" {
		_ = b.chat.Reset(ctx, sessionID)
		_ = b.clearOrderState(sessionID)
		_ = b.sendMessage(ctx, chatID, "Suhbat tozalandi. Endi yangi zakazni paket nomidan boshlaymiz.")
		return
	}
	if text == "/menu" {
		_ = b.sendMessageWithMarkup(ctx, chatID, menuText(), menuKeyboard())
		return
	}

	_ = b.sendChatAction(ctx, chatID, "typing")

	// Streaming: birinchi matn kelganda xabar yuboramiz, keyin content o'sgani sari
	// editMessageText bilan yangilaymiz (throttle bilan — Telegram rate-limit uchun).
	// Oqim davomida oddiy matn (qisman Markdown buzilmasin uchun), oxirida HTML formatli.
	var (
		reply    strings.Builder
		msgID    int
		lastEdit time.Time
	)
	for ev := range b.chat.Stream(ctx, sessionID, text) {
		switch ev.Kind {
		case agent.KindContent:
			reply.WriteString(ev.Content)
			cur := reply.String()
			if len(cur) > streamMaxLen {
				continue // juda uzun — stream qilmaymiz, oxirida to'liq yuboriladi
			}
			if msgID == 0 {
				if strings.TrimSpace(cur) == "" {
					continue
				}
				if id, err := b.sendText(ctx, chatID, cur, ""); err == nil {
					msgID, lastEdit = id, time.Now()
				}
			} else if time.Since(lastEdit) >= b.editInterval {
				_ = b.editText(ctx, chatID, msgID, cur, "", nil)
				lastEdit = time.Now()
			}
		case agent.KindToolResult:
			if ev.Tool == "rocket_park_order" {
				if order, ok := parseRocketParkOrderResult(ev.Result); ok && order.isConfirmed() {
					if b.markOrderNotified(sessionID, order.fingerprint()) {
						_ = b.sendAdminOrder(ctx, chatID, m, order, text)
					}
				}
			}
		case agent.KindError:
			_ = b.sendMessage(ctx, chatID, "Kechirasiz, xatolik yuz berdi: "+ev.Err)
			return
		}
	}

	out := strings.TrimSpace(reply.String())
	if out == "" {
		out = "Kechirasiz, javob bera olmadim. Qayta urinib ko'ring."
	}
	// Yakuniy holat: oqim boshlanган va matn qisqa bo'lsa — oxirgi xabarni HTML formatli
	// qilib yangilaymiz; aks holda to'liq (bo'linadigan, formatli) yangi xabar yuboramiz.
	if msgID != 0 && len(out) <= streamMaxLen {
		if err := b.editText(ctx, chatID, msgID, markdownToHTML(out), "HTML", nil); err != nil {
			_ = b.editText(ctx, chatID, msgID, out, "", nil) // HTML tahlili xato bo'lsa — oddiy matn
		}
	} else if err := b.sendMessage(ctx, chatID, out); err != nil {
		slog.Warn("telegram sendMessage xatosi", "chat", chatID, "err", err)
	}
}

func (b *bot) handleCallback(ctx context.Context, q *callbackQuery) {
	if q == nil || strings.TrimSpace(q.Data) == "" {
		return
	}
	_ = b.answerCallbackQuery(ctx, q.ID, "")
	chatID := q.Message.Chat.ID
	sessionID := strconv.FormatInt(chatID, 10)
	data := strings.TrimSpace(q.Data)

	switch {
	case data == "menu:prices":
		_ = b.editCallbackMessage(ctx, q.Message, pricesText(), menuKeyboard())
	case data == "menu:packages":
		_ = b.editCallbackMessage(ctx, q.Message, packagesText(), packagesKeyboard())
	case data == "menu:promotions":
		_ = b.editCallbackMessage(ctx, q.Message, promotionsText(), menuKeyboard())
	case data == "menu:order":
		_ = b.editCallbackMessage(ctx, q.Message, orderIntroText(), packagesKeyboard())
	case strings.HasPrefix(data, "pkg:"):
		code := strings.TrimPrefix(data, "pkg:")
		if p, ok := rocketpark.MatchPackage(code); ok {
			_ = b.editCallbackMessage(ctx, q.Message, packageDetailText(p), orderAfterPackageKeyboard())
		} else {
			_ = b.editCallbackMessage(ctx, q.Message, "❌ Paket topilmadi. Iltimos, qayta tanlang.", packagesKeyboard())
		}
	case data == "session:reset":
		_ = b.chat.Reset(ctx, sessionID)
		_ = b.clearOrderState(sessionID)
		_ = b.editCallbackMessage(ctx, q.Message, "🧹 Suhbat tozalandi. Yangi zakazni paketdan boshlaymiz.", startKeyboard())
	default:
		_ = b.editCallbackMessage(ctx, q.Message, "⚠️ Noma'lum tugma. Qayta urinib ko'ring.", menuKeyboard())
	}
}

const (
	defaultEditInterval = 800 * time.Millisecond // rate-limit uchun ~1-2 tahrir/sek
	// streamMaxLen — Telegram'ning QAT'IY chegarasi (4096) atrofida; sozlama emas.
	streamMaxLen = 3900
)

// ---- Telegram Bot API (minimal) ----

type message struct {
	MessageID int    `json:"message_id"`
	Text string `json:"text"`
	Chat struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	From *user `json:"from,omitempty"`
}

type update struct {
	UpdateID      int            `json:"update_id"`
	Message       *message       `json:"message"`
	CallbackQuery *callbackQuery `json:"callback_query"`
}

type user struct {
	ID        int64  `json:"id"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type callbackQuery struct {
	ID      string   `json:"id"`
	From    *user    `json:"from,omitempty"`
	Data    string   `json:"data,omitempty"`
	Message *message `json:"message,omitempty"`
}

func (b *bot) getMe(ctx context.Context) (string, error) {
	var res struct {
		Username string `json:"username"`
	}
	if err := b.call(ctx, "getMe", nil, &res); err != nil {
		return "", err
	}
	return res.Username, nil
}

func (b *bot) getUpdates(ctx context.Context, offset int) ([]update, error) {
	q := url.Values{}
	q.Set("offset", strconv.Itoa(offset))
	q.Set("timeout", strconv.Itoa(int(longPoll.Seconds())))
	q.Set("allowed_updates", `["message","callback_query"]`)

	var updates []update
	err := b.call(ctx, "getUpdates?"+q.Encode(), nil, &updates)
	return updates, err
}

func (b *bot) sendMessage(ctx context.Context, chatID int64, text string) error {
	return b.sendMessageWithMarkup(ctx, chatID, text, nil)
}

func (b *bot) sendMessageWithMarkup(ctx context.Context, chatID int64, text string, replyMarkup any) error {
	// Telegram xabar chegarasi 4096 belgi — uzun javobni bo'lib yuboramiz.
	for _, chunk := range splitMessage(text, 3900) {
		// Model Markdown chiqaradi — uni Telegram HTML'iga o'giramiz (bold/havola ko'rinsin).
		// HTML tahlili xato bo'lsa Telegram 400 beradi — oddiy matn bilan qayta yuboramiz.
		body := map[string]any{
			"chat_id":                  chatID,
			"text":                     markdownToHTML(chunk),
			"parse_mode":               "HTML",
			"disable_web_page_preview": true,
		}
		if replyMarkup != nil {
			body["reply_markup"] = replyMarkup
		}
		if err := b.call(ctx, "sendMessage", body, nil); err != nil {
			plain := map[string]any{"chat_id": chatID, "text": chunk, "disable_web_page_preview": true}
			if replyMarkup != nil {
				plain["reply_markup"] = replyMarkup
			}
			if err2 := b.call(ctx, "sendMessage", plain, nil); err2 != nil {
				return err2
			}
		}
	}
	return nil
}

func startText() string {
	return strings.Join([]string{
		"👋 Assalomu alaykum! Men Rocket Park zakaz yordamchisiman.",
		"⚡ Qisqacha tariflar:",
		"🍕 Pizza setlar: 10 bola — 2 500 000 so'm, 15 bola — 3 500 000 so'm, 20 bola — 4 500 000 so'm",
		"🍔 Burger setlar: 10 bola — 2 700 000 so'm, 15 bola — 3 700 000 so'm, 20 bola — 4 700 000 so'm",
		"🎮 O'yin narxlari: 1 soat — 100 000 so'm, cheklanmagan vaqt — 120 000 so'm, jeton — 20 000 so'm",
		"👇 Tugmalardan tanlang yoki savol yozing.",
	}, "\n")
}

func menuText() string {
	return strings.Join([]string{
		"🧭 Men bilan nimalar qilsa bo'ladi:",
		"• 📦 paketlar va tariflarni ko'rish",
		"• 🧾 zakazni boshlash",
		"• ✨ aksiyalarni bilish",
		"• 💬 savol yozib, javob olish",
	}, "\n")
}

func pricesText() string {
	return strings.Join([]string{
		"💰 Rocket Park tariflari:",
		"• 1 soat — 100 000 so'm",
		"• cheklanmagan vaqt — 120 000 so'm",
		"• kuzatuvchi uchun kirish — 20 000 so'm",
		"• jeton — 20 000 so'm",
	}, "\n")
}

func packagesText() string {
	return strings.Join([]string{
		"📦 Rocket Park paketlari:",
		"• 🍕 Standart Pizza — 10 bola, 2 500 000 so'm",
		"• 🍕 Premium Pizza — 15 bola, 3 500 000 so'm",
		"• 🍕 Lux Pizza — 20 bola, 4 500 000 so'm",
		"• 🍔 Standart Burger — 10 bola, 2 700 000 so'm",
		"• 🍔 Premium Burger — 15 bola, 3 700 000 so'm",
		"• 🍔 Lux Burger — 20 bola, 4 700 000 so'm",
		"Har bir paket ichida o'yin maydoni, jetonlar va ovqatlar bor.",
	}, "\n")
}

func promotionsText() string {
	return strings.Join([]string{
		"✨ Amaldagi aksiyalar:",
		"• 21:00 dan keyin — 50% chegirma",
		"• tug'ilgan kun egasiga — 50% chegirma",
		"• tug'ilgan kun mehmonlariga — 3+1",
		"• ko'p bolali oilalar uchun — 3+1",
	}, "\n")
}

func packageDetailText(p rocketpark.MenuPackage) string {
	return strings.Join([]string{
		fmt.Sprintf("✅ %s", p.Name),
		fmt.Sprintf("🧒 Sig'im: %d bola", p.Capacity),
		fmt.Sprintf("💰 Narx: %s so'm", formatMoney(p.PriceUZS)),
		"📋 Ichiga kiradi:",
		"- " + strings.Join(p.Items, "\n- "),
		"",
		"📝 Endi faqat shularni yozing: bolalar soni, sana, vaqt, ism va telefon.",
	}, "\n")
}

func orderIntroText() string {
	return strings.Join([]string{
		"🧾 Zakazni boshlaymiz.",
		"1) Paketni tanlang.",
		"2) Bolalar soni, sana, vaqt, ism va telefonni yozing.",
		"3) Men qolganini tartiblayman.",
		"",
		"Misol: `Premium Pizza, 15 bola, 2026-09-01, 18:30, Samandar, +998901234567`",
	}, "\n")
}

func orderAfterPackageKeyboard() map[string]any {
	return inlineKeyboard(
		[]inlineButton{
			{Text: "🧾 Zakaz boshlash", CallbackData: "menu:order"},
			{Text: "📦 Boshqa paket", CallbackData: "menu:packages"},
		},
		[]inlineButton{
			{Text: "🧹 Tozalash", CallbackData: "session:reset"},
			{Text: "💰 Tariflar", CallbackData: "menu:prices"},
		},
	)
}

func startKeyboard() map[string]any {
	return inlineKeyboard(
		[]inlineButton{
			{Text: "💰 Tariflar", CallbackData: "menu:prices"},
			{Text: "📦 Paketlar", CallbackData: "menu:packages"},
		},
		[]inlineButton{
			{Text: "✨ Aksiyalar", CallbackData: "menu:promotions"},
			{Text: "🧾 Zakaz boshlash", CallbackData: "menu:order"},
		},
		[]inlineButton{
			{Text: "🧹 Suhbatni tozalash", CallbackData: "session:reset"},
		},
	)
}

func menuKeyboard() map[string]any { return startKeyboard() }

func packagesKeyboard() map[string]any {
	rows := [][]inlineButton{
		{{Text: "🍕 Standart Pizza", CallbackData: "pkg:standard_pizza"}, {Text: "🍕 Premium Pizza", CallbackData: "pkg:premium_pizza"}},
		{{Text: "🍕 Lux Pizza", CallbackData: "pkg:lux_pizza"}, {Text: "🍔 Standart Burger", CallbackData: "pkg:standard_burger"}},
		{{Text: "🍔 Premium Burger", CallbackData: "pkg:premium_burger"}, {Text: "🍔 Lux Burger", CallbackData: "pkg:lux_burger"}},
		{{Text: "↩️ Ortga", CallbackData: "menu:order"}},
	}
	return inlineKeyboard(rows...)
}

func (b *bot) editCallbackMessage(ctx context.Context, m *message, text string, replyMarkup any) error {
	if m == nil {
		return b.sendMessageWithMarkup(ctx, 0, text, replyMarkup)
	}
	return b.editText(ctx, m.Chat.ID, m.MessageID, markdownToHTML(text), "HTML", replyMarkup)
}

type inlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
}

func inlineKeyboard(rows ...[]inlineButton) map[string]any {
	return map[string]any{"inline_keyboard": rows}
}

type rocketParkOrderResult struct {
	Action       string   `json:"action,omitempty"`
	PackageCode  string   `json:"package_code,omitempty"`
	PackageName  string   `json:"package_name,omitempty"`
	Category     string   `json:"category,omitempty"`
	Variant      string   `json:"variant,omitempty"`
	Capacity     int      `json:"capacity,omitempty"`
	PriceUZS     int      `json:"price_uzs,omitempty"`
	Children     int      `json:"children,omitempty"`
	Date         string   `json:"date,omitempty"`
	Time         string   `json:"time,omitempty"`
	CustomerName string   `json:"customer_name,omitempty"`
	Phone        string   `json:"phone,omitempty"`
	EventType    string   `json:"event_type,omitempty"`
	Notes        string   `json:"notes,omitempty"`
	Missing      []string `json:"missing,omitempty"`
	Suggestions  []string `json:"suggestions,omitempty"`
	Summary      string   `json:"summary,omitempty"`
	Error        string   `json:"error,omitempty"`
}

func (r rocketParkOrderResult) isConfirmed() bool {
	return strings.TrimSpace(r.Action) == "confirm" &&
		strings.TrimSpace(r.Error) == "" &&
		len(r.Missing) == 0 &&
		strings.TrimSpace(r.PackageCode) != "" &&
		strings.TrimSpace(r.Phone) != "" &&
		strings.TrimSpace(r.CustomerName) != ""
}

func (r rocketParkOrderResult) fingerprint() string {
	return strings.Join([]string{
		r.Action,
		r.PackageCode,
		r.PackageName,
		r.Date,
		r.Time,
		strconv.Itoa(r.Children),
		r.CustomerName,
		r.Phone,
		r.EventType,
		r.Notes,
	}, "|")
}

func parseRocketParkOrderResult(raw string) (rocketParkOrderResult, bool) {
	var r rocketParkOrderResult
	if strings.TrimSpace(raw) == "" {
		return r, false
	}
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return r, false
	}
	return r, true
}

func filterNonEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func formatMoney(v int) string {
	s := strconv.Itoa(v)
	if len(s) <= 3 {
		return s
	}
	n := len(s)
	var out strings.Builder
	for i, r := range s {
		if i > 0 && (n-i)%3 == 0 {
			out.WriteByte(' ')
		}
		out.WriteRune(r)
	}
	return out.String()
}

var (
	mdBold = regexp.MustCompile(`\*\*(.+?)\*\*`)
	mdLink = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
)

// markdownToHTML modelning Markdown javobini Telegram HTML'iga o'giradi:
// **qalin** -> <b>, [matn](url) -> <a>. Qolgan belgilar HTML uchun himoyalanadi.
func markdownToHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = mdBold.ReplaceAllString(s, "<b>$1</b>")
	s = mdLink.ReplaceAllString(s, `<a href="$2">$1</a>`)
	return s
}

func (b *bot) sendChatAction(ctx context.Context, chatID int64, action string) error {
	return b.call(ctx, "sendChatAction", map[string]any{"chat_id": chatID, "action": action}, nil)
}

func (b *bot) answerCallbackQuery(ctx context.Context, callbackID, text string) error {
	body := map[string]any{"callback_query_id": callbackID}
	if strings.TrimSpace(text) != "" {
		body["text"] = text
	}
	return b.call(ctx, "answerCallbackQuery", body, nil)
}

// sendText bitta xabar yuboradi va uning message_id sini qaytaradi (stream uchun).
func (b *bot) sendText(ctx context.Context, chatID int64, text, parseMode string) (int, error) {
	body := map[string]any{"chat_id": chatID, "text": text, "disable_web_page_preview": true}
	if parseMode != "" {
		body["parse_mode"] = parseMode
	}
	var res struct {
		MessageID int `json:"message_id"`
	}
	err := b.call(ctx, "sendMessage", body, &res)
	return res.MessageID, err
}

// editText mavjud xabar matnini yangilaydi (streaming effekti uchun).
func (b *bot) editText(ctx context.Context, chatID int64, msgID int, text, parseMode string, replyMarkup any) error {
	body := map[string]any{
		"chat_id":                  chatID,
		"message_id":               msgID,
		"text":                     text,
		"disable_web_page_preview": true,
	}
	if parseMode != "" {
		body["parse_mode"] = parseMode
	}
	if replyMarkup != nil {
		body["reply_markup"] = replyMarkup
	}
	return b.call(ctx, "editMessageText", body, nil)
}

func (b *bot) sendAdminOrder(ctx context.Context, chatID int64, m *message, order rocketParkOrderResult, input string) error {
	if b.adminChatID == 0 {
		return nil
	}
	buyer := "noma'lum"
	if m != nil && m.From != nil {
		parts := []string{m.From.FirstName, m.From.LastName}
		buyer = strings.TrimSpace(strings.Join(filterNonEmpty(parts), " "))
		if buyer == "" && m.From.Username != "" {
			buyer = "@" + m.From.Username
		}
	}
	text := strings.Join([]string{
		"Yangi Rocket Park zakazi!",
		fmt.Sprintf("Chat ID: %d", chatID),
		fmt.Sprintf("Telegram mijoz: %s", buyer),
		fmt.Sprintf("Paket: %s (%s)", order.PackageName, order.PackageCode),
		fmt.Sprintf("Bola soni: %d", order.Children),
		fmt.Sprintf("Sana: %s", order.Date),
		fmt.Sprintf("Vaqt: %s", order.Time),
		fmt.Sprintf("Ism: %s", order.CustomerName),
		fmt.Sprintf("Telefon: %s", order.Phone),
		fmt.Sprintf("Tadbir turi: %s", order.EventType),
		fmt.Sprintf("Izoh: %s", order.Notes),
		fmt.Sprintf("Kiruvchi matn: %s", input),
	}, "\n")
	return b.sendMessage(ctx, b.adminChatID, text)
}

func (b *bot) markOrderNotified(sessionID, fingerprint string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if prev, ok := b.notified[sessionID]; ok && prev == fingerprint {
		return false
	}
	b.notified[sessionID] = fingerprint
	return true
}

func (b *bot) clearOrderState(sessionID string) error {
	b.mu.Lock()
	delete(b.notified, sessionID)
	b.mu.Unlock()
	return nil
}

// call Bot API metodini chaqiradi va {ok, result} javobidan result'ni out'ga yozadi.
func (b *bot) call(ctx context.Context, method string, body, out any) error {
	var reader *bytes.Reader
	httpMethod := http.MethodGet
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
		httpMethod = http.MethodPost
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, httpMethod, apiBase+b.token+"/"+method, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := b.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var env struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return err
	}
	if !env.OK {
		return fmt.Errorf("telegram %s: %s", method, env.Description)
	}
	if out != nil {
		return json.Unmarshal(env.Result, out)
	}
	return nil
}

// splitMessage matnni limitdan oshmaydigan bo'laklarga bo'ladi (imkon qadar qator chegarasidan).
func splitMessage(s string, limit int) []string {
	if len(s) <= limit {
		return []string{s}
	}
	var parts []string
	for len(s) > limit {
		cut := strings.LastIndex(s[:limit], "\n")
		if cut <= 0 {
			cut = limit
		}
		parts = append(parts, s[:cut])
		s = strings.TrimPrefix(s[cut:], "\n")
	}
	if s != "" {
		parts = append(parts, s)
	}
	return parts
}
