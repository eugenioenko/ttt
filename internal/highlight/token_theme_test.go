package highlight

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func ptr(s string) *string { return &s }

func TestTokenThemeResolvesLikeVSCode(t *testing.T) {
	theme := NewTokenTheme([]TokenRule{
		{Selector: "keyword", Foreground: "#111111"},
		{Selector: "keyword.control", Foreground: "#222222"},
		{Selector: "string", Foreground: "#333333"},
		{Selector: "source.css string", Foreground: "#444444"},
		{Selector: "markup.italic", FontStyle: ptr("italic")},
		{Selector: "comment", Foreground: "#555555", FontStyle: ptr("italic")},
		{Selector: "comment.line.todo", FontStyle: ptr("")},
		{Selector: "string", Foreground: "#666666"},
	})
	style := func(names ...string) TokenStyle {
		s := theme.resolve(names)
		if s == term.StyleDefault {
			return TokenStyle{Fg: "none"}
		}
		return theme.Styles[s-term.TokenStyle(0)]
	}
	for name, tc := range map[string]struct {
		got, want TokenStyle
	}{
		"more specific scope wins":     {style("source.ts", "keyword.control.flow.ts"), TokenStyle{Fg: "#222222"}},
		"later rule wins a tie":        {style("source.ts", "string.quoted.ts"), TokenStyle{Fg: "#666666"}},
		"parent selector wins":         {style("source.css", "string.quoted.css"), TokenStyle{Fg: "#444444"}},
		"font style alone keeps color": {style("text.md", "markup.italic.md"), TokenStyle{Italic: true}},
		"inner level inherits outer":   {style("source.ts", "comment.block.ts", "punctuation.x"), TokenStyle{Fg: "#555555", Italic: true}},
		"empty font style resets":      {style("source.ts", "comment.line.todo.ts"), TokenStyle{Fg: "#555555"}},
		"no match is default":          {style("source.ts", "variable.other.ts"), TokenStyle{Fg: "none"}},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: got %+v, want %+v", name, tc.got, tc.want)
		}
	}
	if NewTokenTheme(nil) != nil {
		t.Error("empty rules should compile to nil")
	}
}

func TestHighlighterFollowsTokenThemeChanges(t *testing.T) {
	defer SetTokenTheme(nil)
	h := New("a.go")
	line := "package main"
	before := styleAt(h.HighlightLine(line), 0)

	SetTokenTheme(NewTokenTheme([]TokenRule{{Selector: "keyword", Foreground: "#abcdef"}}))
	withTheme := styleAt(h.HighlightLine(line), 0)
	if withTheme < term.TokenStyle(0) || CurrentTokenTheme().Styles[withTheme-term.TokenStyle(0)].Fg != "#abcdef" {
		t.Fatalf("keyword style with token theme = %v, want the #abcdef slot", withTheme)
	}
	if got := styleAt(h.HighlightLine(line), 8); got != term.StyleDefault {
		t.Errorf("unmatched token = %v, want default", got)
	}

	SetTokenTheme(nil)
	if got := styleAt(h.HighlightLine(line), 0); got != before {
		t.Errorf("after removing the token theme = %v, want %v", got, before)
	}
}
