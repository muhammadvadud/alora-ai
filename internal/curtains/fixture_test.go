package curtains_test

import (
	"testing"

	"github.com/muhammadvadud/alora-ai/internal/curtains/curtainstest"
)

// TestFixture_HasMultiPartCases — etalonda kamida 6 ta ko'p matoli holat (1.11):
// ko'p bo'lakli hisob (kant/tul/lambrekan/astar) eng ko'p xato beradigan joy.
func TestFixture_HasMultiPartCases(t *testing.T) {
	fx, _, err := curtainstest.Load(curtainstest.DefaultPath())
	if err != nil {
		t.Fatal(err)
	}
	multi := 0
	for _, c := range fx.Cases {
		parts := 0
		for _, m := range c.Input.Measurements {
			parts += len(m.Parts)
		}
		if parts >= 2 {
			multi++
		}
	}
	if multi < 6 {
		t.Fatalf("ko'p matoli holatlar %d < 6", multi)
	}
}
