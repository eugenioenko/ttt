package markdown

import (
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
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
	if got := styleForText(lines[0], "+added line"); got != term.StyleSyntaxInserted {
		t.Fatalf("added diff fence style = %v, want inserted", got)
	}
	if got := styleForText(lines[1], "-deleted line"); got != term.StyleSyntaxDeleted {
		t.Fatalf("deleted diff fence style = %v, want deleted", got)
	}
	rendered := Render("```diff\n+added line\n-deleted line\n```")
	if got := styleForText(lineForText(t, rendered, "+added line"), "+added line"); got != term.StyleSyntaxInserted {
		t.Fatalf("rendered added diff fence style = %v, want inserted", got)
	}
	if got := styleForText(lineForText(t, rendered, "-deleted line"), "-deleted line"); got != term.StyleSyntaxDeleted {
		t.Fatalf("rendered deleted diff fence style = %v, want deleted", got)
	}
}

func TestRenderCodeBlockCarriesStateAcrossLines(t *testing.T) {
	lines := renderCodeBlock([]string{"/* open", "inside", "*/"}, "go")
	for i, text := range []string{"/* open", "inside", "*/"} {
		if got := styleForText(lines[i], text); got != term.StyleSyntaxComment {
			t.Fatalf("line %d %q style = %v, want syntax comment", i, text, got)
		}
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
	want := []LineKind{KindParagraph, KindDivider, KindParagraph}
	if len(lines) != len(want) {
		t.Fatalf("expected %d lines, got %q", len(want), textLines(lines))
	}
	for i, kind := range want {
		if lines[i].Kind != kind {
			t.Errorf("line %d: expected kind %v, got %v (%q)", i, kind, lines[i].Kind, lines[i].Text())
		}
	}
	if lines[1].Spans[0].Style != term.StyleBorder || lines[1].Spans[0].Text != "---" {
		t.Error("expected divider line with StyleBorder")
	}
}

func TestRenderEndsWithoutBlankLine(t *testing.T) {
	for _, text := range []string{"hello", "# Title\n\nbody", "```go\nx := 1\n```", "---"} {
		lines := Render(text)
		if last := lines[len(lines)-1]; last.Kind == KindBlank {
			t.Errorf("Render(%q) ends with a blank line: %q", text, textLines(lines))
		}
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

func TestWrapLineWideRunes(t *testing.T) {
	line := Line{Kind: KindParagraph, Spans: []Span{
		{Text: "界界 ab ", Style: term.StyleHoverBold},
		{Text: "語語cd", Style: term.StyleHoverCode},
	}}
	for _, width := range []int{1, 2, 5, 8} {
		wrapped := WrapLine(line, width)
		var text string
		var styles []term.Style
		for _, row := range wrapped {
			text += row.Text()
			styles = append(styles, FlattenStyles(row)...)
			if w := textwidth.String(row.Text()); w > width && !(width == 1 && len([]rune(row.Text())) == 1) {
				t.Fatalf("width %d: row %q occupies %d columns", width, row.Text(), w)
			}
		}
		if text != line.Text() || !slices.Equal(styles, FlattenStyles(line)) {
			t.Fatalf("width %d: wrapping changed text or styles", width)
		}
	}
}

func unwrapRows(rows []Line) (string, []term.Style) {
	var text strings.Builder
	var styles []term.Style
	for _, row := range rows {
		runes := []rune(row.Text())
		text.WriteString(string(runes[row.PrefixRunes:]))
		styles = append(styles, FlattenStyles(row)[row.PrefixRunes:]...)
	}
	return text.String(), styles
}

func TestWrapLineRepeatsHang(t *testing.T) {
	bar := Span{Text: "│ ", Style: term.StyleBorder}
	line := Line{Kind: KindBlockquote, Hang: []Span{bar}, Spans: []Span{
		bar,
		{Text: "alpha beta gamma delta epsilon zeta", Style: term.StylePaletteItem},
	}}
	for _, width := range []int{8, 12, 20} {
		rows := WrapLine(line, width)
		if len(rows) < 2 {
			t.Fatalf("width %d: expected the quote to wrap, got %q", width, textLines(rows))
		}
		for i, row := range rows {
			if !strings.HasPrefix(row.Text(), "│ ") || row.Spans[0].Style != term.StyleBorder {
				t.Errorf("width %d row %d: %q does not start with the quote bar", width, i, row.Text())
			}
			if w := textwidth.String(row.Text()); w > width {
				t.Errorf("width %d row %d: %q occupies %d columns", width, i, row.Text(), w)
			}
			wantPrefix := 2
			if i == 0 {
				wantPrefix = 0
			}
			if row.PrefixRunes != wantPrefix {
				t.Errorf("width %d row %d: PrefixRunes = %d, want %d", width, i, row.PrefixRunes, wantPrefix)
			}
		}
		text, styles := unwrapRows(rows)
		if text != line.Text() || !slices.Equal(styles, FlattenStyles(line)) {
			t.Errorf("width %d: dropping the repeated prefix gives %q, want %q", width, text, line.Text())
		}
	}
}

func TestWrapLineHangWithWideRunes(t *testing.T) {
	line := Line{Kind: KindListItem, Hang: []Span{{Text: "  ", Style: term.StylePaletteItem}}, Spans: []Span{
		{Text: "• ", Style: term.StylePaletteItem},
		{Text: "東京 大阪 名古屋 札幌 福岡", Style: term.StylePaletteItem},
	}}
	for _, width := range []int{4, 6, 9} {
		rows := WrapLine(line, width)
		for i, row := range rows {
			if w := textwidth.String(row.Text()); w > width {
				t.Errorf("width %d row %d: %q occupies %d columns", width, i, row.Text(), w)
			}
			if i > 0 && !strings.HasPrefix(row.Text(), "  ") {
				t.Errorf("width %d row %d: %q lost its hanging indent", width, i, row.Text())
			}
		}
		if text, _ := unwrapRows(rows); text != line.Text() {
			t.Errorf("width %d: dropping the repeated prefix gives %q, want %q", width, text, line.Text())
		}
	}
}

func TestWrapLineKeepsTextAfterLeadingPrefix(t *testing.T) {
	bar := Span{Text: "│ ", Style: term.StyleBorder}
	line := Line{Kind: KindBlockquote, Hang: []Span{bar}, Spans: []Span{
		bar,
		{Text: "supercalifragilistic word", Style: term.StylePaletteItem},
	}}
	rows := WrapLine(line, 10)
	for i, row := range rows {
		if strings.TrimSpace(strings.TrimPrefix(row.Text(), "│ ")) == "" {
			t.Errorf("row %d holds only the prefix: %q", i, textLines(rows))
		}
	}
}

func assertLines(t *testing.T, got []Line, want ...string) {
	t.Helper()
	if tl := textLines(got); !slices.Equal(tl, want) {
		t.Errorf("got lines %q, want %q", tl, want)
	}
}

func wrapAll(lines []Line, width int) []Line {
	var rows []Line
	for _, l := range lines {
		if l.Kind.Wrappable() {
			rows = append(rows, WrapLine(l, width)...)
		} else {
			rows = append(rows, l)
		}
	}
	return rows
}

func TestRenderNestedListUnderOrderedItem(t *testing.T) {
	assertLines(t, Render("1. top\n   - nested a\n   - nested b"),
		"1. top", "   • nested a", "   • nested b")
}

func TestRenderThreeLevelList(t *testing.T) {
	assertLines(t, Render("- a\n  - b\n    - c"), "• a", "  • b", "    • c")
}

func TestRenderNestedQuote(t *testing.T) {
	assertLines(t, Render("> outer\n> > inner"), "│ outer", "│ │ inner")
}

func TestRenderListInsideQuote(t *testing.T) {
	assertLines(t, Render("> - item"), "│ • item")
}

func TestRenderNestedListAddsNoBlankInsideParent(t *testing.T) {
	assertLines(t, Render("1. top\n   - nested\n2. second"), "1. top", "   • nested", "2. second")
}

func TestRenderNestedQuoteThenTableSingleBlank(t *testing.T) {
	lines := Render("> outer\n> > inner\n\n| a | b |\n|---|---|\n| 1 | 2 |")
	if len(lines) < 4 || lines[2].Kind != KindBlank || lines[2].Text() != "" || lines[3].Kind != KindTable {
		t.Errorf("expected one plain blank between the quote and the table, got %q", textLines(lines))
	}
}

func TestRenderBlankInsideQuoteKeepsBar(t *testing.T) {
	assertLines(t, Render("> ```\n> code\n> ```\n> after"), "│ code", "│", "│ after")
}

func TestRenderNoConsecutiveBlankLines(t *testing.T) {
	input := "1. top\n   - nested a\n   - nested b\n2. second\n\nSteps continue below.\n\n" +
		"3. starts at three\n4. four\n\n> outer\n> > inner\n\n| name | note |\n|------|------|\n| 東京 | wide |\n\n" +
		"> - item in a quote\n\n- a\n  - b\n    - c\n\nend"
	lines := Render(input)
	for i := 1; i < len(lines); i++ {
		if lines[i].Kind == KindBlank && lines[i-1].Kind == KindBlank {
			t.Fatalf("consecutive blank lines at %d: %q", i, textLines(lines))
		}
	}
}

func TestRenderWrappedBulletKeepsHangingIndent(t *testing.T) {
	rows := wrapAll(Render("- a list item long enough that it has to wrap onto a second line when narrow"), 40)
	if len(rows) < 2 || !strings.HasPrefix(rows[0].Text(), "• ") {
		t.Fatalf("expected a wrapped bullet item, got %q", textLines(rows))
	}
	for i, row := range rows {
		if w := textwidth.String(row.Text()); w > 40 {
			t.Errorf("row %d occupies %d columns: %q", i, w, row.Text())
		}
		if i > 0 && (!strings.HasPrefix(row.Text(), "  ") || strings.HasPrefix(row.Text(), "   ")) {
			t.Errorf("row %d should start with exactly two spaces of hang: %q", i, row.Text())
		}
	}
}

func TestRenderWrappedTenthItemHangsFourSpaces(t *testing.T) {
	var src strings.Builder
	for i := 1; i <= 9; i++ {
		src.WriteString("1. x\n")
	}
	src.WriteString("1. the tenth item is long enough that it has to wrap onto another row\n")
	rows := wrapAll(Render(src.String()), 40)
	start := slices.IndexFunc(rows, func(l Line) bool { return strings.HasPrefix(l.Text(), "10. ") })
	if start < 0 || start+1 >= len(rows) {
		t.Fatalf("expected a wrapped tenth item, got %q", textLines(rows))
	}
	for _, row := range rows[start+1:] {
		if !strings.HasPrefix(row.Text(), "    ") || strings.HasPrefix(row.Text(), "     ") {
			t.Errorf("continuation should start with exactly four spaces: %q", row.Text())
		}
	}
}

func TestRenderWrappedQuoteKeepsBar(t *testing.T) {
	rows := wrapAll(Render("> a quote long enough that it has to wrap onto a second line when narrow"), 40)
	if len(rows) < 2 {
		t.Fatalf("expected the quote to wrap, got %q", textLines(rows))
	}
	for i, row := range rows {
		if !strings.HasPrefix(row.Text(), "│ ") {
			t.Errorf("row %d lost the quote bar: %q", i, row.Text())
		}
		if w := textwidth.String(row.Text()); w > 40 {
			t.Errorf("row %d occupies %d columns: %q", i, w, row.Text())
		}
	}
}

func TestRenderHardLineBreaks(t *testing.T) {
	assertLines(t, Render("line one  \nline two\\\nline three"), "line one", "line two", "line three")
}

func TestRenderSoftLineBreakJoinsWithSpace(t *testing.T) {
	assertLines(t, Render("alpha\nbeta"), "alpha beta")
}

func TestRenderHardBreakInListItemKeepsHang(t *testing.T) {
	lines := Render("- first  \n  second")
	assertLines(t, lines, "• first", "  second")
	hang := Line{Spans: lines[1].Hang}.Text()
	if lines[1].Kind != KindListItem || hang != "  " {
		t.Errorf("hard-break row should be a list item hanging by two spaces, got kind %v hang %q", lines[1].Kind, hang)
	}
}

func TestRenderHardBreakInQuoteKeepsBar(t *testing.T) {
	assertLines(t, Render("> first  \n> second"), "│ first", "│ second")
}

func TestRenderHardBreakInsideEmphasis(t *testing.T) {
	lines := Render("*first  \nsecond*")
	assertLines(t, lines, "first", "second")
	if lines[1].Spans[0].Style != term.StyleHoverItalic {
		t.Errorf("second row should stay italic, got style %v", lines[1].Spans[0].Style)
	}
}

func TestRenderOrderedListKeepsStartNumber(t *testing.T) {
	assertLines(t, Render("Steps continue below.\n\n3. starts at three\n4. four"),
		"Steps continue below.", "", "3. starts at three", "4. four")
}

func TestRenderOrderedListStartingAtOne(t *testing.T) {
	assertLines(t, Render("1. a\n2. b"), "1. a", "2. b")
}

func TestRenderNestedListAlignsUnderTwoDigitParent(t *testing.T) {
	assertLines(t, Render("9. nine\n   - child of nine\n10. ten\n    - child of ten"),
		"9. nine", "   • child of nine", "10. ten", "    • child of ten")
}

func TestRenderTableAlignsWideCells(t *testing.T) {
	lines := Render("| name | note |\n|------|------|\n| 東京 | wide |\n| ab | narrow |")
	var rows []string
	for _, l := range lines {
		if l.Kind == KindTable {
			rows = append(rows, l.Text())
		}
	}
	if len(rows) != 4 {
		t.Fatalf("expected 4 table rows, got %q", rows)
	}
	col := -1
	for _, row := range rows {
		i := strings.IndexAny(row, "│┼")
		if i < 0 {
			t.Fatalf("row %q has no column separator", row)
		}
		at := textwidth.String(row[:i])
		if col >= 0 && at != col {
			t.Errorf("separator at column %d in %q, want %d: %q", at, row, col, rows)
		}
		col = at
	}
}

func TestDetailsDefaultCollapsedState(t *testing.T) {
	input := `<details>
<summary>Click here for details</summary>

Here is the hidden content:
- bullet point 1
- bullet point 2
</details>`

	lines := Render(input)
	if len(lines) == 0 {
		t.Fatal("expected rendered lines")
	}
	if lines[0].Text() != "▶ Click here for details" {
		t.Fatalf("expected summary '▶ Click here for details', got %q", lines[0].Text())
	}
	if lines[0].Kind != KindSummary {
		t.Fatalf("expected KindSummary, got %v", lines[0].Kind)
	}
	tl := textLines(lines)
	for _, l := range tl {
		if strings.Contains(l, "bullet point") {
			t.Fatalf("hidden content leaked into collapsed output: %v", tl)
		}
	}

	doc := Parse(input)
	if len(doc.Details()) != 1 {
		t.Fatalf("expected 1 detail block, got %d", len(doc.Details()))
	}
	det := doc.Details()[0]
	if det.Open {
		t.Fatal("detail should start collapsed")
	}

	doc.Toggle(det.ID)
	if !det.Open {
		t.Fatal("detail should be open after toggle")
	}

	openLines := doc.Render()
	if openLines[0].Text() != "▼ Click here for details" {
		t.Fatalf("expected summary '▼ Click here for details', got %q", openLines[0].Text())
	}
	foundBullet := false
	for _, l := range textLines(openLines) {
		if strings.Contains(l, "bullet point 1") {
			foundBullet = true
		}
	}
	if !foundBullet {
		t.Fatalf("expected bullet point in open output: %v", textLines(openLines))
	}
}

func TestDetailsOpenAttribute(t *testing.T) {
	samples := []string{
		`<details open>
<summary>Open by default</summary>

Expanded body text
</details>`,
		`<details open="open">
<summary>Open by default</summary>

Expanded body text
</details>`,
		`<details class="collapsible" open>
<summary>Open by default</summary>

Expanded body text
</details>`,
	}

	for i, input := range samples {
		lines := Render(input)
		if lines[0].Text() != "▼ Open by default" {
			t.Fatalf("sample %d: expected '▼ Open by default', got %q", i, lines[0].Text())
		}
		foundBody := false
		for _, l := range textLines(lines) {
			if strings.Contains(l, "Expanded body text") {
				foundBody = true
			}
		}
		if !foundBody {
			t.Fatalf("sample %d: expected body text visible in open details: %v", i, textLines(lines))
		}
	}
}

func TestNestedDetails(t *testing.T) {
	input := `<details>
<summary>Outer</summary>

Outer body

<details>
<summary>Inner</summary>

Inner body
</details>

Outer after
</details>`

	doc := Parse(input)
	if len(doc.Details()) != 2 {
		t.Fatalf("expected 2 detail blocks, got %d", len(doc.Details()))
	}
	outerID := doc.Details()[0].ID
	innerID := doc.Details()[1].ID

	// 1. Both collapsed: only Outer summary is visible
	lines := doc.Render()
	tl := textLines(lines)
	if len(tl) != 1 || tl[0] != "▶ Outer" {
		t.Fatalf("expected only '▶ Outer', got %v", tl)
	}

	// 2. Expand outer: Outer body and Inner summary become visible
	doc.Toggle(outerID)
	lines = doc.Render()
	tl = textLines(lines)
	foundOuterBody := false
	foundInnerSummary := false
	foundInnerBody := false
	for _, l := range tl {
		if strings.Contains(l, "Outer body") {
			foundOuterBody = true
		}
		if l == "▶ Inner" {
			foundInnerSummary = true
		}
		if strings.Contains(l, "Inner body") {
			foundInnerBody = true
		}
	}
	if !foundOuterBody || !foundInnerSummary || foundInnerBody {
		t.Fatalf("expected outer body and collapsed inner summary, got %v", tl)
	}

	// 3. Expand inner: Inner body becomes visible
	doc.Toggle(innerID)
	lines = doc.Render()
	tl = textLines(lines)
	foundInnerBody = false
	foundInnerOpenSummary := false
	for _, l := range tl {
		if l == "▼ Inner" {
			foundInnerOpenSummary = true
		}
		if strings.Contains(l, "Inner body") {
			foundInnerBody = true
		}
	}
	if !foundInnerOpenSummary || !foundInnerBody {
		t.Fatalf("expected open inner summary and inner body, got %v", tl)
	}

	// 4. Collapse outer again: only Outer summary visible
	doc.Toggle(outerID)
	lines = doc.Render()
	tl = textLines(lines)
	if len(tl) != 1 || tl[0] != "▶ Outer" {
		t.Fatalf("expected only '▶ Outer' after outer collapsed, got %v", tl)
	}
}

func TestDetailsPreservesFencedCodeContainingHTML(t *testing.T) {
	input := "```html\n<details>\n<summary>Code</summary>\n</details>\n```"
	lines := Render(input)
	for _, l := range lines {
		if l.Kind == KindSummary {
			t.Fatalf("fenced code block was wrongly parsed as disclosure: %v", textLines(lines))
		}
	}
	tl := textLines(lines)
	foundDetails := false
	for _, l := range tl {
		if strings.Contains(l, "<details>") {
			foundDetails = true
		}
	}
	if !foundDetails {
		t.Fatalf("expected literal <details> inside code fence: %v", tl)
	}
}

func TestDetailsIgnoresClosingTagInInlineCode(t *testing.T) {
	input := "<details>\n<summary>More information</summary>\n\nUse `</details>` to close a section.\n\nThis should stay hidden until expanded.\n</details>"

	doc := Parse(input)
	if len(doc.Details()) != 1 {
		t.Fatalf("expected 1 detail block, got %d", len(doc.Details()))
	}
	tl := textLines(doc.Render())
	if len(tl) != 1 || tl[0] != "▶ More information" {
		t.Fatalf("expected only '▶ More information' when collapsed, got %v", tl)
	}

	doc.Toggle(doc.Details()[0].ID)
	tl = textLines(doc.Render())
	want := []string{
		"▼ More information",
		"",
		"Use </details> to close a section.",
		"",
		"This should stay hidden until expanded.",
	}
	if !slices.Equal(tl, want) {
		t.Fatalf("expected %v when expanded, got %v", want, tl)
	}
}

func TestDetailsClosesAfterEscapedBacktick(t *testing.T) {
	input := "<details>\n<summary>More information</summary>\n\nInside.\n\\` </details> `\n\nOutside."

	doc := Parse(input)
	if len(doc.Details()) != 1 {
		t.Fatalf("expected 1 detail block, got %d", len(doc.Details()))
	}
	tl := textLines(doc.Render())
	want := []string{"▶ More information", "", "Outside."}
	if !slices.Equal(tl, want) {
		t.Fatalf("expected %v when collapsed, got %v", want, tl)
	}
}

func TestDetailsPreservesOrdinaryProseWithHTML(t *testing.T) {
	input := "Use `<details>` and <details> in prose."
	lines := Render(input)
	if len(lines) == 0 {
		t.Fatal("expected lines")
	}
	if lines[0].Kind != KindParagraph {
		t.Fatalf("expected KindParagraph, got %v", lines[0].Kind)
	}
	if !strings.Contains(lines[0].Text(), "<details>") {
		t.Fatalf("expected literal <details> preserved in prose: %q", lines[0].Text())
	}
}

func TestDetailsDefaultSummaryText(t *testing.T) {
	input := "<details>\nNo summary tag provided\n</details>"
	lines := Render(input)
	if lines[0].Text() != "▶ Details" {
		t.Fatalf("expected '▶ Details', got %q", lines[0].Text())
	}
}

func TestDetailsPreservesMarkdownStylesInside(t *testing.T) {
	input := `<details open>
<summary><b>Styled</b> Summary</summary>

### Inner Heading

- list item 1

` + "```go\nfunc foo() {}\n```" + `

| Col1 | Col2 |
|---|---|
| A | B |
</details>`

	lines := Render(input)
	foundHeading := false
	foundListItem := false
	foundCode := false
	foundTable := false

	for _, l := range lines {
		if l.Kind == KindHeading && strings.Contains(l.Text(), "Inner Heading") {
			foundHeading = true
		}
		if l.Kind == KindListItem && strings.Contains(l.Text(), "list item 1") {
			foundListItem = true
		}
		if l.Kind == KindCode && strings.Contains(l.Text(), "func foo") {
			foundCode = true
		}
		if l.Kind == KindTable {
			foundTable = true
		}
	}

	if !foundHeading {
		t.Error("expected KindHeading inside open details")
	}
	if !foundListItem {
		t.Error("expected KindListItem inside open details")
	}
	if !foundCode {
		t.Error("expected KindCode inside open details")
	}
	if !foundTable {
		t.Error("expected KindTable inside open details")
	}
}

func TestNestedSingleLineDetailsTrailingContent(t *testing.T) {
	input := `<details>
<summary>Outer</summary>
<details><summary>Inner</summary>Nested body</details>
Trailing content inside outer
</details>

Content outside outer`

	doc := Parse(input)
	if len(doc.Details()) != 2 {
		t.Fatalf("expected 2 details, got %d", len(doc.Details()))
	}

	// When outer is collapsed, content outside the outer section must remain visible.
	doc.SetOpen(1, false)
	rendered := doc.Render()
	foundOutside := false
	foundInside := false
	for _, l := range rendered {
		text := l.Text()
		if strings.Contains(text, "Content outside outer") {
			foundOutside = true
		}
		if strings.Contains(text, "Trailing content inside outer") {
			foundInside = true
		}
	}
	if !foundOutside {
		t.Error("content outside outer details should remain visible when outer is collapsed")
	}
	if foundInside {
		t.Error("content inside outer details should be hidden when outer is collapsed")
	}

	// When outer is expanded, both inside and outside content are visible.
	doc.SetOpen(1, true)
	renderedOpen := doc.Render()
	foundOutsideOpen := false
	foundInsideOpen := false
	for _, l := range renderedOpen {
		text := l.Text()
		if strings.Contains(text, "Content outside outer") {
			foundOutsideOpen = true
		}
		if strings.Contains(text, "Trailing content inside outer") {
			foundInsideOpen = true
		}
	}
	if !foundOutsideOpen {
		t.Error("content outside outer details should be visible when outer is open")
	}
	if !foundInsideOpen {
		t.Error("content inside outer details should be visible when outer is open")
	}
}

func TestWrapLinePreservesDetailID(t *testing.T) {
	line := Line{Kind: KindSummary, DetailID: 7, Spans: []Span{{Text: "▶ A summary long enough to wrap", Style: term.StyleHoverBold}}}
	rows := WrapLine(line, 10)
	if len(rows) < 2 {
		t.Fatal("expected wrapped summary")
	}
	for _, row := range rows {
		if row.DetailID != line.DetailID || row.Kind != KindSummary {
			t.Fatalf("wrapped summary lost disclosure metadata: %+v", row)
		}
	}
}
