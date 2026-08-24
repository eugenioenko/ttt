package markdown

import (
	"fmt"
	"strings"

	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/term"
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
	if len([]rune(text)) <= width {
		return []Line{line}
	}
	styles := FlattenStyles(line)
	runes := []rune(text)
	var result []Line
	for len(runes) > 0 {
		end := width
		if end > len(runes) {
			end = len(runes)
		}
		if end < len(runes) {
			bp := -1
			for j := end - 1; j > 0; j-- {
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
		result = append(result, Line{Kind: line.Kind, Spans: spans})
		runes = runes[end:]
		styles = styles[end:]
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
	if len(r.lines) == 0 {
		r.lines = []Line{{Spans: []Span{{Text: "", Style: term.StyleDefault}}}}
	}
	return r.lines
}

type renderer struct {
	src   []byte
	lines []Line
}

func (r *renderer) emit(line Line) {
	r.lines = append(r.lines, line)
}

func (r *renderer) emitBlank() {
	r.lines = append(r.lines, Line{Kind: KindBlank, Spans: []Span{{Text: "", Style: term.StyleDefault}}})
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
		spans := r.collectInlineSpans(n, term.StylePaletteItem)
		r.emit(Line{Kind: KindParagraph, Spans: spans})
		r.emitBlank()

	case *ast.TextBlock:
		spans := r.collectInlineSpans(n, term.StylePaletteItem)
		r.emit(Line{Kind: KindParagraph, Spans: spans})

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
		r.emit(Line{Kind: KindDivider, Spans: []Span{{Text: "---", Style: term.StyleBorder}}})
		r.emitBlank()

	case *ast.List:
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			if li, ok := child.(*ast.ListItem); ok {
				r.renderListItem(li, n.IsOrdered())
			}
		}
		r.emitBlank()

	case *east.Table:
		r.renderTable(n)
		r.emitBlank()

	case *ast.Blockquote:
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			if p, ok := child.(*ast.Paragraph); ok {
				spans := r.collectInlineSpans(p, term.StylePaletteItem)
				spans = append([]Span{{Text: "│ ", Style: term.StyleBorder}}, spans...)
				r.emit(Line{Kind: KindBlockquote, Spans: spans})
			} else {
				r.renderBlock(child)
			}
		}
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
		r.lines = append(r.lines, cl)
	}
}

func (r *renderer) renderListItem(li *ast.ListItem, ordered bool) {
	marker := "• "
	if ordered {
		idx := 0
		parent := li.Parent()
		for c := parent.FirstChild(); c != nil; c = c.NextSibling() {
			idx++
			if c == li {
				break
			}
		}
		marker = fmt.Sprintf("%d. ", idx)
	}

	first := true
	for child := li.FirstChild(); child != nil; child = child.NextSibling() {
		switch child.Kind() {
		case ast.KindTextBlock, ast.KindParagraph:
			spans := r.collectInlineSpans(child, term.StylePaletteItem)
			if first {
				spans = append([]Span{{Text: marker, Style: term.StylePaletteItem}}, spans...)
				first = false
			} else {
				indent := strings.Repeat(" ", len([]rune(marker)))
				spans = append([]Span{{Text: indent, Style: term.StylePaletteItem}}, spans...)
			}
			r.emit(Line{Kind: KindListItem, Spans: spans})
		default:
			r.renderBlock(child)
		}
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
		n += len([]rune(s.Text))
	}
	return n
}

func (r *renderer) collectInlineSpans(n ast.Node, defaultStyle term.Style) []Span {
	var spans []Span
	r.walkInline(n, defaultStyle, &spans)
	if len(spans) == 0 {
		spans = []Span{{Text: "", Style: defaultStyle}}
	}
	return spans
}

func (r *renderer) walkInline(n ast.Node, style term.Style, spans *[]Span) {
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		switch c := child.(type) {
		case *ast.Text:
			txt := string(c.Value(r.src))
			if txt != "" {
				*spans = append(*spans, Span{Text: txt, Style: style})
			}
			if c.SoftLineBreak() && child.NextSibling() != nil {
				*spans = append(*spans, Span{Text: " ", Style: style})
			}
		case *ast.String:
			txt := string(c.Value)
			if txt != "" {
				*spans = append(*spans, Span{Text: txt, Style: style})
			}
		case *ast.CodeSpan:
			txt := string(c.Text(r.src)) //nolint:staticcheck // Node.Text is deprecated but walkInline may differ for edge-case CodeSpan children
			*spans = append(*spans, Span{Text: txt, Style: term.StyleHoverCode})
		case *ast.Emphasis:
			emphStyle := term.StyleHoverBold
			if c.Level == 1 {
				emphStyle = term.StyleHoverItalic
			}
			r.walkInline(c, emphStyle, spans)
		case *ast.Link:
			r.walkInline(c, style, spans)
		case *ast.AutoLink:
			txt := string(c.URL(r.src))
			*spans = append(*spans, Span{Text: txt, Style: style})
		default:
			r.walkInline(child, style, spans)
		}
	}
}

func renderCodeBlock(block []string, lang string) []Line {
	var h *highlight.Highlighter
	if lang != "" {
		h = highlight.New("file." + lang)
	}
	lines := make([]Line, len(block))
	for i, text := range block {
		if h != nil {
			spans := h.HighlightLine(text)
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
