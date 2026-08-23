package markdown

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func textLines(lines []Line) []string {
	var result []string
	for _, l := range lines {
		result = append(result, l.Text())
	}
	return result
}

func TestRenderPlainText(t *testing.T) {
	lines := Render("hello world")
	if lines[0].Text() != "hello world" {
		t.Errorf("expected 'hello world', got %q", lines[0].Text())
	}
}

func TestRenderInlineCode(t *testing.T) {
	lines := Render("use `fmt.Println` here")
	if lines[0].Text() != "use fmt.Println here" {
		t.Errorf("expected 'use fmt.Println here', got %q", lines[0].Text())
	}
	found := false
	for _, s := range lines[0].Spans {
		if s.Text == "fmt.Println" && s.Style == term.StyleHoverCode {
			found = true
		}
	}
	if !found {
		t.Error("expected inline code span with StyleHoverCode")
	}
}

func TestRenderBold(t *testing.T) {
	lines := Render("this is **bold** text")
	if lines[0].Text() != "this is bold text" {
		t.Errorf("expected 'this is bold text', got %q", lines[0].Text())
	}
	found := false
	for _, s := range lines[0].Spans {
		if s.Text == "bold" && s.Style == term.StyleHoverBold {
			found = true
		}
	}
	if !found {
		t.Error("expected bold span with StyleHoverBold")
	}
}

func TestRenderLink(t *testing.T) {
	lines := Render("see [docs](https://example.com) here")
	if lines[0].Text() != "see docs here" {
		t.Errorf("expected 'see docs here', got %q", lines[0].Text())
	}
}

func TestRenderCodeBlock(t *testing.T) {
	input := "before\n\n```go\nfunc main() {}\n```\n\nafter"
	lines := Render(input)
	tl := textLines(lines)
	found := false
	for _, l := range tl {
		if l == "func main() {}" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected code block line 'func main() {}', got %v", tl)
	}
}

func TestRenderCodeBlockHighlightAndFallbackStyles(t *testing.T) {
	known := renderCodeBlock([]string{"func main() {}"}, "go")
	if got := styleForText(known[0], "func"); got != term.StyleSyntaxKeyword {
		t.Fatalf("known Go fence func style = %v, want syntax keyword", got)
	}
	if got := styleForText(known[0], "main"); got != term.StyleSyntaxFunction {
		t.Fatalf("known Go fence main style = %v, want syntax function", got)
	}
	if got := styleForText(known[0], " "); got != term.StyleHoverCode {
		t.Fatalf("known Go fence unmapped gap style = %v, want hover code fallback", got)
	}
	renderedKnown := lineForText(t, Render("```go\nfunc main() {}\n```"), "func main() {}")
	if got := styleForText(renderedKnown, "func"); got != term.StyleSyntaxKeyword {
		t.Fatalf("rendered Go fence func style = %v, want syntax keyword", got)
	}
	if got := styleForText(renderedKnown, "main"); got != term.StyleSyntaxFunction {
		t.Fatalf("rendered Go fence main style = %v, want syntax function", got)
	}

	unknown := renderCodeBlock([]string{"plain text"}, "unknown-ttt-language")
	if len(unknown[0].Spans) != 1 || unknown[0].Spans[0] != (Span{Text: "plain text", Style: term.StyleHoverCode}) {
		t.Fatalf("unknown fence spans = %#v, want one hover-code fallback span", unknown[0].Spans)
	}

	empty := renderCodeBlock([]string{""}, "go")
	if len(empty[0].Spans) != 1 || empty[0].Spans[0] != (Span{Text: "", Style: term.StyleHoverCode}) {
		t.Fatalf("empty known fence spans = %#v, want empty hover-code fallback span", empty[0].Spans)
	}
}

func TestRenderDiffFenceUsesDedicatedStyles(t *testing.T) {
	lines := renderCodeBlock([]string{"+added line", "-deleted line"}, "diff")
	if got := styleForText(lines[0], "+added line"); got != term.StyleDiffAdded {
		t.Fatalf("added diff fence style = %v, want diff added", got)
	}
	if got := styleForText(lines[1], "-deleted line"); got != term.StyleDiffDeleted {
		t.Fatalf("deleted diff fence style = %v, want diff deleted", got)
	}
	rendered := Render("```diff\n+added line\n-deleted line\n```")
	if got := styleForText(lineForText(t, rendered, "+added line"), "+added line"); got != term.StyleDiffAdded {
		t.Fatalf("rendered added diff fence style = %v, want diff added", got)
	}
	if got := styleForText(lineForText(t, rendered, "-deleted line"), "-deleted line"); got != term.StyleDiffDeleted {
		t.Fatalf("rendered deleted diff fence style = %v, want diff deleted", got)
	}
}

func TestRenderCodeBlockKeepsLineIsolatedLexerState(t *testing.T) {
	lines := renderCodeBlock([]string{"/* open", "inside", "*/"}, "go")
	if got := styleForText(lines[0], "/* open"); got != term.StyleSyntaxComment {
		t.Fatalf("opener style = %v, want syntax comment", got)
	}
	if got := styleForText(lines[1], "inside"); got != term.StyleHoverCode {
		t.Fatalf("isolated interior style = %v, want hover code fallback", got)
	}
}

func styleForText(line Line, text string) term.Style {
	for _, span := range line.Spans {
		if span.Text == text || strings.Contains(span.Text, text) {
			return span.Style
		}
	}
	return term.StyleDefault
}

func lineForText(t *testing.T, lines []Line, text string) Line {
	t.Helper()
	for _, line := range lines {
		if line.Text() == text {
			return line
		}
	}
	t.Fatalf("rendered lines do not contain %q: %v", text, textLines(lines))
	return Line{}
}

func TestRenderDivider(t *testing.T) {
	lines := Render("above\n\n---\n\nbelow")
	foundDivider := false
	for _, l := range lines {
		if len(l.Spans) > 0 && l.Spans[0].Style == term.StyleBorder && l.Spans[0].Text == "---" {
			foundDivider = true
		}
	}
	if !foundDivider {
		t.Error("expected divider line with StyleBorder")
	}
}

func TestRenderMultilineCodeBlock(t *testing.T) {
	input := "```\nline1\nline2\nline3\n```"
	lines := Render(input)
	tl := textLines(lines)
	for _, expected := range []string{"line1", "line2", "line3"} {
		found := false
		for _, l := range tl {
			if l == expected {
				found = true
			}
		}
		if !found {
			t.Errorf("expected %q in output, got %v", expected, tl)
		}
	}
}

func TestRenderItalic(t *testing.T) {
	lines := Render("this is *italic* text")
	if lines[0].Text() != "this is italic text" {
		t.Errorf("expected 'this is italic text', got %q", lines[0].Text())
	}
	found := false
	for _, s := range lines[0].Spans {
		if s.Text == "italic" && s.Style == term.StyleHoverItalic {
			found = true
		}
	}
	if !found {
		t.Error("expected italic span with StyleHoverItalic")
	}
}

func TestRenderLinkWithInlineCode(t *testing.T) {
	lines := Render("[`string` on pkg.go.dev](https://pkg.go.dev/builtin#string)")
	text := lines[0].Text()
	if text != "string on pkg.go.dev" {
		t.Errorf("expected 'string on pkg.go.dev', got %q", text)
	}
	found := false
	for _, s := range lines[0].Spans {
		if s.Text == "string" && s.Style == term.StyleHoverCode {
			found = true
		}
	}
	if !found {
		t.Error("expected inline code span inside link")
	}
}

func TestRenderEmptyText(t *testing.T) {
	lines := Render("")
	if len(lines) == 0 {
		t.Fatal("expected at least 1 line")
	}
}

func TestWrapLineShortLine(t *testing.T) {
	line := Line{Spans: []Span{{Text: "hello", Style: term.StylePaletteItem}}}
	result := WrapLine(line, 60)
	if len(result) != 1 {
		t.Fatalf("expected 1 line, got %d", len(result))
	}
	if result[0].Text() != "hello" {
		t.Errorf("expected 'hello', got %q", result[0].Text())
	}
}

func TestWrapLineBreaksAtSpace(t *testing.T) {
	line := Line{Spans: []Span{{Text: "hello world foo", Style: term.StylePaletteItem}}}
	result := WrapLine(line, 12)
	if len(result) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(result))
	}
	var full string
	for _, l := range result {
		full += l.Text()
	}
	if full != "hello world foo" {
		t.Errorf("expected full text preserved, got %q", full)
	}
}

func TestWrapLinePreservesKind(t *testing.T) {
	line := Line{Kind: KindParagraph, Spans: []Span{{Text: "hello world foo", Style: term.StylePaletteItem}}}
	result := WrapLine(line, 8)
	for i, l := range result {
		if l.Kind != KindParagraph {
			t.Errorf("line %d: expected KindParagraph, got %d", i, l.Kind)
		}
	}
}

func TestWrapLinePreservesStyles(t *testing.T) {
	line := Line{Spans: []Span{
		{Text: "hello ", Style: term.StyleHoverBold},
		{Text: "world again", Style: term.StyleHoverCode},
	}}
	result := WrapLine(line, 8)
	if len(result) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(result))
	}
	var allText string
	for _, l := range result {
		allText += l.Text()
	}
	if allText != "hello world again" {
		t.Errorf("expected full text preserved, got %q", allText)
	}
}

func TestFlattenStyles(t *testing.T) {
	line := Line{Spans: []Span{
		{Text: "ab", Style: term.StyleHoverBold},
		{Text: "cd", Style: term.StyleHoverCode},
	}}
	styles := FlattenStyles(line)
	if len(styles) != 4 {
		t.Fatalf("expected 4 styles, got %d", len(styles))
	}
	if styles[0] != term.StyleHoverBold || styles[1] != term.StyleHoverBold {
		t.Error("first two styles should be StyleHoverBold")
	}
	if styles[2] != term.StyleHoverCode || styles[3] != term.StyleHoverCode {
		t.Error("last two styles should be StyleHoverCode")
	}
}

func TestKindWrappable(t *testing.T) {
	wrappable := []LineKind{KindParagraph, KindHeading, KindListItem, KindBlockquote}
	for _, k := range wrappable {
		if !k.Wrappable() {
			t.Errorf("expected %d to be wrappable", k)
		}
	}
	notWrappable := []LineKind{KindCode, KindTable, KindDivider, KindBlank}
	for _, k := range notWrappable {
		if k.Wrappable() {
			t.Errorf("expected %d to not be wrappable", k)
		}
	}
}

func TestLineText(t *testing.T) {
	line := Line{Spans: []Span{
		{Text: "hello", Style: term.StyleDefault},
		{Text: " ", Style: term.StyleDefault},
		{Text: "world", Style: term.StyleHoverBold},
	}}
	if line.Text() != "hello world" {
		t.Errorf("expected 'hello world', got %q", line.Text())
	}
}

func TestLineTextEmpty(t *testing.T) {
	line := Line{Spans: []Span{}}
	if line.Text() != "" {
		t.Errorf("expected empty string, got %q", line.Text())
	}
}

func TestRenderUnmatchedBacktick(t *testing.T) {
	lines := Render("unmatched ` backtick")
	if lines[0].Text() != "unmatched ` backtick" {
		t.Errorf("expected 'unmatched ` backtick', got %q", lines[0].Text())
	}
}

func TestRenderUnmatchedBold(t *testing.T) {
	lines := Render("unmatched ** bold")
	text := lines[0].Text()
	if text != "unmatched ** bold" {
		t.Errorf("expected literal **, got %q", text)
	}
}

func TestRenderUnclosedCodeBlock(t *testing.T) {
	lines := Render("```\ncode line\nno close")
	tl := textLines(lines)
	found := false
	for _, l := range tl {
		if l == "code line" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'code line' in output, got %v", tl)
	}
}

func TestRenderParagraphJoining(t *testing.T) {
	input := "Install this plugin and Prettier will\nbe automatically configured for all\nsupported file types."
	lines := Render(input)
	joined := lines[0].Text()
	expected := "Install this plugin and Prettier will be automatically configured for all supported file types."
	if joined != expected {
		t.Errorf("expected joined paragraph, got %q", joined)
	}
}

func TestRenderTable(t *testing.T) {
	input := "| A | B |\n|---|---|\n| 1 | 2 |"
	lines := Render(input)
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines for table, got %d", len(lines))
	}
	tableLines := 0
	for _, l := range lines {
		if l.Kind == KindTable {
			tableLines++
		}
	}
	if tableLines < 3 {
		t.Errorf("expected at least 3 KindTable lines, got %d", tableLines)
	}
	foundSep := false
	for _, l := range lines {
		if strings.Contains(l.Text(), "─") {
			foundSep = true
		}
	}
	if !foundSep {
		t.Errorf("expected separator row with ─")
	}
}

func TestRenderList(t *testing.T) {
	input := "- item 1\n- item 2\n- item 3"
	lines := Render(input)
	tl := textLines(lines)
	found := 0
	for _, l := range tl {
		if l == "• item 1" || l == "• item 2" || l == "• item 3" {
			found++
		}
	}
	if found != 3 {
		t.Errorf("expected 3 list items with bullet, got %d: %v", found, tl)
	}
}

func TestRenderHeading(t *testing.T) {
	lines := Render("# Hello World")
	if lines[0].Text() != "# Hello World" {
		t.Errorf("expected '# Hello World', got %q", lines[0].Text())
	}
	found := false
	for _, s := range lines[0].Spans {
		if s.Style == term.StyleHoverBold {
			found = true
		}
	}
	if !found {
		t.Error("expected heading to use StyleHoverBold")
	}
}

func TestRenderCodeBlockKind(t *testing.T) {
	lines := Render("```go\nfunc main() {}\n```")
	found := false
	for _, l := range lines {
		if l.Kind == KindCode && strings.Contains(l.Text(), "func main") {
			found = true
		}
	}
	if !found {
		t.Error("expected code block line with KindCode")
	}
}

func TestRenderBlockquote(t *testing.T) {
	lines := Render("> this is a quote")
	found := false
	for _, l := range lines {
		if l.Kind == KindBlockquote && strings.Contains(l.Text(), "this is a quote") {
			found = true
		}
	}
	if !found {
		t.Error("expected blockquote line with KindBlockquote")
	}
	hasBorder := false
	for _, l := range lines {
		for _, s := range l.Spans {
			if s.Text == "│ " && s.Style == term.StyleBorder {
				hasBorder = true
			}
		}
	}
	if !hasBorder {
		t.Error("expected blockquote prefix with StyleBorder")
	}
}
