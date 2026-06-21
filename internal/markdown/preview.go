package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/eugenioenko/ttt/internal/core/highlight"
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

const PreviewWrapWidth = 80

// RenderPreview parses markdown content using goldmark and produces styled lines
// suitable for a read-only preview panel. maxWidth controls word wrapping.
func RenderPreview(content string, maxWidth int) []Line {
	if maxWidth <= 0 {
		maxWidth = PreviewWrapWidth
	}
	source := []byte(content)
	md := goldmark.New()
	doc := md.Parser().Parse(text.NewReader(source))

	var lines []Line
	walkNode(doc, source, maxWidth, &lines, 0, false, false)
	if lines == nil {
		lines = []Line{}
	}
	return lines
}

func walkNode(node ast.Node, source []byte, maxWidth int, lines *[]Line, depth int, inBlockquote bool, inList bool) {
	switch n := node.(type) {
	case *ast.Document:
		walkChildren(n, source, maxWidth, lines, depth, inBlockquote, inList)

	case *ast.Heading:
		var spans []Span
		collectInlineSpans(n, source, &spans, term.StyleHoverBold)
		prefix := strings.Repeat("#", n.Level) + " "
		headerSpans := []Span{{Text: prefix, Style: term.StyleHoverBold}}
		headerSpans = append(headerSpans, spans...)
		line := Line{Spans: headerSpans}
		wrapped := wrapLine(line, maxWidth)
		addBlankLine(lines)
		*lines = append(*lines, wrapped...)
		addBlankLine(lines)

	case *ast.Paragraph:
		var spans []Span
		collectInlineSpans(n, source, &spans, term.StyleDefault)
		if len(spans) == 0 {
			spans = []Span{{Text: "", Style: term.StyleDefault}}
		}
		line := Line{Spans: spans}
		if inBlockquote {
			wrapped := wrapLine(line, maxWidth-4)
			for _, w := range wrapped {
				quoted := prependBlockquote(w)
				*lines = append(*lines, quoted)
			}
		} else if inList {
			wrapped := wrapLine(line, maxWidth)
			*lines = append(*lines, wrapped...)
		} else {
			wrapped := wrapLine(line, maxWidth)
			*lines = append(*lines, wrapped...)
			addBlankLine(lines)
		}

	case *ast.FencedCodeBlock, *ast.CodeBlock:
		var codeLines []string
		lang := ""
		if fcb, ok := n.(*ast.FencedCodeBlock); ok {
			lang = string(fcb.Language(source))
		}
		for i := 0; i < n.Lines().Len(); i++ {
			seg := n.Lines().At(i)
			line := string(seg.Value(source))
			line = strings.TrimRight(line, "\n")
			codeLines = append(codeLines, line)
		}
		rendered := renderCodeBlockPreview(codeLines, lang)
		*lines = append(*lines, rendered...)
		addBlankLine(lines)

	case *ast.Blockquote:
		walkChildren(n, source, maxWidth, lines, depth+1, true, inList)

	case *ast.List:
		addBlankLineIfNeeded(lines)
		itemIndex := 1
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			if listItem, ok := child.(*ast.ListItem); ok {
				var bullet string
				if n.IsOrdered() {
					bullet = fmt.Sprintf("  %d. ", itemIndex)
					itemIndex++
				} else {
					bullet = "  • "
				}
				renderListItem(listItem, source, maxWidth, lines, bullet, depth)
			}
		}
		addBlankLine(lines)

	case *ast.ListItem:
		// Handled by the List case above
		return

	case *ast.ThematicBreak:
		rule := strings.Repeat("─", min(maxWidth, 40))
		*lines = append(*lines, Line{Spans: []Span{{Text: rule, Style: term.StyleMuted}}})
		addBlankLine(lines)

	case *ast.TextBlock:
		var spans []Span
		collectInlineSpans(n, source, &spans, term.StyleDefault)
		if len(spans) > 0 {
			line := Line{Spans: spans}
			wrapped := wrapLine(line, maxWidth)
			*lines = append(*lines, wrapped...)
		}

	case *ast.HTMLBlock:
		// Render raw HTML as plain text
		for i := 0; i < n.Lines().Len(); i++ {
			seg := n.Lines().At(i)
			text := strings.TrimRight(string(seg.Value(source)), "\n")
			*lines = append(*lines, Line{Spans: []Span{{Text: text, Style: term.StyleMuted}}})
		}

	default:
		// For unknown block nodes, try walking children
		if node.HasChildren() {
			walkChildren(node, source, maxWidth, lines, depth, inBlockquote, inList)
		}
	}
}

func walkChildren(node ast.Node, source []byte, maxWidth int, lines *[]Line, depth int, inBlockquote bool, inList bool) {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		walkNode(child, source, maxWidth, lines, depth, inBlockquote, inList)
	}
}

func renderListItem(item *ast.ListItem, source []byte, maxWidth int, lines *[]Line, bullet string, depth int) {
	bulletLen := len([]rune(bullet))
	indent := strings.Repeat(" ", bulletLen)

	// Collect inline content from the first paragraph (or text block) of the list item
	first := true
	for child := item.FirstChild(); child != nil; child = child.NextSibling() {
		switch c := child.(type) {
		case *ast.Paragraph, *ast.TextBlock:
			var spans []Span
			collectInlineSpans(c, source, &spans, term.StyleDefault)
			if len(spans) == 0 {
				continue
			}
			line := Line{Spans: spans}
			wrapped := wrapLine(line, maxWidth-bulletLen)
			for i, w := range wrapped {
				var prefix string
				if first && i == 0 {
					prefix = bullet
				} else {
					prefix = indent
				}
				prefixed := Line{Spans: append([]Span{{Text: prefix, Style: term.StyleDefault}}, w.Spans...)}
				*lines = append(*lines, prefixed)
			}
			first = false
		case *ast.List:
			// Nested list
			walkNode(c, source, maxWidth-bulletLen, lines, depth+1, false, true)
		default:
			walkNode(c, source, maxWidth, lines, depth+1, false, true)
		}
	}
}

func collectInlineSpans(node ast.Node, source []byte, spans *[]Span, defaultStyle term.Style) {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		collectInlineNode(child, source, spans, defaultStyle)
	}
}

func collectInlineNode(node ast.Node, source []byte, spans *[]Span, defaultStyle term.Style) {
	switch n := node.(type) {
	case *ast.Text:
		text := string(n.Segment.Value(source))
		if len(text) > 0 {
			*spans = append(*spans, Span{Text: text, Style: defaultStyle})
		}
		if n.SoftLineBreak() {
			*spans = append(*spans, Span{Text: " ", Style: defaultStyle})
		}
		if n.HardLineBreak() {
			*spans = append(*spans, Span{Text: " ", Style: defaultStyle})
		}

	case *ast.String:
		text := string(n.Value)
		if len(text) > 0 {
			*spans = append(*spans, Span{Text: text, Style: defaultStyle})
		}

	case *ast.CodeSpan:
		var buf bytes.Buffer
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			if t, ok := child.(*ast.Text); ok {
				buf.Write(t.Segment.Value(source))
			}
		}
		*spans = append(*spans, Span{Text: buf.String(), Style: term.StyleHoverCode})

	case *ast.Emphasis:
		style := term.StyleHoverBold
		collectInlineSpans(n, source, spans, style)

	case *ast.Link:
		// Render link text then URL in muted style
		collectInlineSpans(n, source, spans, defaultStyle)
		url := string(n.Destination)
		if url != "" {
			*spans = append(*spans, Span{Text: " (" + url + ")", Style: term.StyleMuted})
		}

	case *ast.Image:
		// Show alt text
		*spans = append(*spans, Span{Text: "[image: ", Style: term.StyleMuted})
		collectInlineSpans(n, source, spans, term.StyleMuted)
		*spans = append(*spans, Span{Text: "]", Style: term.StyleMuted})

	case *ast.AutoLink:
		url := string(n.URL(source))
		*spans = append(*spans, Span{Text: url, Style: term.StyleMuted})

	case *ast.RawHTML:
		var buf bytes.Buffer
		for i := 0; i < n.Segments.Len(); i++ {
			seg := n.Segments.At(i)
			buf.Write(seg.Value(source))
		}
		*spans = append(*spans, Span{Text: buf.String(), Style: term.StyleMuted})

	default:
		// For unknown inline nodes with children, recurse
		if node.HasChildren() {
			collectInlineSpans(node, source, spans, defaultStyle)
		}
	}
}

func renderCodeBlockPreview(block []string, lang string) []Line {
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

func prependBlockquote(line Line) Line {
	prefix := Span{Text: "  │ ", Style: term.StyleMuted}
	newSpans := make([]Span, 0, len(line.Spans)+1)
	newSpans = append(newSpans, prefix)
	// Apply muted style to all spans in blockquote
	for _, s := range line.Spans {
		newSpans = append(newSpans, Span{Text: s.Text, Style: term.StyleMuted})
	}
	return Line{Spans: newSpans}
}

func addBlankLine(lines *[]Line) {
	*lines = append(*lines, Line{Spans: []Span{{Text: "", Style: term.StyleDefault}}})
}

func addBlankLineIfNeeded(lines *[]Line) {
	if len(*lines) == 0 {
		return
	}
	last := (*lines)[len(*lines)-1]
	if last.Text() != "" {
		addBlankLine(lines)
	}
}

