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
	htmlImageRegex   = regexp.MustCompile(`(?i)<img\b(?:[^>"']|"[^"]*"|'[^']*')*>`)
	htmlAltRegex     = regexp.MustCompile(`(?i)(?:^|\s)alt\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	// A tag needs a name right after < so literal comparisons like 1 < 2 > 1
	// survive; quoted attribute values may contain >.
	htmlTagNameRegex = regexp.MustCompile(`</?[A-Za-z][A-Za-z0-9-]*(?:\s(?:[^>"']|"[^"]*"|'[^']*')*)?/?>`)
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

// rawImageAlt keeps entities encoded, so stripHTML can remove tags before
// decoding once and an encoded "&lt;b&gt;" in alt text survives.
func rawImageAlt(tag string) string {
	m := htmlAltRegex.FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	return m[1] + m[2]
}

func stripHTML(raw string) string {
	s := htmlCommentRegex.ReplaceAllString(raw, "")
	s = htmlBreakRegex.ReplaceAllString(s, "\n")
	s = htmlImageRegex.ReplaceAllStringFunc(s, rawImageAlt)
	s = htmlTagNameRegex.ReplaceAllStringFunc(s, stripTag)
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
	if strings.HasPrefix(tag, "<!--") {
		return
	}
	switch {
	case htmlBreakRegex.FindString(tag) == tag:
		*segs = append(*segs, nil)
	case htmlImageRegex.FindString(tag) == tag:
		if alt := html.UnescapeString(rawImageAlt(tag)); alt != "" {
			segs.add(Span{Text: alt, Style: style})
		}
	}
}
