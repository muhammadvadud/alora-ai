package tools

import (
	"reflect"
	"testing"
)

func TestNormalizeServiceKeys(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"bitta satrda uchta kalit", []string{"sewing tape hook"}, []string{"sewing", "tape", "hook"}},
		{"vergul bilan", []string{"sewing, tape"}, []string{"sewing", "tape"}},
		{"bo'sh joy va katta harf", []string{"  Sewing "}, []string{"sewing"}},
		{"dublikat", []string{"sewing", "sewing tape"}, []string{"sewing", "tape"}},
		{"bo'sh", nil, []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := normalizeServiceKeys(c.in); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}
