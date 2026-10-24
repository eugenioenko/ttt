package markdown

import (
	"fmt"
	"slices"
	"strings"

	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	gmtext "github.com/yuin/goldmark/text"
)

type LineKind int

const (
	KindParagraph LineKind = iota
	KindHeading
	KindCode
	KindTable
	KindDivider
	KindListItem
	KindBlockquote
	KindBlank
)

func (k LineKind) Wrappable() bool {
	switch k {
	case KindParagraph, KindHeading, KindListItem, KindBlockquote:
		return true
	default:
		return false
	}
}

type Span struct {
	Text  string
	Style term.Style
}

type Line struct {
	Spans []Span
	Kind  LineKind
	Hang  []Span
	// PrefixRunes is how many leading runes of a wrapped row repeat Hang
	// instead of carrying source text; offsets into the unwrapped line must
	// skip them.
	PrefixRunes int
}

func (l Line) Text() string {
	var b strings.Builder
	for _, s := range l.Spans {
		b.WriteString(s.Text)
	}
	return b.String()
}

func WrapLine(line Line, width int) []Line {
	text := line.Text()
	if width <= 0 || textwidth.String(text) <= width {
		return []Line{line}
	}
	hang := Line{Spans: line.Hang}.Text()
	hangRunes := len([]rune(hang))
	hangWidth := textwidth.String(hang)
	styles := FlattenStyles(line)
	runes := []rune(text)
	var result []Line
	rowWidth := width
	// The first row starts with the line's own prefix (a marker or quote bar),
	// which is as long as Hang; breaking at a space inside it would leave a row
	// holding nothing but the prefix.
	minBreak := max(1, hangRunes)
	for len(runes) > 0 {
		end, columns := 0, 0
		for end < len(runes) {
			w := textwidth.Rune(runes[end])
			if columns+w > rowWidth && end > 0 {
				break
			}
			columns += w
			end++
		}
		if end < len(runes) {
			bp := -1
			for j := end - 1; j >= minBreak; j-- {
				if runes[j] == ' ' {
					bp = j
					break
				}
			}
			if bp > 0 {
				end = bp + 1
			}
		}
		var spans []Span
		pos := 0
		chunk := runes[:end]
		chunkStyles := styles[:end]
		for pos < len(chunk) {
			st := chunkStyles[pos]
			start := pos
			for pos < len(chunk) && chunkStyles[pos] == st {
				pos++
			}
			spans = append(spans, Span{Text: string(chunk[start:pos]), Style: st})
		}
		row := Line{Kind: line.Kind, Spans: spans, Hang: line.Hang}
		if len(result) > 0 && hangRunes > 0 {
			row.Spans = append(append([]Span(nil), line.Hang...), spans...)
			row.PrefixRunes = hangRunes
		}
		result = append(result, row)
		runes = runes[end:]
		styles = styles[end:]
		rowWidth = max(1, width-hangWidth)
		minBreak = 1
	}
	return result
}

func FlattenStyles(line Line) []term.Style {
	var styles []term.Style
	for _, span := range line.Spans {
		for range []rune(span.Text) {
			styles = append(styles, span.Style)
		}
	}
	return styles
}

var md = goldmark.New(goldmark.WithExtensions(extension.Table))

func Render(text string) []Line {
	src := []byte(text)
	doc := md.Parser().Parse(gmtext.NewReader(src))
	r := &renderer{src: src}
	r.renderNode(doc)
	r.dropTrailingBlanks()
	if len(r.lines) == 0 {
		r.lines = []Line{{Spans: []Span{{Text: "", Style: term.StyleDefault}}}}
	}
	return r.lines
}

type renderer struct {
	src   []byte
	lines []Line
	// indent holds the continuation prefix of every enclosing list item and
	// quote, outermost first.
	indent []Span
}

func (r *renderer) emit(line Line) {
	line.Spans = append(append([]Span(nil), r.indent...), line.Spans...)
	line.Hang = append(append([]Span(nil), r.indent...), line.Hang...)
	r.lines = append(r.lines, line)
}

func (r *renderer) emitBlank() {
	if n := len(r.lines); n > 0 && r.lines[n-1].Kind == KindBlank {
		return
	}
	spans := trimTrailingSpaces(r.indent)
	if len(spans) == 0 {
		spans = []Span{{Text: "", Style: term.StyleDefault}}
	}
	r.lines = append(r.lines, Line{Kind: KindBlank, Spans: spans})
}

func (r *renderer) pushIndent(spans ...Span) {
	r.indent = append(r.indent, spans...)
}

func (r *renderer) popIndent(n int) {
	r.indent = r.indent[:len(r.indent)-n]
}

func trimTrailingSpaces(spans []Span) []Span {
	out := append([]Span(nil), spans...)
	for len(out) > 0 {
		last := &out[len(out)-1]
		last.Text = strings.TrimRight(last.Text, " ")
		if last.Text != "" {
			break
		}
		out = out[:len(out)-1]
	}
	return out
}

func (r *renderer) dropTrailingBlanks() {
	for len(r.lines) > 0 && r.lines[len(r.lines)-1].Kind == KindBlank {
		r.lines = r.lines[:len(r.lines)-1]
	}
}

func (r *renderer) renderNode(n ast.Node) {
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		r.renderBlock(child)
	}
}

func (r *renderer) renderBlock(n ast.Node) {
	switch n := n.(type) {
	case *ast.Heading:
		spans := r.collectInlineSpans(n, term.StyleHoverBold)
		prefix := strings.Repeat("#", n.Level) + " "
		spans = append([]Span{{Text: prefix, Style: term.StyleHoverBold}}, spans...)
		r.emit(Line{Kind: KindHeading, Spans: spans})
		r.emitBlank()

	case *ast.Paragraph:
		r.emitInline(KindParagraph, nil, nil, r.collectInlineLines(n, term.StylePaletteItem))
		r.emitBlank()

	case *ast.TextBlock:
		r.emitInline(KindParagraph, nil, nil, r.collectInlineLines(n, term.StylePaletteItem))

	case *ast.FencedCodeBlock:
		lang := ""
		if n.Info != nil {
			lang = strings.TrimSpace(string(n.Info.Value(r.src)))
		}
		r.emitCodeBlock(r.collectBlockLines(n), lang)
		r.emitBlank()

	case *ast.CodeBlock:
		r.emitCodeBlock(r.collectBlockLines(n), "")
		r.emitBlank()

	case *ast.ThematicBreak:
		r.dropTrailingBlanks()
		r.emit(Line{Kind: KindDivider, Spans: []Span{{Text: "---", Style: term.StyleBorder}}})

	case *ast.List:
		r.renderList(n)
		r.emitBlank()

	case *east.Table:
		r.renderTable(n)
		r.emitBlank()

	case *ast.Blockquote:
		r.pushIndent(Span{Text: "│ ", Style: term.StyleBorder})
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			if p, ok := child.(*ast.Paragraph); ok {
				r.emitInline(KindBlockquote, nil, nil, r.collectInlineLines(p, term.StylePaletteItem))
			} else {
				r.renderBlock(child)
			}
		}
		r.dropTrailingBlanks()
		r.popIndent(1)
		r.emitBlank()

	case *ast.HTMLBlock:
		for i := 0; i < n.Lines().Len(); i++ {
			seg := n.Lines().At(i)
			line := strings.TrimRight(string(seg.Value(r.src)), "\n")
			r.emit(Line{Kind: KindCode, Spans: []Span{{Text: line, Style: term.StylePaletteItem}}})
		}
		r.emitBlank()

	default:
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			r.renderBlock(child)
		}
	}
}

func (r *renderer) collectBlockLines(n ast.Node) []string {
	var block []string
	for i := 0; i < n.Lines().Len(); i++ {
		seg := n.Lines().At(i)
		block = append(block, strings.TrimRight(string(seg.Value(r.src)), "\n"))
	}
	return block
}

func (r *renderer) emitCodeBlock(block []string, lang string) {
	for _, cl := range renderCodeBlock(block, lang) {
		cl.Kind = KindCode
		r.emit(cl)
	}
}

func (r *renderer) renderList(list *ast.List) {
	num := list.Start
	for child := list.FirstChild(); child != nil; child = child.NextSibling() {
		li, ok := child.(*ast.ListItem)
		if !ok {
			continue
		}
		marker := "• "
		if list.IsOrdered() {
			marker = fmt.Sprintf("%d. ", num)
			num++
		}
		r.renderListItem(li, marker)
	}
}

func (r *renderer) renderListItem(li *ast.ListItem, marker string) {
	hang := Span{Text: strings.Repeat(" ", textwidth.String(marker)), Style: term.StylePaletteItem}
	first := true
	for child := li.FirstChild(); child != nil; child = child.NextSibling() {
		isText := child.Kind() == ast.KindTextBlock || child.Kind() == ast.KindParagraph
		if first && isText {
			lead := []Span{{Text: marker, Style: term.StylePaletteItem}}
			r.emitInline(KindListItem, lead, []Span{hang}, r.collectInlineLines(child, term.StylePaletteItem))
			first = false
			continue
		}
		r.pushIndent(hang)
		switch c := child.(type) {
		case *ast.List:
			r.renderList(c)
		default:
			if isText {
				r.emitInline(KindListItem, nil, nil, r.collectInlineLines(child, term.StylePaletteItem))
			} else {
				r.renderBlock(child)
			}
		}
		r.popIndent(1)
	}
}

func (r *renderer) renderTable(t *east.Table) {
	var headers [][]Span
	var rows [][][]Span
	var colWidths []int

	if header := t.FirstChild(); header != nil {
		if th, ok := header.(*east.TableHeader); ok {
			for cell := th.FirstChild(); cell != nil; cell = cell.NextSibling() {
				if tc, ok := cell.(*east.TableCell); ok {
					spans := r.collectInlineSpans(tc, term.StyleHoverBold)
					headers = append(headers, spans)
					w := spanWidth(spans)
					colWidths = append(colWidths, w)
				}
			}
		}
	}

	for child := t.FirstChild(); child != nil; child = child.NextSibling() {
		row, ok := child.(*east.TableRow)
		if !ok {
			continue
		}
		var cells [][]Span
		col := 0
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			if tc, ok := cell.(*east.TableCell); ok {
				spans := r.collectInlineSpans(tc, term.StylePaletteItem)
				cells = append(cells, spans)
				w := spanWidth(spans)
				if col < len(colWidths) {
					if w > colWidths[col] {
						colWidths[col] = w
					}
				} else {
					colWidths = append(colWidths, w)
				}
				col++
			}
		}
		rows = append(rows, cells)
	}

	r.emitTableRow(headers, colWidths, term.StyleHoverBold)
	var sep []Span
	for i, w := range colWidths {
		if i > 0 {
			sep = append(sep, Span{Text: "─┼─", Style: term.StyleBorder})
		}
		sep = append(sep, Span{Text: strings.Repeat("─", w), Style: term.StyleBorder})
	}
	r.emit(Line{Kind: KindTable, Spans: sep})
	for _, row := range rows {
		r.emitTableRow(row, colWidths, term.StylePaletteItem)
	}
}

func (r *renderer) emitTableRow(cells [][]Span, colWidths []int, defaultStyle term.Style) {
	var spans []Span
	for i, cell := range cells {
		if i > 0 {
			spans = append(spans, Span{Text: " │ ", Style: term.StyleBorder})
		}
		spans = append(spans, cell...)
		w := spanWidth(cell)
		pad := 0
		if i < len(colWidths) {
			pad = colWidths[i] - w
		}
		if pad > 0 {
			spans = append(spans, Span{Text: strings.Repeat(" ", pad), Style: defaultStyle})
		}
	}
	r.emit(Line{Kind: KindTable, Spans: spans})
}

func spanWidth(spans []Span) int {
	n := 0
	for _, s := range spans {
		n += textwidth.String(s.Text)
	}
	return n
}

func (r *renderer) collectInlineSpans(n ast.Node, defaultStyle term.Style) []Span {
	return slices.Concat(r.collectInlineLines(n, defaultStyle)...)
}

func (r *renderer) collectInlineLines(n ast.Node, defaultStyle term.Style) [][]Span {
	segs := inlineLines{nil}
	r.walkInline(n, defaultStyle, &segs)
	for len(segs) > 1 && len(segs[len(segs)-1]) == 0 {
		segs = segs[:len(segs)-1]
	}
	for i := range segs {
		if len(segs[i]) == 0 {
			segs[i] = []Span{{Text: "", Style: defaultStyle}}
		}
	}
	return segs
}

func (r *renderer) emitInline(kind LineKind, lead, hang []Span, segs [][]Span) {
	for i, seg := range segs {
		prefix := lead
		if i > 0 {
			prefix = hang
		}
		spans := append(append([]Span(nil), prefix...), seg...)
		r.emit(Line{Kind: kind, Spans: spans, Hang: hang})
	}
}

type inlineLines [][]Span

func (l *inlineLines) add(s Span) {
	last := len(*l) - 1
	(*l)[last] = append((*l)[last], s)
}

func (r *renderer) walkInline(n ast.Node, style term.Style, segs *inlineLines) {
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		switch c := child.(type) {
		case *ast.Text:
			txt := string(c.Value(r.src))
			if txt != "" {
				segs.add(Span{Text: txt, Style: style})
			}
			if c.HardLineBreak() {
				*segs = append(*segs, nil)
			} else if c.SoftLineBreak() && child.NextSibling() != nil {
				segs.add(Span{Text: " ", Style: style})
			}
		case *ast.String:
			txt := string(c.Value)
			if txt != "" {
				segs.add(Span{Text: txt, Style: style})
			}
		case *ast.CodeSpan:
			txt := string(c.Text(r.src)) //nolint:staticcheck // Node.Text is deprecated but walkInline may differ for edge-case CodeSpan children
			segs.add(Span{Text: txt, Style: term.StyleHoverCode})
		case *ast.Emphasis:
			emphStyle := term.StyleHoverBold
			if c.Level == 1 {
				emphStyle = term.StyleHoverItalic
			}
			r.walkInline(c, emphStyle, segs)
		case *ast.Link:
			r.walkInline(c, style, segs)
		case *ast.AutoLink:
			txt := string(c.URL(r.src))
			segs.add(Span{Text: txt, Style: style})
		default:
			r.walkInline(child, style, segs)
		}
	}
}

func renderCodeBlock(block []string, lang string) []Line {
	var h *highlight.Highlighter
	if lang != "" {
		h = highlight.NewForLanguage(lang)
		h.Synchronous()
	}
	lines := make([]Line, len(block))
	for i, text := range block {
		if h != nil {
			spans := h.HighlightLineAt(block, i)
			lines[i] = highlightToLine(text, spans)
		} else {
			lines[i] = Line{Spans: []Span{{Text: text, Style: term.StyleHoverCode}}}
		}
	}
	return lines
}

func highlightToLine(text string, spans []highlight.Span) Line {
	if len(spans) == 0 {
		return Line{Spans: []Span{{Text: text, Style: term.StyleHoverCode}}}
	}
	runes := []rune(text)
	var result []Span
	pos := 0
	for _, s := range spans {
		if s.Start > pos {
			result = append(result, Span{Text: string(runes[pos:s.Start]), Style: term.StyleHoverCode})
		}
		result = append(result, Span{Text: string(runes[s.Start:s.End]), Style: s.Style})
		pos = s.End
	}
	if pos < len(runes) {
		result = append(result, Span{Text: string(runes[pos:]), Style: term.StyleHoverCode})
	}
	return Line{Spans: result}
}
