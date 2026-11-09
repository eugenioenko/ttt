package markdown

import (
	"html"
	"regexp"
	"strings"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/yuin/goldmark/ast"
)

var (
	htmlCommentRegex = regexp.MustCompile(`(?s)<!--.*?-->`)
	htmlBreakRegex   = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlImageRegex   = regexp.MustCompile(`(?i)<img\b[^>]*>`)
	htmlAltRegex     = regexp.MustCompile(`(?i)\balt\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	// Parse consumes real disclosures before goldmark runs, so a details or
	// summary tag that reaches an HTML block is literal text (for example one
	// inside a multiline code span) and must stay visible.
	htmlDisclosureTag = regexp.MustCompile(`(?i)^</?(details|summary)\b`)
)

func stripTag(tag string) string {
	if htmlDisclosureTag.MatchString(tag) {
		return tag
	}
	return ""
}

func imageAlt(tag string) string {
	m := htmlAltRegex.FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	return html.UnescapeString(m[1] + m[2])
}

func stripHTML(raw string) string {
	s := htmlCommentRegex.ReplaceAllString(raw, "")
	s = htmlBreakRegex.ReplaceAllString(s, "\n")
	s = htmlImageRegex.ReplaceAllStringFunc(s, imageAlt)
	s = htmlTagRegex.ReplaceAllStringFunc(s, stripTag)
	return html.UnescapeString(s)
}

func (r *renderer) renderHTMLBlock(n *ast.HTMLBlock) {
	var raw strings.Builder
	for i := 0; i < n.Lines().Len(); i++ {
		seg := n.Lines().At(i)
		raw.Write(seg.Value(r.src))
	}
	if n.HasClosure() {
		raw.Write(n.ClosureLine.Value(r.src))
	}
	emitted := false
	for line := range strings.SplitSeq(stripHTML(raw.String()), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			r.emit(Line{Kind: KindParagraph, Spans: []Span{{Text: line, Style: term.StylePaletteItem}}})
			emitted = true
		}
	}
	if emitted {
		r.emitBlank()
	}
}

func (r *renderer) renderRawHTML(n *ast.RawHTML, style term.Style, segs *inlineLines) {
	var raw strings.Builder
	for i := 0; i < n.Segments.Len(); i++ {
		seg := n.Segments.At(i)
		raw.Write(seg.Value(r.src))
	}
	tag := raw.String()
	switch {
	case htmlBreakRegex.MatchString(tag):
		*segs = append(*segs, nil)
	case htmlImageRegex.MatchString(tag):
		if alt := imageAlt(tag); alt != "" {
			segs.add(Span{Text: alt, Style: style})
		}
	}
}
