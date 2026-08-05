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

func TestHighlightAll_MultilineComment(t *testing.T) {
	h := New("test.js")
	if h == nil {
		t.Fatal("expected highlighter for .js files")
	}

	lines := []string{
		`import { foo } from "bar";`,
		`// single line comment`,
		`/* inline block */`,
		`/*`,
		`   multiline comment`,
		`*/`,
		`var x = 1;`,
	}

	allSpans := h.HighlightAll(lines)

	// Line 1: single-line comment should be Comment style
	assertLineHasStyle(t, allSpans[1], term.StyleSyntaxComment)

	// Line 2: inline block comment should be Comment style
	assertLineHasStyle(t, allSpans[2], term.StyleSyntaxComment)

	// Lines 3-5: multiline comment — all should be Comment style
	assertLineHasStyle(t, allSpans[3], term.StyleSyntaxComment)
	assertLineHasStyle(t, allSpans[4], term.StyleSyntaxComment)
	assertLineHasStyle(t, allSpans[5], term.StyleSyntaxComment)

	// Line 6: normal code, should NOT be comment
	for _, s := range allSpans[6] {
		if s.Style == term.StyleSyntaxComment {
			t.Errorf("line 6 should not have comment spans, got %v", s)
		}
	}

	// Verify that HighlightLine (the old per-line API) does NOT highlight
	// multiline comments correctly — this is the known limitation.
	spans := h.HighlightLine("   multiline comment")
	for _, s := range spans {
		if s.Style == term.StyleSyntaxComment {
			t.Error("HighlightLine should NOT highlight orphan comment text (no delimiters on line)")
		}
	}
}

func TestHighlightAll_CacheReuse(t *testing.T) {
	h := New("test.js")
	if h == nil {
		t.Fatal("expected highlighter for .js files")
	}

	lines := []string{"// comment", "code"}
	first := h.HighlightAll(lines)
	second := h.HighlightAll(lines)

	if len(first) != len(second) {
		t.Error("cached result should have same length")
	}

	h.ClearCache()
	third := h.HighlightAll(lines)
	if len(third) != len(first) {
		t.Error("result after ClearCache should have same length")
	}
}

func assertLineHasStyle(t *testing.T, spans []Span, style term.Style) {
	t.Helper()
	for _, s := range spans {
		if s.Style == style {
			return
		}
	}
	t.Errorf("expected style %v in spans, got %v", style, spans)
}
