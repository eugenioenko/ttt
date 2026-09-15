package highlight

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func allStyled(t *testing.T, spans []Span, line string, want term.Style, ctx string) {
	t.Helper()
	for i := range []rune(line) {
		if got := styleAt(spans, i); got != want {
			t.Errorf("%s: rune %d of %q = %v, want %v", ctx, i, line, got, want)
			return
		}
	}
}

func TestTemplateLiteralSpansLines(t *testing.T) {
	lines := []string{
		"const sql = `",
		"  SELECT * FROM todos",
		"  WHERE id = 3",
		"`;",
		"const after = 1;",
	}
	h := New("query.js")
	for _, i := range []int{1, 2} {
		allStyled(t, h.HighlightLineAt(lines, i), lines[i], term.StyleSyntaxString, "inside template")
	}
	if got := styleAt(h.HighlightLineAt(lines, 3), 0); got != term.StyleSyntaxString {
		t.Errorf("closing backtick = %v, want string", got)
	}
	if got := styleAt(h.HighlightLineAt(lines, 4), 0); got != term.StyleSyntaxKeyword {
		t.Errorf("line after the template = %v, want keyword", got)
	}
}

// A template that opens and closes on one line must not leak state: the
// appended closer used to detect an open region is itself an opener.
func TestClosedTemplateDoesNotOpenRegion(t *testing.T) {
	lines := []string{"const a = `x`;", "const b = 2;"}
	h := New("test.js")
	if got := styleAt(h.HighlightLineAt(lines, 1), 0); got != term.StyleSyntaxKeyword {
		t.Errorf("line after a closed template = %v, want keyword", got)
	}
}

func TestGoRawStringSpansLines(t *testing.T) {
	lines := []string{"var q = `", "  select 1", "`", "var n = 2"}
	h := New("main.go")
	allStyled(t, h.HighlightLineAt(lines, 1), lines[1], term.StyleSyntaxString, "inside raw string")
	if got := styleAt(h.HighlightLineAt(lines, 3), 0); got != term.StyleSyntaxKeyword {
		t.Errorf("line after the raw string = %v, want keyword", got)
	}
}

func TestPythonDocstringSpansLines(t *testing.T) {
	lines := []string{`def f():`, `    """`, `    docs`, `    """`, `    return 1`}
	h := New("mod.py")
	allStyled(t, h.HighlightLineAt(lines, 2), lines[2], term.StyleSyntaxString, "inside docstring")
	if got := styleAt(h.HighlightLineAt(lines, 4), 4); got != term.StyleSyntaxKeyword {
		t.Errorf("line after the docstring = %v, want keyword", got)
	}
}

// Python has two docstring delimiters and the table carried only one: the
// ”' entry was a mistyped Go literal that compiled, leaving its closer empty.
func TestPythonSingleQuoteDocstringSpansLines(t *testing.T) {
	lines := []string{`def f():`, `    '''`, `    docs`, `    '''`, `    return 1`}
	h := New("mod.py")
	allStyled(t, h.HighlightLineAt(lines, 2), lines[2], term.StyleSyntaxString, "inside docstring")
	if got := styleAt(h.HighlightLineAt(lines, 4), 4); got != term.StyleSyntaxKeyword {
		t.Errorf("line after the docstring = %v, want keyword", got)
	}
}

// An empty delimiter never closes, so its region would swallow the rest of
// the buffer. Go accepts the literal that causes it, so assert the table.
func TestRegionCandidatesHaveBothDelimiters(t *testing.T) {
	for _, c := range regionCandidates {
		if c.open == "" || c.close == "" {
			t.Errorf("candidate %q..%q has an empty delimiter", c.open, c.close)
		}
	}
}

// Chroma coalesces adjacent strings, so `""" a\nb """` reads as one token in
// languages that have no triple-quoted literal. Those must not gain a region.
func TestLanguagesWithoutTripleQuoteRegion(t *testing.T) {
	for _, file := range []string{"main.go", "lib.rs", "app.js"} {
		h := New(file)
		for _, r := range h.regions {
			if r.open == `"""` || r.open == "'''" {
				t.Errorf("%s: unexpected %q region", file, r.open)
			}
		}
	}
}

// Markdown's inline code is not a multi-line region; treating it as one would
// gray out half a document after a stray backtick.
func TestMarkdownHasNoStringRegion(t *testing.T) {
	h := New("notes.md")
	for _, r := range h.regions {
		if r.style == term.StyleSyntaxString {
			t.Errorf("markdown: unexpected string region %q", r.open)
		}
	}
}

// A backslash escapes the closer inside a string region, but not inside a
// comment, where nothing is special.
func TestEscapedCloserInsideStringRegion(t *testing.T) {
	str := region{open: "`", close: "`", style: term.StyleSyntaxString, escapes: true}
	if got := closesAt("a \\` b` rest", str); got != 7 {
		t.Errorf("escaped closer: got %d, want 7", got)
	}
	comment := region{open: "/*", close: "*/", style: term.StyleSyntaxComment}
	if got := closesAt("a \\*/ rest", comment); got != 5 {
		t.Errorf("comment closer: got %d, want 5", got)
	}
}

// Two region kinds in one language: whichever opens first on the line wins.
func TestEarliestRegionWins(t *testing.T) {
	lines := []string{"const a = `x /* y", "still string", "`;"}
	h := New("test.js")
	allStyled(t, h.HighlightLineAt(lines, 1), lines[1], term.StyleSyntaxString, "template beats comment")
}

// A region that closes at the very end of its line: the closer appended to
// probe for an open region coalesces with the closer already there, so the
// lexer reports a region starting before end of line and the state leaked
// into the next one. TestClosedTemplateDoesNotOpenRegion misses this because
// its trailing ";" breaks the coalescing.
func TestRegionClosedAtEndOfLineDoesNotLeak(t *testing.T) {
	for _, tc := range []struct {
		file  string
		lines []string
		want  term.Style
	}{
		{"a.js", []string{"const a = `x`", "const b = 2;"}, term.StyleSyntaxKeyword},
		{"a.go", []string{"q := `select 1`", "func main() {}"}, term.StyleSyntaxKeyword},
		{"a.py", []string{`x = """abc"""`, `def f(): pass`}, term.StyleSyntaxKeyword},
		{"a.py", []string{`x = '''abc'''`, `def f(): pass`}, term.StyleSyntaxKeyword},
		{"a.lua", []string{"x = --[[ a ]]", "local y = 2"}, term.StyleSyntaxKeyword},
	} {
		h := New(tc.file)
		if got := styleAt(h.HighlightLineAt(tc.lines, 1), 0); got != tc.want {
			t.Errorf("%s: line after %q = %v, want %v", tc.file, tc.lines[0], got, tc.want)
		}
	}
}

// A half-written delimiter must not become a whole one: the probe appends the
// closer, so a line ending in "" would otherwise open a Python docstring.
func TestAppendedCloserCannotCompleteAnOpener(t *testing.T) {
	for _, tc := range []struct {
		file  string
		lines []string
	}{
		{"a.py", []string{`x = ""`, `def f(): pass`}},
		{"a.js", []string{"const a = ``", "const b = 2;"}},
		{"a.js", []string{"code(); /", "const b = 2;"}},
	} {
		h := New(tc.file)
		if got := styleAt(h.HighlightLineAt(tc.lines, 1), 0); got != term.StyleSyntaxKeyword {
			t.Errorf("%s: line after %q = %v, want keyword", tc.file, tc.lines[0], got)
		}
	}
}

// Rejecting a line because it closes a region must not reject the line: a
// later opener on the same line still carries.
func TestRegionReopenedAfterClosingOnSameLine(t *testing.T) {
	for _, tc := range []struct {
		file  string
		lines []string
	}{
		{"a.js", []string{"const a = `x`; const b = `y", "still string"}},
		{"a.js", []string{"/*a*/ /*b*/ /*c", "still comment"}},
		{"a.py", []string{`s = '''a''' + """b`, `still string`}},
	} {
		h := New(tc.file)
		want := term.StyleSyntaxString
		if tc.lines[0] == "/*a*/ /*b*/ /*c" {
			want = term.StyleSyntaxComment
		}
		allStyled(t, h.HighlightLineAt(tc.lines, 1), tc.lines[1], want, tc.file+" reopened region")
	}
}

// The triple-quote probe is conservative by design: coalescing makes a run of
// short literals look like one region, so a language only gains the region
// when its lexer emits the delimiter as its own token. This is the matrix the
// PR body claims.
func TestTripleQuoteRegionLanguageMatrix(t *testing.T) {
	for file, want := range map[string]bool{
		"a.py":    true,
		"a.kt":    true,
		"a.swift": false,
		"a.scala": false,
		"a.java":  false,
		"a.go":    false,
		"a.rs":    false,
	} {
		h := New(file)
		if h == nil {
			t.Fatalf("no lexer for %s", file)
		}
		got := false
		for _, r := range h.regions {
			if r.open == `"""` {
				got = true
			}
		}
		if got != want {
			t.Errorf(`%s: """ region = %v, want %v`, file, got, want)
		}
	}
}

// Go raw strings have no escapes, so a backslash before the closing backtick
// must not swallow it. Guessing left the region open for the whole buffer.
func TestGoRawStringCloserIsNotEscaped(t *testing.T) {
	lines := []string{"const help = `usage:", `  ttt -d C:\` + "`", "func main() {}"}
	h := New("main.go")
	if got := styleAt(h.HighlightLineAt(lines, 2), 0); got != term.StyleSyntaxKeyword {
		t.Errorf("line after a raw string ending in a backslash = %v, want keyword", got)
	}
}

// JavaScript template literals do have escapes, which is the same probe
// answering the other way.
func TestTemplateLiteralCloserIsEscaped(t *testing.T) {
	h := New("app.js")
	for _, r := range h.regions {
		if r.open == "`" && !r.escapes {
			t.Error("template literal region: escapes = false, want true")
		}
	}
}

// A language has one block comment. Collecting every matching candidate gave
// TypeScript an "<!-- -->" region, and `a <!--b` then commented out the rest
// of the buffer.
func TestOneBlockCommentPerLanguage(t *testing.T) {
	lines := []string{"if (a <!--b) { c() }", "const d = 2"}
	h := New("app.ts")
	comments := 0
	for _, r := range h.regions {
		if r.style == term.StyleSyntaxComment {
			comments++
		}
	}
	if comments != 1 {
		t.Errorf("typescript: %d comment regions, want 1", comments)
	}
	if got := styleAt(h.HighlightLineAt(lines, 1), 0); got != term.StyleSyntaxKeyword {
		t.Errorf("line after `a <!--b` = %v, want keyword", got)
	}
}
