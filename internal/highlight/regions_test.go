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
	if got := styleAt(h.HighlightLineAt(lines, 4), 0); got != term.StyleSyntaxStorage {
		t.Errorf("line after the template = %v, want storage keyword", got)
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
	if got := styleAt(h.HighlightLineAt(lines, 4), 4); got != term.StyleSyntaxControl {
		t.Errorf("line after the docstring = %v, want control keyword", got)
	}
}

// Python has two docstring delimiters; both must close.
func TestPythonSingleQuoteDocstringSpansLines(t *testing.T) {
	lines := []string{`def f():`, `    '''`, `    docs`, `    '''`, `    return 1`}
	h := New("mod.py")
	allStyled(t, h.HighlightLineAt(lines, 2), lines[2], term.StyleSyntaxString, "inside docstring")
	if got := styleAt(h.HighlightLineAt(lines, 4), 4); got != term.StyleSyntaxControl {
		t.Errorf("line after the docstring = %v, want control keyword", got)
	}
}

// A half-written delimiter must not open a multi-line region: a line ending in
// "" is an empty string, not the start of a Python docstring.
func TestHalfDelimiterDoesNotOpenRegion(t *testing.T) {
	for _, tc := range []struct {
		file  string
		lines []string
	}{
		{"a.py", []string{`x = ""`, `def f(): pass`}},
		{"a.js", []string{"const a = ``", "const b = 2;"}},
		{"a.js", []string{"code(); /", "const b = 2;"}},
	} {
		h := New(tc.file)
		if got := styleAt(h.HighlightLineAt(tc.lines, 1), 0); got != term.StyleSyntaxStorage {
			t.Errorf("%s: line after %q = %v, want storage keyword", tc.file, tc.lines[0], got)
		}
	}
}
