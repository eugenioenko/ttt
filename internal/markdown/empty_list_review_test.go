package markdown

import (
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

func TestReviewEmptyListMarkerStaysInParagraph(t *testing.T) {
	for _, marker := range []string{"+", "*", "1.", "1)"} {
		t.Run(marker, func(t *testing.T) {
			fragment := "Use `\n" + marker + "\n` end."
			node := goldmark.New().Parser().Parse(text.NewReader([]byte(fragment)))
			spans := 0
			if err := ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
				if entering && n.Kind() == ast.KindCodeSpan {
					spans++
				}
				return ast.WalkContinue, nil
			}); err != nil || spans != 1 {
				t.Fatalf("Goldmark spans=%d, error=%v, want one span across an empty marker", spans, err)
			}
		})
	}
}

func TestReviewDetailsSpanAcrossEmptyListMarker(t *testing.T) {
	for _, marker := range []string{"+", "*", "1.", "1)"} {
		t.Run(marker, func(t *testing.T) {
			input := "<details>\n<summary>More</summary>\nUse `\n" + marker +
				"\n</details>\n`\nHidden.\n</details>\nOutside."
			doc := Parse(input)
			if len(doc.Details()) != 1 {
				t.Fatalf("details=%d, want1", len(doc.Details()))
			}
			collapsed := strings.Join(textLines(doc.Render()), "\n")
			if !strings.Contains(collapsed, "Outside.") || strings.Contains(collapsed, "Hidden.") {
				t.Errorf("collapsed=%q, want outside visible and inside hidden", collapsed)
			}
			inside := strings.Join(textLines(doc.Details()[0].Content.Render()), "\n")
			if !strings.Contains(inside, "Hidden.") || strings.Contains(inside, "Outside.") {
				t.Errorf("body=%q, want inside hidden and outside excluded", inside)
			}
		})
	}
}
