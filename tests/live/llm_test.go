//go:build live

package live

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/muhammadvadud/alora-ai/gen/alorav1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

var liveMoneyRe = regexp.MustCompile(`(?i)\d[\d\s]{4,}\s*(?:so['’]m|сум)`)

type llmRun struct {
	name, provider string
	tools          []string
	content        string
	toolResults    string
	unbackedPrice  string
	stage          string
	ttft, total    time.Duration
	tokens         int
	hallucination  bool
	err            error
}

var firstRuns struct {
	sync.Mutex
	items []llmRun
}

func runLLMScenario(t *testing.T, c alorav1.AgentClient, sc llmScenario, n int) llmRun {
	t.Helper()
	r := llmRun{name: sc.Name}
	sid := fmt.Sprintf("live-quality-%d-%02d", time.Now().UnixNano(), n)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, "x-service-token", "devtoken", "x-request-id", fmt.Sprintf("live-%d-%d", n, time.Now().UnixNano()))
	start := time.Now()
	for i, msg := range sc.Messages {
		stream, err := c.Chat(ctx, &alorav1.ChatRequest{SessionId: sid, UserId: "live-quality", TenantId: "live", Message: msg, Locale: sc.Locale, Context: map[string]string{"conversation_id": sid, "role": sc.Role, "customer_mode": boolString(sc.CustomerMode)}})
		if err != nil {
			r.err = fmt.Errorf("xabar %d: %w", i+1, err)
			return r
		}
		for {
			ev, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				r.err = fmt.Errorf("xabar %d oqimi: %w", i+1, err)
				return r
			}
			switch ev.Kind {
			case "content":
				if r.ttft == 0 {
					r.ttft = time.Since(start)
				}
				r.content += ev.Content
			case "tool_call":
				r.tools = append(r.tools, ev.Tool)
			case "tool_result":
				r.toolResults += ev.Result
			case "stage":
				r.stage = ev.Content
			case "error":
				r.err = fmt.Errorf("agent error: %s", ev.Error)
			case "done":
				if ev.Usage != nil {
					r.provider = ev.Usage.Provider
					r.tokens += int(ev.Usage.PromptTokens + ev.Usage.CompletionTokens)
				}
			}
		}
	}
	r.total = time.Since(start)
	r.unbackedPrice = firstUnbackedPrice(r.content, r.toolResults)
	r.hallucination = r.unbackedPrice != ""
	return r
}

func validateLLMRun(sc llmScenario, r llmRun) []string {
	var problems []string
	if r.err != nil {
		problems = append(problems, r.err.Error())
	}
	if len(sc.ExpectToolsAny) > 0 && !containsAny(r.tools, sc.ExpectToolsAny) {
		problems = append(problems, fmt.Sprintf("kutilgan tool'lardan biri chaqirilmadi: want=%v got=%v", sc.ExpectToolsAny, r.tools))
	}
	for _, forbidden := range sc.ForbidTools {
		if contains(r.tools, forbidden) {
			problems = append(problems, "taqiqlangan tool chaqirildi: "+forbidden)
		}
	}
	if sc.ExpectStage != "" && r.stage != sc.ExpectStage {
		problems = append(problems, fmt.Sprintf("stage=%q, want=%q", r.stage, sc.ExpectStage))
	}
	for _, phrase := range sc.MustNotContain {
		if strings.Contains(strings.ToLower(r.content), strings.ToLower(phrase)) {
			problems = append(problems, "taqiqlangan ibora: "+phrase)
		}
	}
	if r.hallucination {
		problems = append(problems, "tool natijasida asosi bo'lmagan narx: "+r.unbackedPrice+" — "+r.content)
	}
	return problems
}

func TestLLM_Scenarios(t *testing.T) {
	c, closeConn := liveAgent(t)
	defer closeConn()
	runs := runSuite(t, c, 0)
	firstRuns.Lock()
	firstRuns.items = append([]llmRun(nil), runs...)
	firstRuns.Unlock()
	assertRuns(t, runs)
}

func TestLLM_HallucinationRate(t *testing.T) {
	c, closeConn := liveAgent(t)
	defer closeConn()
	firstRuns.Lock()
	base := append([]llmRun(nil), firstRuns.items...)
	firstRuns.Unlock()
	if len(base) != len(llmScenarios) {
		base = runSuite(t, c, 0)
	}
	second := runSuite(t, c, 1)
	all := make([]llmRun, 0, len(base)+len(second))
	all = append(all, base...)
	all = append(all, second...)
	var hallucinations int
	for _, r := range all {
		if r.hallucination {
			hallucinations++
		}
	}
	logRuns(t, all)
	rate := float64(hallucinations) / float64(len(all))
	t.Logf("hallucination ulushi: %d/%d = %.1f%% (40 suhbatlik audit; TZ 100 suhbat talabi xarajat limiti sabab qisqartirildi)", hallucinations, len(all), rate*100)
	if rate >= 0.05 {
		t.Fatalf("hallucination ulushi 5%% dan yuqori: %d/%d", hallucinations, len(all))
	}
	assertRuns(t, second)
}

func runSuite(t *testing.T, c alorav1.AgentClient, round int) []llmRun {
	t.Helper()
	runs := make([]llmRun, len(llmScenarios))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, sc := range llmScenarios {
		wg.Add(1)
		go func(i int, sc llmScenario) {
			defer wg.Done()
			sem <- struct{}{}
			runs[i] = runLLMScenario(t, c, sc, round*100+i)
			<-sem
		}(i, sc)
	}
	wg.Wait()
	return runs
}

func assertRuns(t *testing.T, runs []llmRun) {
	for i, r := range runs {
		for _, problem := range validateLLMRun(llmScenarios[i], r) {
			t.Errorf("%s: %s", r.name, problem)
		}
	}
	logRuns(t, runs)
}

func logRuns(t *testing.T, runs []llmRun) {
	ordered := append([]llmRun(nil), runs...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].name < ordered[j].name })
	t.Log("ssenariy | provider | tool'lar | ttft | total | tokens | hallucination")
	for _, r := range ordered {
		t.Logf("%s | %s | %s | %s | %s | %d | %t", r.name, r.provider, strings.Join(r.tools, ","), r.ttft.Round(time.Millisecond), r.total.Round(time.Millisecond), r.tokens, r.hallucination)
	}
}

func liveAgent(t *testing.T) (alorav1.AgentClient, func()) {
	t.Helper()
	addr := getenvOrSkip(t, "ALORA_LIVE_AGENT")
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("agentga ulanish: %v", err)
	}
	return alorav1.NewAgentClient(conn), func() { _ = conn.Close() }
}

func getenvOrSkip(t *testing.T, name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		t.Skip(name + " belgilanmagan")
	}
	return value
}

func boolString(v bool) string {
	if v {
		return "1"
	}
	return "0"
}
func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
func containsAny(xs, wants []string) bool {
	for _, want := range wants {
		if contains(xs, want) {
			return true
		}
	}
	return false
}

// hasHallucinatedPrice — content'dagi har bir summa tool natijalari ichida bo'lishi shart;
// birinchi asossiz summa qaytariladi (diagnostika uchun — qaysi raqam o'ylab topilgani ko'rinsin).
func hasHallucinatedPrice(content, results string) bool {
	return firstUnbackedPrice(content, results) != ""
}

func firstUnbackedPrice(content, results string) string {
	resultDigits := onlyDigits(results)
	for _, price := range liveMoneyRe.FindAllString(content, -1) {
		digits := onlyDigits(price)
		if digits != "" && !strings.Contains(resultDigits, digits) {
			return strings.TrimSpace(price)
		}
	}
	return ""
}

func onlyDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
