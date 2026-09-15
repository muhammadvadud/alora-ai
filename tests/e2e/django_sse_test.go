//go:build live

package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestDjangoSSE_AgentEndToEnd(t *testing.T) {
	base := strings.TrimRight(envOrSkip(t, "ALORA_E2E_DJANGO", "http://localhost:8000"), "/")
	token := os.Getenv("ALORA_E2E_JWT")
	if token == "" {
		phone := envOrSkip(t, "ALORA_E2E_PHONE", "")
		password := envOrSkip(t, "ALORA_E2E_PASSWORD", "")
		token = login(t, base, phone, password)
	}
	conversationID := os.Getenv("ALORA_E2E_CONVERSATION_ID")
	if conversationID == "" {
		conversationID = createConversation(t, base, token)
	}

	first := postSSE(t, base, token, conversationID, "Mehmonxona uchun hi-tech parda kerak, katalogdan mos mahsulotlarni top")
	assertKinds(t, first, []string{"tool_call", "products", "content", "done"})
	if !hasTool(first, "set_context") && !hasTool(first, "product_search") {
		t.Fatalf("birinchi xabarda kutilgan tool chaqirilmadi: %#v", first)
	}
	if !hasKind(first, "products") {
		t.Fatalf("birinchi xabarda products event yo'q: %#v", first)
	}
	if usage := doneUsage(first); usage == nil {
		t.Fatalf("done usage'siz: %#v", first)
	}

	second := postSSE(t, base, token, conversationID, "Karniz 4.2 m, balandlik 2.8 m, hisobla")
	assertKinds(t, second, []string{"tool_call", "content", "done"})
	if !hasTool(second, "curtain_calculate") {
		t.Fatalf("curtain_calculate chaqirilmadi: %#v", second)
	}
	resultTotal := findTotal(second)
	content := concatContent(second)
	if resultTotal == "" {
		t.Fatal("curtain_calculate tool_result ichida total topilmadi")
	}
	// AI summani "3 245 000 so'm" deb yozadi (display), tool esa "3245000" — raqamlar
	// bo'shliqsiz solishtiriladi; matndagi HAR summa tool natijalari ichida bo'lishi shart.
	if !strings.Contains(digitsOnly(content), digitsOnly(resultTotal)) {
		t.Fatalf("content tool totalini aks ettirmadi: total=%q content=%q", resultTotal, content)
	}
	resultDigits := digitsOnly(concatToolResults(second))
	for _, m := range regexp.MustCompile(`\d[\d\s]{4,}\s*so'm`).FindAllString(content, -1) {
		if !strings.Contains(resultDigits, digitsOnly(m)) {
			t.Fatalf("content'da tool natijasidan tashqari summa bor: %q", m)
		}
	}

	reset(t, base, token, conversationID)
	assertPersisted(t, base, token, conversationID)
	assertUsageForConversation(t, base, token, conversationID)
}

func hasKind(events []map[string]any, want string) bool {
	for _, event := range events {
		if event["kind"] == want {
			return true
		}
	}
	return false
}

func hasTool(events []map[string]any, want string) bool {
	for _, event := range events {
		if event["kind"] == "tool_call" && event["tool"] == want {
			return true
		}
	}
	return false
}

func doneUsage(events []map[string]any) map[string]any {
	for _, event := range events {
		if event["kind"] == "done" {
			usage, _ := event["usage"].(map[string]any)
			return usage
		}
	}
	return nil
}

func postSSE(t *testing.T, base, token, conversationID, content string) []map[string]any {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"content": content})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, base+"/api/v1/conversations/"+conversationID+"/messages/", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("SSE status=%d body=%s", resp.StatusCode, raw)
	}

	var events []map[string]any
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			t.Fatalf("SSE JSON: %v (%s)", err, line)
		}
		events = append(events, event)
		if event["kind"] == "done" {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return events
}

func assertKinds(t *testing.T, events []map[string]any, expected []string) {
	t.Helper()
	var kinds []string
	for _, event := range events {
		kinds = append(kinds, event["kind"].(string))
	}
	pos := 0
	for _, kind := range kinds {
		if pos < len(expected) && kind == expected[pos] {
			pos++
		}
	}
	if pos != len(expected) {
		t.Fatalf("event tartibi: got=%v expected subsequence=%v", kinds, expected)
	}
}

func findTotal(events []map[string]any) string {
	for _, event := range events {
		if event["kind"] != "tool_result" {
			continue
		}
		var value any
		if err := json.Unmarshal([]byte(fmt.Sprint(event["result"])), &value); err != nil {
			continue
		}
		if total := nestedString(value, "total"); total != "" {
			return total
		}
	}
	return ""
}

func nestedString(value any, key string) string {
	switch typed := value.(type) {
	case map[string]any:
		if result, ok := typed[key].(string); ok {
			return result
		}
		for _, child := range typed {
			if result := nestedString(child, key); result != "" {
				return result
			}
		}
	case []any:
		for _, child := range typed {
			if result := nestedString(child, key); result != "" {
				return result
			}
		}
	}
	return ""
}

func concatContent(events []map[string]any) string {
	var content string
	for _, event := range events {
		if event["kind"] == "content" {
			content += fmt.Sprint(event["content"])
		}
	}
	return content
}

func login(t *testing.T, base, phone, password string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"phone": phone, "password": password})
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, base+"/api/v1/auth/login/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result struct {
		Access string `json:"access"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || result.Access == "" {
		t.Fatalf("JWT olish muvaffaqiyatsiz: status=%d err=%v", resp.StatusCode, err)
	}
	return result.Access
}

func createConversation(t *testing.T, base, token string) string {
	t.Helper()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, base+"/api/v1/conversations/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || result.ID == "" {
		t.Fatalf("suhbat yaratilmadi: status=%d err=%v", resp.StatusCode, err)
	}
	return result.ID
}

func reset(t *testing.T, base, token, conversationID string) {
	t.Helper()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, base+"/api/v1/conversations/"+conversationID+"/reset/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reset status=%d", resp.StatusCode)
	}
}

func assertPersisted(t *testing.T, base, token, conversationID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, base+"/api/v1/conversations/"+conversationID+"/messages/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			var payload any
			_ = json.NewDecoder(resp.Body).Decode(&payload)
			resp.Body.Close()
			if list, ok := payload.([]any); ok && len(list) >= 2 {
				return
			}
			if object, ok := payload.(map[string]any); ok {
				if list, ok := object["results"].([]any); ok && len(list) >= 2 {
					return
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("Django Message yozuvlari 5 sekund ichida ko'rinmadi")
}

// assertUsageForConversation — Go outbox → Django ReportUsage → Celery → UsageRecord.
// `usage/summary` kunlik rollup'dan (cron) o'qiydi, shuning uchun darhol ko'rinmaydi;
// `usage/conversations` esa UsageRecord'lardan to'g'ridan-to'g'ri yig'adi.
func assertUsageForConversation(t *testing.T, base, token, conversationID string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, base+"/api/v1/usage/conversations/?period=day", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			var rows []struct {
				ConversationID string `json:"conversation_id"`
				Tokens         int    `json:"tokens"`
			}
			decodeErr := json.NewDecoder(resp.Body).Decode(&rows)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK && decodeErr == nil {
				for _, r := range rows {
					if r.ConversationID == conversationID && r.Tokens > 0 {
						return
					}
				}
			} else if resp.StatusCode != http.StatusOK {
				t.Fatalf("UsageRecord tekshiruvi status=%d", resp.StatusCode)
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatal("Django usage/conversations 8 sekund ichida shu suhbat uchun usage ko'rsatmadi")
}

func envOrSkip(t *testing.T, key, fallback string) string {
	t.Helper()
	if value := os.Getenv(key); value != "" {
		return value
	}
	if fallback != "" {
		return fallback
	}
	t.Skipf("%s berilmagan", key)
	return ""
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func concatToolResults(events []map[string]any) string {
	var out string
	for _, event := range events {
		if event["kind"] == "tool_result" {
			out += fmt.Sprint(event["result"])
		}
	}
	return out
}
