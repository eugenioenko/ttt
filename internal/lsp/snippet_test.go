package lsp

import (
	"fmt"
	"strings"
	"testing"
)

func TestExpandSnippet(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		text  string
		stops []SnippetStop
	}{
		{
			name:  "html attribute",
			src:   `class="$1"`,
			text:  `class=""`,
			stops: []SnippetStop{{Index: 1, Offset: 7}},
		},
		{
			name:  "final cursor",
			src:   `align-items: $0;`,
			text:  `align-items: ;`,
			stops: []SnippetStop{{Index: 0, Offset: 13}},
		},
		{
			name:  "default",
			src:   `${1:div}`,
			text:  `div`,
			stops: []SnippetStop{{Index: 1, Length: 3}},
		},
		{
			name:  "nested",
			src:   `${1:foo ${2:bar}}`,
			text:  `foo bar`,
			stops: []SnippetStop{{Index: 1, Length: 7}, {Index: 2, Offset: 4, Length: 3}},
		},
		{
			name:  "escaped dollar",
			src:   `\$1`,
			text:  `$1`,
			stops: nil,
		},
		{
			name:  "scss variable is not a stop",
			src:   `$primary`,
			text:  `$primary`,
			stops: nil,
		},
		{
			name:  "choice takes first option",
			src:   `${1|a,b|}`,
			text:  `a`,
			stops: []SnippetStop{{Index: 1, Length: 1}},
		},
		{
			name:  "multi line",
			src:   "a\n${1:b}\n$0",
			text:  "a\nb\n",
			stops: []SnippetStop{{Index: 1, Offset: 2, Length: 1}, {Index: 0, Offset: 4}},
		},
		{
			name:  "escaped choice comma",
			src:   `${1|one\,two,three|}`,
			text:  `one,two`,
			stops: []SnippetStop{{Index: 1, Length: 7}},
		},
		{
			name:  "escaped choice pipe",
			src:   `${1|a\|b,c|}`,
			text:  `a|b`,
			stops: []SnippetStop{{Index: 1, Length: 3}},
		},
		{
			name:  "escaped choice backslash",
			src:   `${1|a\\,b|}`,
			text:  `a\`,
			stops: []SnippetStop{{Index: 1, Length: 2}},
		},
		{
			name:  "escaped brace in default",
			src:   `${1:a\}b}`,
			text:  `a}b`,
			stops: []SnippetStop{{Index: 1, Length: 3}},
		},
		{
			name: "trailing dollar",
			src:  `abc$`,
			text: `abc$`,
		},
		{
			name: "trailing braced dollar",
			src:  `abc${`,
			text: `abc${`,
		},
		{
			name: "unterminated default",
			src:  `${1:a`,
			text: `${1:a`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandSnippet(tt.src)
			if got.Text != tt.text {
				t.Fatalf("text = %q, want %q", got.Text, tt.text)
			}
			if len(got.Stops) != len(tt.stops) {
				t.Fatalf("stops = %#v, want %#v", got.Stops, tt.stops)
			}
			for i := range tt.stops {
				if got.Stops[i] != tt.stops[i] {
					t.Fatalf("stop %d = %#v, want %#v", i, got.Stops[i], tt.stops[i])
				}
			}
		})
	}
}

func TestExpandSnippetUnterminatedDefaultsTerminate(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 200; i++ {
		fmt.Fprintf(&b, "${%d:a ", i)
	}
	src := b.String()

	got := ExpandSnippet(src)
	if got.Text != src {
		t.Fatalf("text changed: got %q, want input unchanged", got.Text)
	}
	if len(got.Stops) != 0 {
		t.Fatalf("stops = %#v, want none", got.Stops)
	}
}

func TestSnippetExpansionFirstStop(t *testing.T) {
	tests := []struct {
		name string
		exp  SnippetExpansion
		want SnippetStop
		ok   bool
	}{
		{
			name: "lowest numbered stop",
			exp:  SnippetExpansion{Stops: []SnippetStop{{Index: 2}, {Index: 1}, {Index: 0}}},
			want: SnippetStop{Index: 1},
			ok:   true,
		},
		{
			name: "falls back to zero",
			exp:  SnippetExpansion{Stops: []SnippetStop{{Index: 0}}},
			want: SnippetStop{Index: 0},
			ok:   true,
		},
		{
			name: "no stops",
			exp:  SnippetExpansion{},
			ok:   false,
		},
		{
			name: "duplicate keeps first",
			exp:  SnippetExpansion{Stops: []SnippetStop{{Index: 1, Offset: 4}, {Index: 1, Offset: 2}}},
			want: SnippetStop{Index: 1, Offset: 2},
			ok:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.exp.FirstStop()
			if ok != tt.ok || got != tt.want {
				t.Fatalf("FirstStop() = %#v, %v; want %#v, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}
