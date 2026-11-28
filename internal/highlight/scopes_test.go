package highlight

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
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

// JSXMarkupAt must follow the line's leading context: markup lines need the
// JSX comment form, while JS code inside an expression does not.
func TestJSXMarkupAt(t *testing.T) {
	lines := []string{
		"export function App() {",
		"  return (",
		"    <div className=\"wrap\">",
		"      <h1>Hello</h1>",
		"      {items.map((item) => (",
		"        // a js comment",
		"        <li key={item.id}>{item}</li>",
		"      ))}",
		"      {cond && <span>yes</span>}",
		"      {/* existing comment */}",
		"    </div>",
		"  );",
		"}",
	}
	want := []bool{false, false, true, true, true, false, true, false, true, true, true, false, false}
	h := New("view.tsx")
	buf := &buffer.Buffer{Lines: lines}
	for i, w := range want {
		if got := h.JSXMarkupAt(buf, i); got != w {
			t.Errorf("line %d (%q) markup = %v, want %v", i, lines[i], got, w)
		}
	}
}

func TestSvelteContextAt(t *testing.T) {
	lines := []string{
		"<script>",
		"  let x = 1;",
		"</script>",
		"<div>",
		"  {items.map((item) => (",
		"    item.name",
		"  ))}",
		"</div>",
		"<style>",
		"  .a { color: red; }",
		"</style>",
		"plain text",
	}
	want := []SvelteContext{
		SvelteMarkup, SvelteScript, SvelteMarkup, SvelteMarkup,
		SvelteMarkup, SvelteExpression, SvelteExpression, SvelteMarkup,
		SvelteMarkup, SvelteStyle, SvelteMarkup, SvelteMarkup,
	}
	h := New("view.svelte")
	buf := &buffer.Buffer{Lines: lines}
	for i, w := range want {
		if got := h.SvelteContextAt(buf, i); got != w {
			t.Errorf("line %d (%q) context = %v, want %v", i, lines[i], got, w)
		}
	}
}

func TestTwigContextAt(t *testing.T) {
	lines := []string{
		"<div class=\"a\">",
		"  {{ name }}",
		"  {# existing #}",
		"</div>",
		"<script>",
		"  let x = 1;",
		"</script>",
		"<style>",
		"  .a { color: red; }",
		"</style>",
		"{% verbatim %}",
		"  <p>{# literal #}</p>",
		"{% endverbatim %}",
	}
	want := []TwigContext{
		TwigMarkup, TwigMarkup, TwigMarkup, TwigMarkup,
		TwigMarkup, TwigScript, TwigMarkup,
		TwigMarkup, TwigStyle, TwigMarkup,
		TwigMarkup, TwigVerbatim, TwigMarkup,
	}
	h := New("view.twig")
	buf := &buffer.Buffer{Lines: lines}
	for i, w := range want {
		if got := h.TwigContextAt(buf, i); got != w {
			t.Errorf("line %d (%q) context = %v, want %v", i, lines[i], got, w)
		}
	}
}

func TestVueContextAt(t *testing.T) {
	lines := []string{
		"<template>",
		"  <div class=\"a\">",
		"    {{ name }}",
		"    <p>text</p>",
		"  </div>",
		"</template>",
		"<script setup lang=\"ts\">",
		"  const x = 1;",
		"</script>",
		"<style scoped>",
		"  .a { color: red; }",
		"  body { margin: 0; }",
		"</style>",
	}
	want := []VueContext{
		VueMarkup, VueMarkup, VueMarkup, VueMarkup, VueMarkup,
		VueMarkup, VueMarkup, VueScript, VueMarkup, VueMarkup, VueStyle, VueStyle, VueMarkup,
	}
	h := New("view.vue")
	buf := &buffer.Buffer{Lines: lines}
	for i, w := range want {
		if got := h.VueContextAt(buf, i); got != w {
			t.Errorf("line %d (%q) context = %v, want %v", i, lines[i], got, w)
		}
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
