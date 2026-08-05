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

func assertLineHasStyle(t *testing.T, spans []Span, style term.Style) {
	t.Helper()
	for _, s := range spans {
		if s.Style == style {
			return
		}
	}
	t.Errorf("expected style %v in spans %v", style, spans)
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

func TestHighlightLineWithState_MultilineComment(t *testing.T) {
	h := New("test.js")
	if h == nil {
		t.Fatal("expected highlighter for .js files")
	}

	// Line 1: opens a multiline comment
	spans1, inComment := h.HighlightLineWithState("/*", false)
	if !inComment {
		t.Error("line 1 should end inside block comment")
	}
	assertLineHasStyle(t, spans1, term.StyleSyntaxComment)

	// Line 2: inside the comment — should be all Comment
	spans2, inComment := h.HighlightLineWithState("   this is a comment", inComment)
	if !inComment {
		t.Error("line 2 should still be inside block comment")
	}
	assertLineHasStyle(t, spans2, term.StyleSyntaxComment)

	// Line 3: closes the comment
	spans3, inComment := h.HighlightLineWithState("*/", inComment)
	if inComment {
		t.Error("line 3 should end outside block comment")
	}
	assertLineHasStyle(t, spans3, term.StyleSyntaxComment)

	// Line 4: normal code after comment
	spans4, inComment := h.HighlightLineWithState("var x = 1;", inComment)
	if inComment {
		t.Error("line 4 should not be inside block comment")
	}
	for _, s := range spans4 {
		if s.Style == term.StyleSyntaxComment {
			t.Error("line 4 should not have comment spans")
		}
	}
}

func TestHighlightLineWithState_InlineComment(t *testing.T) {
	h := New("test.js")
	if h == nil {
		t.Fatal("expected highlighter for .js files")
	}

	// Inline block comment — chroma handles this, should NOT set state
	spans, inComment := h.HighlightLineWithState("/* inline */ var x = 1;", false)
	if inComment {
		t.Error("inline comment should not leave state open")
	}
	assertLineHasStyle(t, spans, term.StyleSyntaxComment)
}

func TestHighlightLineWithState_CommentThenCode(t *testing.T) {
	h := New("test.js")
	if h == nil {
		t.Fatal("expected highlighter for .js files")
	}

	// */ with code after it — should highlight comment portion and then code
	spans, inComment := h.HighlightLineWithState("*/ var x = 1;", true)
	if inComment {
		t.Error("should exit comment state when */ is found")
	}
	assertLineHasStyle(t, spans, term.StyleSyntaxComment)
}

func TestScanBlockCommentState(t *testing.T) {
	lines := []string{
		"import { foo } from 'bar';",
		"// line comment",
		"/* start of multiline",
		"   middle",
		"*/ end",
		"normal code",
	}

	if ScanBlockCommentState(lines, 0) {
		t.Error("line 0 should not be in comment")
	}
	if ScanBlockCommentState(lines, 2) {
		t.Error("line 2 should not be in comment (starts on this line)")
	}
	if !ScanBlockCommentState(lines, 3) {
		t.Error("line 3 should be in comment")
	}
	if !ScanBlockCommentState(lines, 4) {
		t.Error("line 4 should be in comment")
	}
	if ScanBlockCommentState(lines, 5) {
		t.Error("line 5 should not be in comment")
	}
}
