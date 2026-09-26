package highlight

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
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
	assertSpanStyle(t, h, "# Heading", term.StyleSyntaxHeading)
	assertSpanStyle(t, h, "## Subheading", term.StyleSyntaxHeading)
	assertSpanStyle(t, h, "some **bold** text", term.StyleSyntaxBold)
	assertSpanStyle(t, h, "some *italic* text", term.StyleSyntaxItalic)
	assertSpanStyle(t, h, "inline `code` here", term.StyleSyntaxCode)
	assertSpanStyle(t, h, "[link](https://example.com)", term.StyleSyntaxLink)
}

func TestHighlightDiff(t *testing.T) {
	h := New("changes.diff")
	if h == nil {
		t.Fatal("expected highlighter for .diff files")
	}
	assertSpanStyle(t, h, "+added line", term.StyleSyntaxInserted)
	assertSpanStyle(t, h, "-removed line", term.StyleSyntaxDeleted)
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
	if styleAt(h.HighlightLineAt(lines, 4), 0) != term.StyleSyntaxStorage {
		t.Error("expected storage keyword on line after comment close")
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
	if got := styleAt(h.HighlightLineAt(lines, 1), 0); got != term.StyleSyntaxStorage {
		t.Errorf("line after string containing /* should be normal code, got %v", got)
	}
}

func TestOpenerInLineCommentDoesNotStartComment(t *testing.T) {
	lines := []string{"// note /*", "const b = 2;"}
	h := New("test.js")
	if got := styleAt(h.HighlightLineAt(lines, 1), 0); got != term.StyleSyntaxStorage {
		t.Errorf("line after // containing /* should be normal code, got %v", got)
	}
}

func TestInlineBlockCommentDoesNotLeak(t *testing.T) {
	lines := []string{"/* inline */ const a = 1;", "const b = 2;"}
	h := New("test.js")
	if got := styleAt(h.HighlightLineAt(lines, 1), 0); got != term.StyleSyntaxStorage {
		t.Errorf("closed inline comment must not leak to next line, got %v", got)
	}
}

func TestCodeBeforeAndAfterBlockComment(t *testing.T) {
	lines := []string{"const a = 1; /* open", "*/ const b = 2;"}
	h := New("test.js")
	first := h.HighlightLineAt(lines, 0)
	if styleAt(first, 0) != term.StyleSyntaxStorage {
		t.Error("code before the opener should stay highlighted")
	}
	if styleAt(first, 13) != term.StyleSyntaxComment {
		t.Error("opener onwards should be comment")
	}
	second := h.HighlightLineAt(lines, 1)
	if styleAt(second, 0) != term.StyleSyntaxComment {
		t.Error("closer should be comment")
	}
	if styleAt(second, 3) != term.StyleSyntaxStorage {
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
	if styleAt(plain, 0) != term.StyleSyntaxStorage {
		t.Fatal("expected storage keyword at start")
	}
	// request the same text again; a mutated cache entry would lose the keyword
	again := h.HighlightLineAt([]string{line}, 0)
	if styleAt(again, 0) != term.StyleSyntaxStorage {
		t.Error("cached spans were mutated in place")
	}
}

func TestInvalidationEditAboveViewport(t *testing.T) {
	lines := []string{"const a = 1;", "const b = 2;", "const c = 3;", "const d = 4;"}
	h := New("test.js")
	if styleAt(h.HighlightLineAt(lines, 3), 0) != term.StyleSyntaxStorage {
		t.Fatal("precondition: last line should be code")
	}
	// Edit line 0 to open a comment; everything below must become comment.
	lines[0] = "/* now open"
	h.ClearCache()
	allComment(t, h.HighlightLineAt(lines, 3), lines[3], "after opening edit")

	// Close it again; the tail must go back to code.
	lines[0] = "const a = 1;"
	h.ClearCache()
	if styleAt(h.HighlightLineAt(lines, 3), 0) != term.StyleSyntaxStorage {
		t.Error("closing the comment again should restore code styling")
	}
}

// A whole-buffer rewrite (format, sort, replace-all) touches lines far from the
// cursor. Content-based invalidation must still catch it.
func TestInvalidationWholeBufferRewrite(t *testing.T) {
	lines := []string{"const a = 1;", "const b = 2;", "const c = 3;"}
	h := New("test.js")
	h.HighlightLineAt(lines, 2)

	rewritten := []string{"/* header", "   rewritten", "*/"}
	copy(lines, rewritten)
	h.ClearCache()
	for i := range lines {
		allComment(t, h.HighlightLineAt(lines, i), lines[i], "whole-buffer rewrite")
	}
}

func TestInvalidationBufferShrinks(t *testing.T) {
	lines := []string{"/* open", "inside", "*/", "const a = 1;"}
	h := New("test.js")
	h.HighlightLineAt(lines, 3)

	// Delete the closer line.
	lines = []string{"/* open", "inside", "const a = 1;"}
	h.ClearCache()
	allComment(t, h.HighlightLineAt(lines, 2), lines[2], "after shrink")
}

func TestInvalidationBufferGrows(t *testing.T) {
	lines := []string{"const a = 1;", "const b = 2;"}
	h := New("test.js")
	h.HighlightLineAt(lines, 1)

	lines = append(lines, "/* open", "inside")
	h.ClearCache()
	allComment(t, h.HighlightLineAt(lines, 3), lines[3], "after growth")
}

func TestInvalidationUnchangedBufferKeepsStyling(t *testing.T) {
	lines := []string{"/* open", "inside", "*/", "const a = 1;"}
	h := New("test.js")
	h.HighlightLineAt(lines, 3)
	h.ClearCache()
	if styleAt(h.HighlightLineAt(lines, 3), 0) != term.StyleSyntaxStorage {
		t.Error("styling changed after a no-op edit")
	}
}

func TestHighlightRuneIndexedHalfOpenSpans(t *testing.T) {
	h := New("main.go")
	if h == nil {
		t.Fatal("expected Go highlighter")
	}

	for _, tc := range []struct {
		name   string
		prefix string
	}{
		{name: "multibyte", prefix: "é "},
		{name: "fullwidth", prefix: "界 "},
		{name: "combining", prefix: "e\u0301 "},
		{name: "mixed", prefix: "é界e\u0301 "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := tc.prefix + "func main() {}"
			start := len([]rune(tc.prefix))
			want := Span{Start: start, End: start + len([]rune("func")), Style: term.StyleSyntaxKeyword}
			spans := h.HighlightLine(line)
			if !containsSpan(spans, want) {
				t.Fatalf("spans for %q = %#v, want exact half-open rune span %#v", line, spans, want)
			}
			if styleAt(spans, want.End-1) != want.Style {
				t.Fatalf("last rune inside %#v lost keyword style", want)
			}
			if styleAt(spans, want.End) == want.Style {
				t.Fatalf("end rune %d must be outside half-open span %#v", want.End, want)
			}
		})
	}
}

func TestHighlightStringSpanCountsUnicodeRunes(t *testing.T) {
	h := New("main.go")
	line := "x := \"é界e\u0301\""
	want := Span{Start: 5, End: 11, Style: term.StyleSyntaxString}
	spans := h.HighlightLine(line)
	if !containsSpan(spans, want) {
		t.Fatalf("spans for %q = %#v, want %#v", line, spans, want)
	}
}

func TestHighlightEmptyInvalidAndDefaultGaps(t *testing.T) {
	h := New("main.go")
	if spans := h.HighlightLine(""); len(spans) != 0 {
		t.Fatalf("empty line spans = %#v, want none", spans)
	}
	lines := []string{"package main"}
	if spans := h.HighlightLineAt(lines, -1); spans != nil {
		t.Fatalf("negative line spans = %#v, want nil", spans)
	}
	if spans := h.HighlightLineAt(lines, len(lines)); spans != nil {
		t.Fatalf("past-end line spans = %#v, want nil", spans)
	}

	spans := h.HighlightLine(lines[0])
	if got := styleAt(spans, 0); got != term.StyleSyntaxKeyword {
		t.Fatalf("package style = %v, want keyword", got)
	}
	for _, span := range spans {
		if span.Style == term.StyleDefault {
			t.Fatalf("default token emitted as explicit span: %#v", span)
		}
	}
	for _, col := range []int{7} {
		if got := styleAt(spans, col); got != term.StyleDefault {
			t.Fatalf("default gap at rune %d = %v, want default", col, got)
		}
	}
}

func TestHighlightFilenameSelectionAndFallback(t *testing.T) {
	for _, tc := range []struct {
		filename string
		language string
	}{
		{filename: "main.go", language: "Go"},
		{filename: "/tmp/project/main.go", language: "Go"},
		{filename: "main.go.bak", language: "Go"},
		{filename: "Dockerfile", language: "Dockerfile"},
		{filename: "main.go.orig", language: "Go"},
		{filename: "Rakefile", language: "Ruby"},
		{filename: "Gemfile", language: "Ruby"},
	} {
		t.Run(tc.filename, func(t *testing.T) {
			h := New(tc.filename)
			if h == nil {
				t.Fatalf("New(%q) = nil, want %s", tc.filename, tc.language)
			}
			if got := h.Language(); got != tc.language {
				t.Fatalf("New(%q).Language() = %q, want %q", tc.filename, got, tc.language)
			}
		})
	}
	if h := New("file.unknown-ttt-language"); h != nil {
		t.Fatalf("unknown filename selected %q, want nil fallback", h.Language())
	}
}

func containsSpan(spans []Span, want Span) bool {
	for _, span := range spans {
		if span == want {
			return true
		}
	}
	return false
}
