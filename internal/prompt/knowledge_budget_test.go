package prompt

import (
	"fmt"
	"strings"
	"testing"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
)

func TestBuild_KnowledgeBudgetKeepsCompleteRules(t *testing.T) {
	rules := make([]curtains.KnowledgeRule, 100)
	for index := range rules {
		rules[index] = curtains.KnowledgeRule{ID: fmt.Sprint(index), Priority: 100 - index, Recommendation: strings.Repeat("қ", 230) + "ТУГАДИ"}
	}
	output := (Builder{}).Build(Input{Knowledge: rules, KnowledgeInlineMax: 50})
	start := strings.Index(output, "<knowledge>")
	end := strings.Index(output, "</knowledge>") + len("</knowledge>")
	block := output[start:end]
	if length := len([]rune(block)); length > 2000 {
		t.Fatalf("knowledge block=%d, budget=2000", length)
	}
	if count := strings.Count(block, "ТУГАДИ"); count == 0 || count > 8 {
		t.Fatalf("butun qoidalar soni noto'g'ri: %d", count)
	}
	for _, line := range strings.Split(block, "\n") {
		if strings.HasPrefix(line, "- ") && !strings.HasSuffix(line, "ТУГАДИ") {
			t.Fatalf("qoida yarmida uzilgan: %q", line)
		}
	}
}

func TestBuild_UsesSameConditionsAsKnowledgeLookup(t *testing.T) {
	output := (Builder{}).Build(Input{State: curtains.SessionState{Room: "kids"}, Knowledge: []curtains.KnowledgeRule{
		{ID: "children", Condition: map[string]any{"room": []string{"kids", "bedroom"}}, Recommendation: "BOLALAR XAVFSIZLIGI"},
		{ID: "unknown", Condition: map[string]any{"unsupported": "value"}, Recommendation: "MOS EMAS"},
	}})
	if !strings.Contains(output, "BOLALAR XAVFSIZLIGI") || strings.Contains(output, "MOS EMAS") {
		t.Fatal("prompt va retrieval shartlari bir xil bo'lishi kerak")
	}
}
