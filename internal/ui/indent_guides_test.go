package ui

import (
	"reflect"
	"testing"
)

func TestIndentGuideWidths(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		tabW  int
		want  []int
	}{
		{"plain", []string{"foo", "    foo", "        foo"}, 4, []int{0, 4, 8}},
		{"misaligned", []string{"      foo"}, 4, []int{6}},
		{"tabs", []string{"\tfoo", "\t\tfoo"}, 4, []int{4, 8}},
		{"mixed", []string{"  \tfoo"}, 4, []int{4}},
		{"blank inherits prev", []string{"    a", "", "  b"}, 4, []int{4, 4, 2}},
		{"leading blank is zero", []string{"", "  b"}, 4, []int{0, 2}},
		{"whitespace-only inherits", []string{"\ta", "\t", "b"}, 4, []int{4, 4, 0}},
		{"empty", nil, 4, []int{}},
	}
	for _, c := range cases {
		if got := indentGuideWidths(c.lines, c.tabW); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: indentGuideWidths = %v, want %v", c.name, got, c.want)
		}
	}
}
