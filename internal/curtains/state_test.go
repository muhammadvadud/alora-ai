package curtains_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/muhammadvadud/alora-ai/internal/curtains"
	"github.com/shopspring/decimal"
)

func TestSessionState_Apply(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name          string
		initialStage  curtains.Stage
		tool          string
		args          map[string]any
		expectedStage curtains.Stage
		check         func(t *testing.T, s curtains.SessionState)
	}{
		{
			name:          "set_context updates fields",
			initialStage:  curtains.StageNeed,
			tool:          "set_context",
			args:          map[string]any{"room": "mehmonxona", "style": "hi-tech", "budget_min": 100.5, "budget_max": "500.5", "colors": "oq,qora"},
			expectedStage: curtains.StageNeed,
			check: func(t *testing.T, s curtains.SessionState) {
				if s.Room != "mehmonxona" {
					t.Errorf("expected room mehmonxona, got %s", s.Room)
				}
				if s.Style != "hi-tech" {
					t.Errorf("expected style hi-tech, got %s", s.Style)
				}
				if s.BudgetMin.String() != "100.5" {
					t.Errorf("expected budget_min 100.5, got %s", s.BudgetMin.String())
				}
				if s.BudgetMax.String() != "500.5" {
					t.Errorf("expected budget_max 500.5, got %s", s.BudgetMax.String())
				}
				if len(s.Colors) != 2 || s.Colors[0] != "oq" || s.Colors[1] != "qora" {
					t.Errorf("expected colors [oq,qora], got %v", s.Colors)
				}
			},
		},
		{
			name:          "product_search moves to recommend and updates fields",
			initialStage:  curtains.StageNeed,
			tool:          "product_search",
			args:          map[string]any{"room": "yotoqxona", "style": "klassika", "pattern": "gulli", "transparency": "qalin"},
			expectedStage: curtains.StageRecommend,
			check: func(t *testing.T, s curtains.SessionState) {
				if s.Room != "yotoqxona" {
					t.Errorf("expected room yotoqxona, got %s", s.Room)
				}
				if s.Pattern != "gulli" {
					t.Errorf("expected pattern gulli, got %s", s.Pattern)
				}
			},
		},
		{
			name:          "product_search does not move stage backward",
			initialStage:  curtains.StageCalc,
			tool:          "product_search",
			args:          map[string]any{},
			expectedStage: curtains.StageCalc,
			check:         func(t *testing.T, s curtains.SessionState) {},
		},
		{
			name:         "curtain_calculate moves to calc and parses parts (float64/string)",
			initialStage: curtains.StageRecommend,
			tool:         "curtain_calculate",
			args: map[string]any{
				"cornice_length_m": 4.5,
				"height_m":         "2.8",
				"parts": []any{
					map[string]any{
						"variant_id":  "v1",
						"role":        "portyera",
						"width_m":     2.0,
						"height_m":    "2.85",
						"coefficient": 2, // integer also float64 in json
					},
				},
			},
			expectedStage: curtains.StageCalc,
			check: func(t *testing.T, s curtains.SessionState) {
				if s.CorniceLengthM.String() != "4.5" {
					t.Errorf("expected cornice 4.5, got %s", s.CorniceLengthM.String())
				}
				if s.HeightM.String() != "2.8" {
					t.Errorf("expected height 2.8, got %s", s.HeightM.String())
				}
				if len(s.Parts) != 1 {
					t.Fatalf("expected 1 part, got %d", len(s.Parts))
				}
				p := s.Parts[0]
				if p.VariantID != "v1" || string(p.Role) != "portyera" {
					t.Errorf("expected v1/portyera, got %s/%s", p.VariantID, p.Role)
				}
				if p.WidthM.String() != "2" || p.HeightM.String() != "2.85" || p.Coef.String() != "2" {
					t.Errorf("expected part values 2, 2.85, 2, got %s, %s, %s", p.WidthM.String(), p.HeightM.String(), p.Coef.String())
				}
			},
		},
		{
			name:          "build_options moves to variants",
			initialStage:  curtains.StageCalc,
			tool:          "build_options",
			args:          nil,
			expectedStage: curtains.StageVariants,
			check:         func(t *testing.T, s curtains.SessionState) {},
		},
		{
			name:          "order_draft confirm moves to order",
			initialStage:  curtains.StageVariants,
			tool:          "order_draft",
			args:          map[string]any{"action": "confirm"},
			expectedStage: curtains.StageOrder,
			check:         func(t *testing.T, s curtains.SessionState) {},
		},
		{
			name:          "order_draft quote does not move to order",
			initialStage:  curtains.StageVariants,
			tool:          "order_draft",
			args:          map[string]any{"action": "quote"},
			expectedStage: curtains.StageVariants,
			check:         func(t *testing.T, s curtains.SessionState) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := curtains.NewSessionState()
			s.Stage = tt.initialStage
			s.Apply(tt.tool, tt.args, now)

			if s.Stage != tt.expectedStage {
				t.Errorf("expected stage %s, got %s", tt.expectedStage, s.Stage)
			}
			if s.UpdatedAt != now {
				t.Errorf("expected UpdatedAt to be updated")
			}
			tt.check(t, s)
		})
	}
}

func TestSessionState_NextQuestion(t *testing.T) {
	s := curtains.NewSessionState()

	expectedSequence := []string{
		"room",
		"light",
		"pattern",
		"style",
		"colors",
		"construction",
		"budget",
		"cornice_length_m",
		"height_m",
		"",
	}

	for i, expectedField := range expectedSequence {
		field, hint := s.NextQuestion()
		if field != expectedField {
			t.Fatalf("step %d: expected field %s, got %s (hint: %s)", i, expectedField, field, hint)
		}

		switch field {
		case "room":
			s.Room = "mehmonxona"
		case "light":
			s.LightPreference = "quyoshli"
		case "pattern":
			s.Pattern = "gulsiz"
		case "style":
			s.Style = "hi-tech"
		case "colors":
			s.Colors = []string{"oq"}
		case "construction":
			s.Construction = "rom"
		case "budget":
			s.BudgetMin = decimal.NewFromInt(100)
		case "cornice_length_m":
			s.CorniceLengthM = decimal.NewFromFloat(4.2)
		case "height_m":
			s.HeightM = decimal.NewFromFloat(2.8)
		}
	}
}

func TestSessionState_PromptBlock(t *testing.T) {
	s := curtains.NewSessionState()
	s.Stage = curtains.StageNeed
	s.Room = "mehmonxona"
	s.Style = "hi-tech"
	s.CorniceLengthM = decimal.NewFromFloat(4.2)
	s.HeightM = decimal.NewFromFloat(2.6)
	s.Layers = []string{"tul", "portyera"}

	block := s.PromptBlock()
	expected := "Hozirgi holat: bosqich=need; xona=mehmonxona; uslub=hi-tech; karniz=4.2 m; balandlik=2.6 m; qatlamlar=tul,portyera"

	if block != expected {
		t.Errorf("expected PromptBlock to be %q, got %q", expected, block)
	}

	// Test max length 400 chars
	s.Room = strings.Repeat("A", 400)
	longBlock := s.PromptBlock()
	if len([]rune(longBlock)) != 400 {
		t.Errorf("expected PromptBlock char length to be 400, got %d", len([]rune(longBlock)))
	}
	if !strings.HasSuffix(longBlock, "…") {
		t.Errorf("expected long PromptBlock to end with '…'")
	}
}

func TestSessionState_Summary(t *testing.T) {
	s := curtains.NewSessionState()
	s.Stage = curtains.StageCalc
	s.Room = "mehmonxona"
	s.CorniceLengthM = decimal.NewFromFloat(3.0)
	s.LastCalc = &curtains.CalcResult{
		Total: decimal.NewFromInt(5000),
	}
	s.Parts = []curtains.PartSpec{{VariantID: "v1"}}

	summary := s.Summary()

	if summary["stage"] != "calc" {
		t.Errorf("expected stage calc in summary")
	}
	if summary["room"] != "mehmonxona" {
		t.Errorf("expected room in summary")
	}
	if val, ok := summary["cornice_length_m"].(decimal.Decimal); !ok || val.String() != "3" {
		t.Errorf("expected cornice 3 in summary")
	}
	if summary["parts"] != 1 {
		t.Errorf("expected parts 1 in summary")
	}
	if val, ok := summary["last_total"].(decimal.Decimal); !ok || val.String() != "5000" {
		t.Errorf("expected last_total 5000 in summary")
	}
	if _, ok := summary["style"]; ok {
		t.Errorf("expected style to be absent")
	}
}

func TestSessionState_JSONRoundTrip(t *testing.T) {
	s := curtains.NewSessionState()
	s.Stage = curtains.StageNeed
	s.Room = "yotoqxona"
	s.BudgetMin = decimal.NewFromFloat(100.5)
	s.Colors = []string{"qizil", "oq"}
	s.Parts = []curtains.PartSpec{
		{
			Role:      curtains.RoleTul,
			VariantID: "v1",
			WidthM:    decimal.NewFromFloat(2.5),
		},
	}
	s.LastCalc = &curtains.CalcResult{
		Total: decimal.NewFromInt(1234),
	}
	s.UpdatedAt = time.Now().Truncate(time.Millisecond) // JSON loses nano precision sometimes depending on marshal

	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var s2 curtains.SessionState
	err = json.Unmarshal(b, &s2)
	if err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if s2.Room != s.Room || s2.BudgetMin.String() != s.BudgetMin.String() || len(s2.Colors) != len(s.Colors) {
		t.Errorf("roundtrip mismatch: basic fields")
	}
	if len(s2.Parts) != 1 || s2.Parts[0].WidthM.String() != s.Parts[0].WidthM.String() {
		t.Errorf("roundtrip mismatch: parts")
	}
	if s2.LastCalc == nil || s2.LastCalc.Total.String() != s.LastCalc.Total.String() {
		t.Errorf("roundtrip mismatch: LastCalc")
	}
	if !s2.UpdatedAt.Equal(s.UpdatedAt) {
		t.Errorf("roundtrip mismatch: time")
	}
}

func TestSessionState_SetStage(t *testing.T) {
	s := curtains.NewSessionState()
	s.Stage = curtains.StageCalc

	s.SetStage(curtains.StageRecommend)
	if s.Stage != curtains.StageRecommend {
		t.Errorf("expected stage to be set backwards to recommend")
	}
}
