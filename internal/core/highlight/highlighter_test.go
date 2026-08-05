package highlight

import (
	"github.com/eugenioenko/ttt/internal/term"
	"testing"
)

func TestHighlightGo_Comment(t *testing.T) {
	h := New("main.go")
	if h == nil {
		t.Fatal("expected highlighter for .go files")
	}
	spans := h.HighlightLine("x := 1 // comment")
	found := false
	for _, s := range spans {
		if s.Style == term.StyleSyntaxComment {
			found = true
		}
	}
	if !found {
		t.Error("expected comment span")
	}
}

func TestHighlightGo_String(t *testing.T) {
	h := New("main.go")
	spans := h.HighlightLine(`s := "hello"`)
	found := false
	for _, s := range spans {
		if s.Style == term.StyleSyntaxString {
			found = true
		}
	}
	if !found {
		t.Error("expected string span")
	}
}

func TestHighlightGo_Keyword(t *testing.T) {
	h := New("main.go")
	spans := h.HighlightLine("func main() {}")
	found := false
	for _, s := range spans {
		if s.Style == term.StyleSyntaxKeyword {
			found = true
		}
	}
	if !found {
		t.Error("expected keyword span")
	}
}

func TestHighlightGo_Function(t *testing.T) {
	h := New("main.go")
	spans := h.HighlightLine("func main() {}")
	found := false
	for _, s := range spans {
		if s.Style == term.StyleSyntaxFunction {
			found = true
		}
	}
	if !found {
		t.Error("expected function span")
	}
}

func TestHighlightUnknownFile(t *testing.T) {
	h := New("file.xyz123")
	if h != nil {
		t.Error("expected nil highlighter for unknown extension")
	}
}

func assertSpanStyle(t *testing.T, h *Highlighter, line string, style term.Style) {
	t.Helper()
	for _, s := range h.HighlightLine(line) {
		if s.Style == style {
			return
		}
	}
	t.Errorf("expected style %v in spans for %q", style, line)
}

func TestHighlightMarkdown(t *testing.T) {
	h := New("README.md")
	if h == nil {
		t.Fatal("expected highlighter for .md files")
	}
	assertSpanStyle(t, h, "# Heading", term.StyleSyntaxKeyword)
	assertSpanStyle(t, h, "## Subheading", term.StyleSyntaxKeyword)
	assertSpanStyle(t, h, "some **bold** text", term.StyleSyntaxType)
	assertSpanStyle(t, h, "some *italic* text", term.StyleSyntaxString)
	assertSpanStyle(t, h, "inline `code` here", term.StyleSyntaxString)
}

func TestHighlightDiff(t *testing.T) {
	h := New("changes.diff")
	if h == nil {
		t.Fatal("expected highlighter for .diff files")
	}
	assertSpanStyle(t, h, "+added line", term.StyleDiffAdded)
	assertSpanStyle(t, h, "-removed line", term.StyleDiffDeleted)
}

func TestHighlightJSON(t *testing.T) {
	h := New("config.json")
	if h == nil {
		t.Fatal("expected highlighter for .json files")
	}
	spans := h.HighlightLine(`"key": "value"`)
	if len(spans) == 0 {
		t.Error("expected spans for JSON")
	}
}

// styleAt returns the style covering rune index col, or StyleDefault.
func styleAt(spans []Span, col int) term.Style {
	for _, s := range spans {
		if col >= s.Start && col < s.End {
			return s.Style
		}
	}
	return term.StyleDefault
}

func allComment(t *testing.T, spans []Span, line string, ctx string) {
	t.Helper()
	for i := range []rune(line) {
		if styleAt(spans, i) != term.StyleSyntaxComment {
			t.Errorf("%s: rune %d of %q not styled as comment", ctx, i, line)
			return
		}
	}
}

func TestMultilineBlockComment(t *testing.T) {
	lines := []string{
		"const a = 1;",
		"/*",
		"   still inside",
		"*/",
		"const b = 2;",
	}
	h := New("test.js")
	if h == nil {
		t.Fatal("expected highlighter for .js")
	}
	for _, i := range []int{1, 2, 3} {
		allComment(t, h.HighlightLineAt(lines, i), lines[i], "js")
	}
	// code after the comment closes is highlighted normally again
	if styleAt(h.HighlightLineAt(lines, 4), 0) != term.StyleSyntaxKeyword {
		t.Error("expected keyword on line after comment close")
	}
}

func TestMultilineBlockCommentGo(t *testing.T) {
	lines := []string{"package main", "/*", "doc", "*/", "func main() {}"}
	h := New("main.go")
	for _, i := range []int{1, 2, 3} {
		allComment(t, h.HighlightLineAt(lines, i), lines[i], "go")
	}
	if styleAt(h.HighlightLineAt(lines, 4), 0) != term.StyleSyntaxKeyword {
		t.Error("expected keyword after comment close")
	}
}

func TestOpenerInStringDoesNotStartComment(t *testing.T) {
	lines := []string{`const s = "/*";`, "const b = 2;"}
	h := New("test.js")
	if got := styleAt(h.HighlightLineAt(lines, 1), 0); got != term.StyleSyntaxKeyword {
		t.Errorf("line after string containing /* should be normal code, got %v", got)
	}
}

func TestOpenerInLineCommentDoesNotStartComment(t *testing.T) {
	lines := []string{"// note /*", "const b = 2;"}
	h := New("test.js")
	if got := styleAt(h.HighlightLineAt(lines, 1), 0); got != term.StyleSyntaxKeyword {
		t.Errorf("line after // containing /* should be normal code, got %v", got)
	}
}

func TestInlineBlockCommentDoesNotLeak(t *testing.T) {
	lines := []string{"/* inline */ const a = 1;", "const b = 2;"}
	h := New("test.js")
	if got := styleAt(h.HighlightLineAt(lines, 1), 0); got != term.StyleSyntaxKeyword {
		t.Errorf("closed inline comment must not leak to next line, got %v", got)
	}
}

func TestCodeBeforeAndAfterBlockComment(t *testing.T) {
	lines := []string{"const a = 1; /* open", "*/ const b = 2;"}
	h := New("test.js")
	first := h.HighlightLineAt(lines, 0)
	if styleAt(first, 0) != term.StyleSyntaxKeyword {
		t.Error("code before the opener should stay highlighted")
	}
	if styleAt(first, 13) != term.StyleSyntaxComment {
		t.Error("opener onwards should be comment")
	}
	second := h.HighlightLineAt(lines, 1)
	if styleAt(second, 0) != term.StyleSyntaxComment {
		t.Error("closer should be comment")
	}
	if styleAt(second, 3) != term.StyleSyntaxKeyword {
		t.Error("code after the closer should be highlighted")
	}
}

func TestUnclosedCommentRunsToEndOfBuffer(t *testing.T) {
	lines := []string{"code();", "/* never closed", "a", "b"}
	h := New("test.js")
	for _, i := range []int{2, 3} {
		allComment(t, h.HighlightLineAt(lines, i), lines[i], "unclosed")
	}
}

func TestHTMLBlockComment(t *testing.T) {
	lines := []string{"<p>hi</p>", "<!--", "hidden", "-->"}
	h := New("index.html")
	for _, i := range []int{1, 2, 3} {
		allComment(t, h.HighlightLineAt(lines, i), lines[i], "html")
	}
}

func TestNoBlockCommentLanguageUnaffected(t *testing.T) {
	h := New("script.py")
	if h.blockOpen != "" {
		t.Errorf("python should have no block comment delimiters, got %q", h.blockOpen)
	}
	lines := []string{"x = 1", "y = 2"}
	if styleAt(h.HighlightLineAt(lines, 1), 0) == term.StyleSyntaxComment {
		t.Error("python line should not be a comment")
	}
}

func TestDetectBlockCommentDelimiters(t *testing.T) {
	cases := []struct{ file, open, close string }{
		{"a.go", "/*", "*/"},
		{"a.js", "/*", "*/"},
		{"a.rs", "/*", "*/"},
		{"a.css", "/*", "*/"},
		{"a.html", "<!--", "-->"},
		{"a.hs", "{-", "-}"},
		{"a.py", "", ""},
		{"a.json", "", ""},
	}
	for _, c := range cases {
		h := New(c.file)
		if h == nil {
			t.Fatalf("no highlighter for %s", c.file)
		}
		if h.blockOpen != c.open || h.blockClose != c.close {
			t.Errorf("%s: got %q/%q want %q/%q", c.file, h.blockOpen, h.blockClose, c.open, c.close)
		}
	}
}

func TestStateSurvivesClearCache(t *testing.T) {
	lines := []string{"/*", "inside", "*/"}
	h := New("test.js")
	allComment(t, h.HighlightLineAt(lines, 1), lines[1], "before clear")
	h.ClearCache()
	allComment(t, h.HighlightLineAt(lines, 1), lines[1], "after clear")
}

// The span cache must not be corrupted by the truncation done when a comment
// opens mid-line: the same text is requested both with and without state.
func TestCacheNotAliasedAcrossStates(t *testing.T) {
	h := New("test.js")
	line := "const a = 1; /* open"
	plain := h.HighlightLineAt([]string{line}, 0)
	if styleAt(plain, 0) != term.StyleSyntaxKeyword {
		t.Fatal("expected keyword at start")
	}
	// request the same text again; a mutated cache entry would lose the keyword
	again := h.HighlightLineAt([]string{line}, 0)
	if styleAt(again, 0) != term.StyleSyntaxKeyword {
		t.Error("cached spans were mutated in place")
	}
}
