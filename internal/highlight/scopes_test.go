package highlight

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

// Issue #670: attributes of a tag split over several lines keep their colors.
func TestMultilineTagAttributes(t *testing.T) {
	for _, tc := range []struct {
		file  string
		lines []string
	}{
		{"index.html", []string{"<button", `  type="button"`, `  class="a"`, ">"}},
		{"view.tsx", []string{"<Button", `  type="button"`, `  className="a"`, "/>"}},
	} {
		h := New(tc.file)
		for _, i := range []int{1, 2} {
			if got := styleAt(h.HighlightLineAt(tc.lines, i), 2); got != term.StyleSyntaxAttribute {
				t.Errorf("%s line %d attribute = %v, want attribute", tc.file, i, got)
			}
		}
	}
}

// A multi-line JSX handler is code, not more attributes.
func TestMultilineJSXHandlerIsCode(t *testing.T) {
	lines := []string{"<Button", "  onClick={() => {", "    const next = count + 1;", "  }}", "/>"}
	h := New("view.tsx")
	if got := styleAt(h.HighlightLineAt(lines, 2), 4); got != term.StyleSyntaxStorage {
		t.Errorf("const inside handler = %v, want storage keyword", got)
	}
}

func TestScopeRefinedStyles(t *testing.T) {
	h := New("a.ts")
	line := `if (this.x) { return foo(p, "a\n", true); }`
	for col, want := range map[int]term.Style{
		0:  term.StyleSyntaxControl,
		4:  term.StyleSyntaxSelf,
		9:  term.StyleSyntaxProperty,
		14: term.StyleSyntaxControl,
		31: term.StyleSyntaxEscape,
		36: term.StyleSyntaxConstant,
	} {
		if got := styleAt(h.HighlightLine(line), col); got != want {
			t.Errorf("rune %d of %q = %v, want %v", col, line, got, want)
		}
	}
	for text, col := range map[string]int{
		"function f(param) {}":    11,
		"const s = `a${b}`;":      12,
		"const r = /ab+/g;":       11,
		"@Component() class A {}": 0,
	} {
		want := map[string]term.Style{
			"function f(param) {}":    term.StyleSyntaxParameter,
			"const s = `a${b}`;":      term.StyleSyntaxInterpolation,
			"const r = /ab+/g;":       term.StyleSyntaxRegexp,
			"@Component() class A {}": term.StyleSyntaxDecorator,
		}[text]
		if got := styleAt(h.HighlightLine(text), col); got != want {
			t.Errorf("rune %d of %q = %v, want %v", col, text, got, want)
		}
	}
}

// Go scopes an import path as entity.name.import inside a string. No VS Code
// theme colors that scope, so the path must match its quotes.
func TestGoImportPathMatchesItsQuotes(t *testing.T) {
	lines := []string{"import (", `    "runtime/debug"`, ")"}
	h := New("main.go")
	spans := h.HighlightLineAt(lines, 1)
	for col := 4; col < len(lines[1]); col++ {
		if got := styleAt(spans, col); got != term.StyleSyntaxString {
			t.Fatalf("rune %d of %q = %v, want string", col, lines[1], got)
		}
	}
	single := []string{`import "fmt"`}
	for col := 7; col < len(single[0]); col++ {
		if got := styleAt(h.HighlightLineAt(single, 0), col); got != term.StyleSyntaxString {
			t.Fatalf("rune %d of %q = %v, want string", col, single[0], got)
		}
	}
}

// VS Code themes color CSS selectors apart from HTML tags and attributes.
func TestCSSSelectorsUseSelectorStyle(t *testing.T) {
	for _, tc := range []struct {
		file  string
		lines []string
		line  int
		cols  []int
	}{
		{"a.css", []string{"a, .card #main:hover {", "}"}, 0, []int{0, 3, 9, 14}},
		{"a.scss", []string{".card {", "  a,", "  button {", "  }", "}"}, 1, []int{2}},
		{"a.less", []string{"button {", "}"}, 0, []int{0}},
	} {
		h := New(tc.file)
		spans := h.HighlightLineAt(tc.lines, tc.line)
		for _, col := range tc.cols {
			if got := styleAt(spans, col); got != term.StyleSyntaxSelector {
				t.Errorf("%s: rune %d of %q = %v, want selector", tc.file, col, tc.lines[tc.line], got)
			}
		}
	}
	html := New("index.html")
	lines := []string{`<div id="main" class="card">`, "<style>", "a { color: red; }", "</style>"}
	if got := styleAt(html.HighlightLineAt(lines, 0), 1); got != term.StyleSyntaxTag {
		t.Errorf("HTML tag = %v, want tag", got)
	}
	if got := styleAt(html.HighlightLineAt(lines, 0), 5); got != term.StyleSyntaxAttribute {
		t.Errorf("HTML id attribute = %v, want attribute", got)
	}
	if got := styleAt(html.HighlightLineAt(lines, 2), 0); got != term.StyleSyntaxSelector {
		t.Errorf("selector inside <style> = %v, want selector", got)
	}
}

func TestConstNamesUseReadonlyVariableStyle(t *testing.T) {
	h := New("a.ts")
	line := "const navigate = useNavigate();"
	if got := styleAt(h.HighlightLine(line), 6); got != term.StyleSyntaxReadonlyVariable {
		t.Errorf("const name = %v, want readonly variable", got)
	}
	if got := styleAt(h.HighlightLine("let count = 0;"), 4); got == term.StyleSyntaxReadonlyVariable {
		t.Error("let name should not use the readonly variable style")
	}
}
