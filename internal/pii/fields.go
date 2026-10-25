package pii

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	nameFieldRE    = regexp.MustCompile(`(?i)(^|[\s,;])((?:mening\s+ismim|ismim|mijoz(?:ning)?\s+ismi|меня\s+зовут|имя\s+клиента|фио)[ \t]*[:=-]?[ \t]*)([\p{L}][\p{L}'’ʼʻ-]*(?:[ \t]+[\p{L}][\p{L}'’ʼʻ-]*){0,3})`)
	addressFieldRE = regexp.MustCompile(`(?i)(^|[\s,;])((?:(?:manzilim|mijoz(?:ning)?\s+manzili|мой\s+адрес)(?:[ \t]*[:=][ \t]*|[ \t]+)|(?:manzil|manzili|адрес)[ \t]*[:=][ \t]*))([^;\r\n!?]+)`)
	emailRE        = regexp.MustCompile(`[\p{L}0-9._%+\-]+@[\p{L}0-9.\-]+\.[\p{L}]{2,}`)
)

func scrubStructured(text string) (string, bool) {
	raw := strings.TrimSpace(text)
	if raw == "" || raw[0] != '{' && raw[0] != '[' || !json.Valid([]byte(raw)) {
		return "", false
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return "", false
	}
	value, changed := scrubValue(value)
	if !changed {
		return text, true
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", false
	}
	return string(encoded), true
}

func scrubValue(value any) (any, bool) {
	changed := false
	switch current := value.(type) {
	case map[string]any:
		for key, item := range current {
			var next any
			var different bool
			if marker := personalField(key); marker != "" && item != nil {
				text := fmt.Sprint(item)
				if marker == "phone" {
					next = text
					if !strings.Contains(text, "****") {
						next = MaskPhone(text)
					}
				} else {
					next = marker
				}
				different = next != text
			} else {
				_, identifier := item.(string)
				if identifier && (key == "id" || key == "sku" || key == "code" || strings.HasSuffix(key, "_id")) {
					next = item
				} else {
					next, different = scrubValue(item)
				}
			}
			current[key], changed = next, changed || different
		}
	case []any:
		for index, item := range current {
			next, different := scrubValue(item)
			current[index], changed = next, changed || different
		}
	case string:
		if isDecimalText(current) {
			return current, false
		}
		next := Scrub(current)
		return next, next != current
	}
	return value, changed
}

func personalField(key string) string {
	switch strings.ToLower(key) {
	case "customer_name", "full_name", "first_name", "last_name", "fio":
		return "[ism yashirildi]"
	case "customer_address", "address":
		return "[manzil yashirildi]"
	case "email", "customer_email":
		return "[email yashirildi]"
	case "phone", "customer_phone", "phone_number", "telephone", "mobile", "telefon":
		return "phone"
	}
	return ""
}

func isDecimalText(text string) bool {
	if text == "" {
		return false
	}
	for _, char := range text {
		if char < '0' || char > '9' {
			if char != '.' && char != '-' && char != '+' {
				return false
			}
		}
	}
	return true
}

func scrubLocalPhones(text string) string {
	var out strings.Builder
	previous := 0
	for _, position := range phoneLocalRE.FindAllStringIndex(text, -1) {
		start, end := position[0], position[1]
		out.WriteString(text[previous:start])
		suffix := strings.ToLower(strings.TrimSpace(text[end:]))
		money := false
		for _, unit := range []string{"so'm", "so‘m", "so’m", "soʼm", "uzs", "сум"} {
			money = money || strings.HasPrefix(suffix, unit)
		}
		longNumber := start > 0 && text[start-1] >= '0' && text[start-1] <= '9' || end < len(text) && text[end] >= '0' && text[end] <= '9'
		if money || longNumber {
			out.WriteString(text[start:end])
		} else {
			out.WriteString(MaskPhone(text[start:end]))
		}
		previous = end
	}
	out.WriteString(text[previous:])
	return out.String()
}
