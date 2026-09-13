package golden

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/muhammadvadud/alora-agent/internal/agent"
	"github.com/muhammadvadud/alora-agent/internal/curtains"
	"github.com/muhammadvadud/alora-agent/internal/session"
	"github.com/muhammadvadud/alora-agent/internal/tools"
	"gopkg.in/yaml.v3"
)

// YAML ssenariy formati (rules §14.4): `steps: [{user, llm, expect_tools,
// expect_stage, expect_products, expect_error_contains, expect_result_contains,
// forbid_content_regex, clock_advance}]`.
type scenario struct {
	Name  string `yaml:"name"`
	Role  string `yaml:"role"`
	Steps []step `yaml:"steps"`
}

type step struct {
	User                 string    `yaml:"user"`
	LLM                  []llmStep `yaml:"llm"`
	ExpectTools          []string  `yaml:"expect_tools"`
	ExpectStage          string    `yaml:"expect_stage"`
	ExpectProducts       *bool     `yaml:"expect_products,omitempty"`
	ExpectErrorContains  string    `yaml:"expect_error_contains,omitempty"`
	ExpectResultContains []string  `yaml:"expect_result_contains,omitempty"`
	ForbidContentRegex   []string  `yaml:"forbid_content_regex,omitempty"`
	ClockAdvance         string    `yaml:"clock_advance,omitempty"` // time.ParseDuration: "11m"
}

const (
	testSessionID = "golden-s1"
	staleAfter    = 10 * time.Minute
)

// testClockBase — barcha ssenariylar uchun qat'iy boshlanish vaqti (determinizm).
var testClockBase = time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)

// harness — bitta ssenariy uchun bitta barqaror muhit: katalog, soat, hisob,
// suhbat xotirasi. Ssenariy qadamlari bitta sessiya sifatida ketma-ket ishlaydi
// (stage/history xotirasi qadamlar orasida saqlanadi).
type harness struct {
	clock     *curtains.FakeClock
	catalog   *curtains.Catalog
	calc      *fakeCalc
	store     session.Store
	sessionID string
}

func newHarness() *harness {
	clock := curtains.NewFakeClock(testClockBase)
	catalog := curtains.NewCatalog(clock, staleAfter)
	catalog.ApplySnapshot(testSnapshot())
	store := session.NewMemory()
	return &harness{
		clock: clock, catalog: catalog,
		calc:      &fakeCalc{view: catalog.View()},
		store:     store,
		sessionID: testSessionID,
	}
}

// stageHook — agent.StageReporter: tool'lar state'ni o'zlari yangilaydi
// (OnToolCall bekor), Stage esa sessiya holatidan joriy bosqichni qaytaradi.
type stageHook struct {
	store session.Store
}

func (h *stageHook) OnToolCall(context.Context, string, string, string) {}

func (h *stageHook) Stage(ctx context.Context, sessionID string) (string, string) {
	st, err := h.store.State(ctx, sessionID)
	if err != nil {
		return "", ""
	}
	summary, _ := json.Marshal(st.Summary())
	return string(st.Stage), string(summary)
}

// runStep bitta qadamni (bir foydalanuvchi xabari) to'liq agent navbati sifatida
// ishga tushiradi: yangi mock model + yangi Chat (xotira/katalog/soat saqlanadi).
func (h *harness) runStep(ctx context.Context, st step) (bool, []string) {
	if st.ClockAdvance != "" {
		d, err := time.ParseDuration(st.ClockAdvance)
		if err != nil {
			return false, []string{"clock_advance noto'g'ri: " + err.Error()}
		}
		h.clock.Advance(d)
	}

	model := &mockModel{script: st.LLM}
	deps := tools.Deps{Catalog: h.catalog, Calc: h.calc, State: h.store, Clock: h.clock}
	chat, err := agent.NewChat(ctx, model, tools.Alora(deps), h.store, agent.Options{
		Stream: true, StateHook: &stageHook{store: h.store},
		Provider: "golden-mock", Model: "script",
	})
	if err != nil {
		return false, []string{"NewChat: " + err.Error()}
	}

	rd := collect(chat.Stream(ctx, h.sessionID, st.User))
	return checkStep(st, rd, model)
}

// stepData — bitta qadamda kuzatilgan Event oqimi.
type stepData struct {
	toolSeq     []string
	results     []string
	content     strings.Builder
	sawProducts bool
	stage       string
	agentErr    string
}

func collect(evs <-chan agent.Event) stepData {
	var rd stepData
	for ev := range evs {
		switch ev.Kind {
		case agent.KindToolCall:
			rd.toolSeq = append(rd.toolSeq, ev.Tool)
		case agent.KindToolResult:
			rd.results = append(rd.results, ev.Result)
		case agent.KindContent:
			rd.content.WriteString(ev.Content)
		case agent.KindProducts:
			if len(ev.Products) > 0 {
				rd.sawProducts = true
			}
		case agent.KindStage:
			rd.stage = ev.Content
		case agent.KindError:
			rd.agentErr = ev.Err
		}
	}
	return rd
}

// checkStep kutilgan tool ketma-ketligi, stage, products, error matni, natija
// qismlari, forbid_content_regex va (doim) hallucination qoidasini tekshiradi.
func checkStep(st step, rd stepData, model *mockModel) (bool, []string) {
	var msgs []string
	if rd.agentErr != "" {
		msgs = append(msgs, "agent xatosi: "+rd.agentErr)
	}
	if model.exhausted {
		msgs = append(msgs, "mock skript tugadi — model kutilganidan ortiq chaqirildi")
	}
	if len(st.ExpectTools) > 0 && !slices.Equal(rd.toolSeq, st.ExpectTools) {
		msgs = append(msgs, fmt.Sprintf("tool ketma-ketligi = %v, kutilgan = %v", rd.toolSeq, st.ExpectTools))
	}
	if st.ExpectStage != "" && rd.stage != st.ExpectStage {
		msgs = append(msgs, fmt.Sprintf("stage = %q, kutilgan = %q", rd.stage, st.ExpectStage))
	}
	if st.ExpectProducts != nil && rd.sawProducts != *st.ExpectProducts {
		msgs = append(msgs, fmt.Sprintf("products event bo'lishi = %v, kutilgan = %v", rd.sawProducts, *st.ExpectProducts))
	}
	joined := strings.Join(rd.results, "\n")
	if st.ExpectErrorContains != "" && !toolErrorContains(rd.results, st.ExpectErrorContains) {
		msgs = append(msgs, "tool error'ida qism topilmadi: "+st.ExpectErrorContains)
	}
	for _, want := range st.ExpectResultContains {
		if !strings.Contains(joined, want) {
			msgs = append(msgs, fmt.Sprintf("tool natijasida qism topilmadi: %q", want))
		}
	}
	for _, pat := range st.ForbidContentRegex {
		if regexp.MustCompile(pat).MatchString(rd.content.String()) {
			msgs = append(msgs, "forbid_content_regex javobga mos tushdi: "+pat)
		}
	}
	msgs = append(msgs, checkHallucination(rd.content.String(), joined)...)
	return len(msgs) == 0, msgs
}

// toolErrorContains har bir tool natijasi JSON'dan `error` maydonini o'qib,
// qidirilgan qism bor-borligini tekshiradi (errors-as-results, rules §1.3).
func toolErrorContains(results []string, sub string) bool {
	for _, r := range results {
		if r == "" {
			continue
		}
		var m map[string]any
		if json.Unmarshal([]byte(r), &m) != nil {
			continue
		}
		if s, ok := m["error"].(string); ok && strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// moneyRe — rules §14.4 dagi narx naqshi: kamida 6 belgi (raqam+bo'shliq) bilan
// "so'm". Buni javob matnidagi har bir narx tool natijasida asoslangan bo'lishi
// kerakligini tekshirish uchun ishlatamiz (0 hallucination, TZ §10.2).
var moneyRe = regexp.MustCompile(`\d[\d\s]{5,}\s*so'm`)

// checkHallucination javob matnidagi har narx uchun: raqamlar tool natijasida
// uchrasa bu — asosli narx (MASALAN 4 680 000 curtain_calculate dan chiqdi);
// uchramasa — AI narxni o'zi to'qigan (hallucination, tasks 1.42).
func checkHallucination(content, results string) []string {
	cleanResults := stripWhite(results)
	var msgs []string
	for _, match := range moneyRe.FindAllString(content, -1) {
		digits := moneyDigits(match)
		if digits != "" && !strings.Contains(cleanResults, digits) {
			msgs = append(msgs, "hallucination: javobda tool natijasida bo'lmagan narx: "+strings.TrimSpace(match))
		}
	}
	return msgs
}

// moneyDigits "2 500 000 so'm" → "2500000" (probellarni tashlab, so'm suffixini olib).
func moneyDigits(m string) string {
	in, _, _ := strings.Cut(strings.TrimSpace(m), "so'm")
	return stripWhite(in)
}

// stripWhite barcha whitespace belgilarini olib tashlaydi — raqamlar taqqoslanishi
// uchun "5 070 000" va "5070000" bir xil ko'rinishga kelishi kerak.
func stripWhite(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// scenarioResult — ssenariy yakuni: ok() — hamma qadam o'tdi; describe() — xatolar.
type scenarioResult struct {
	messages []string
}

func (r scenarioResult) ok() bool         { return len(r.messages) == 0 }
func (r scenarioResult) describe() string { return strings.Join(r.messages, "\n") }

// runScenario bitta ssenariyni ishga tushiradi: har qadam uchun yangi agent,
// lekin bitta sessiya (history/stage xotirasi qadamlar orasida saqlanadi).
func runScenario(sc scenario) scenarioResult {
	h := newHarness()
	role := curtains.UserRole(sc.Role)
	if role == "" {
		role = curtains.RoleEmployee
	}
	ctx := curtains.WithUser(context.Background(), curtains.UserInfo{Role: role})

	var msgs []string
	for i, st := range sc.Steps {
		ok, stepMsgs := h.runStep(ctx, st)
		if !ok {
			msgs = append(msgs, fmt.Sprintf("qadam %d (%q): %s", i+1, st.User, strings.Join(stepMsgs, "; ")))
		}
	}
	return scenarioResult{messages: msgs}
}

// TestGoldenScenarios tests/golden/scenarios/*.yaml dagi barcha ssenariylarni
// yuklab ishga tushiradi (tasks 1.41: 5 ta MVP ssenariy).
func TestGoldenScenarios(t *testing.T) {
	files, err := filepath.Glob("scenarios/*.yaml")
	if err != nil {
		t.Fatalf("senariylarni topib bo'lmadi: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("tests/golden/scenarios/ da yaml senariy topilmadi")
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: o'qish: %v", f, err)
		}
		var sc scenario
		if err := yaml.Unmarshal(b, &sc); err != nil {
			t.Fatalf("%s: yaml: %v", f, err)
		}
		if sc.Name == "" {
			sc.Name = f
		}
		t.Run(sc.Name, func(t *testing.T) {
			if res := runScenario(sc); !res.ok() {
				t.Fatalf("ssenariy muvaffaqiyatsiz:\n%s", res.describe())
			}
		})
	}
}
