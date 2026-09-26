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
