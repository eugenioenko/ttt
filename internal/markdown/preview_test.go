package markdown

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func TestRenderPreviewHeading(t *testing.T) {
	lines := RenderPreview("# Hello World", 80)
	// Should have blank line, heading line, blank line
	found := false
	for _, l := range lines {
		text := l.Text()
		if strings.Contains(text, "# Hello World") {
			found = true
			// Check that heading spans use StyleHoverBold
			for _, s := range l.Spans {
				if s.Style != term.StyleHoverBold {
					t.Errorf("expected StyleHoverBold for heading span %q, got %d", s.Text, s.Style)
				}
			}
		}
	}
	if !found {
		t.Error("expected heading text '# Hello World' in output")
	}
}

func TestRenderPreviewH2(t *testing.T) {
	lines := RenderPreview("## Subtitle", 80)
	found := false
	for _, l := range lines {
		if strings.Contains(l.Text(), "## Subtitle") {
			found = true
		}
	}
	if !found {
		t.Error("expected '## Subtitle' in output")
	}
}

func TestRenderPreviewBold(t *testing.T) {
	lines := RenderPreview("this is **bold** text", 80)
	found := false
	for _, l := range lines {
		for _, s := range l.Spans {
			if s.Text == "bold" && s.Style == term.StyleHoverBold {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected bold span with StyleHoverBold")
	}
}

func TestRenderPreviewItalic(t *testing.T) {
	lines := RenderPreview("this is *italic* text", 80)
	found := false
	for _, l := range lines {
		for _, s := range l.Spans {
			if s.Text == "italic" && s.Style == term.StyleHoverBold {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected italic span with StyleHoverBold")
	}
}

func TestRenderPreviewInlineCode(t *testing.T) {
	lines := RenderPreview("use `fmt.Println` here", 80)
	found := false
	for _, l := range lines {
		for _, s := range l.Spans {
			if s.Text == "fmt.Println" && s.Style == term.StyleHoverCode {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected inline code span with StyleHoverCode")
	}
}

func TestRenderPreviewCodeBlock(t *testing.T) {
	input := "before\n\n```go\nfunc main() {}\n```\n\nafter"
	lines := RenderPreview(input, 80)
	foundCode := false
	for _, l := range lines {
		if strings.Contains(l.Text(), "func main()") {
			foundCode = true
		}
	}
	if !foundCode {
		t.Error("expected code block content 'func main() {}' in output")
	}
}

func TestRenderPreviewUnorderedList(t *testing.T) {
	input := "- item one\n- item two\n- item three"
	lines := RenderPreview(input, 80)
	foundBullet := false
	for _, l := range lines {
		text := l.Text()
		if strings.Contains(text, "•") && strings.Contains(text, "item one") {
			foundBullet = true
		}
	}
	if !foundBullet {
		t.Error("expected bullet character in unordered list")
	}
}

func TestRenderPreviewOrderedList(t *testing.T) {
	input := "1. first\n2. second\n3. third"
	lines := RenderPreview(input, 80)
	foundOrdered := false
	for _, l := range lines {
		text := l.Text()
		if strings.Contains(text, "1.") && strings.Contains(text, "first") {
			foundOrdered = true
		}
	}
	if !foundOrdered {
		t.Error("expected numbered item in ordered list")
	}
}

func TestRenderPreviewBlockquote(t *testing.T) {
	input := "> This is a quote"
	lines := RenderPreview(input, 80)
	foundQuote := false
	for _, l := range lines {
		text := l.Text()
		if strings.Contains(text, "│") && strings.Contains(text, "This is a quote") {
			foundQuote = true
			// All spans in blockquote should be muted
			for _, s := range l.Spans {
				if s.Style != term.StyleMuted {
					t.Errorf("expected StyleMuted for blockquote span %q, got %d", s.Text, s.Style)
				}
			}
		}
	}
	if !foundQuote {
		t.Error("expected blockquote with '│' prefix")
	}
}

func TestRenderPreviewHorizontalRule(t *testing.T) {
	input := "above\n\n---\n\nbelow"
	lines := RenderPreview(input, 80)
	foundRule := false
	for _, l := range lines {
		text := l.Text()
		if strings.Contains(text, "────") {
			foundRule = true
			for _, s := range l.Spans {
				if strings.Contains(s.Text, "─") && s.Style != term.StyleMuted {
					t.Errorf("expected StyleMuted for horizontal rule, got %d", s.Style)
				}
			}
		}
	}
	if !foundRule {
		t.Error("expected horizontal rule with '─' characters")
	}
}

func TestRenderPreviewLink(t *testing.T) {
	input := "see [docs](https://example.com) here"
	lines := RenderPreview(input, 80)
	foundLink := false
	foundURL := false
	for _, l := range lines {
		for _, s := range l.Spans {
			if s.Text == "docs" {
				foundLink = true
			}
			if strings.Contains(s.Text, "https://example.com") && s.Style == term.StyleMuted {
				foundURL = true
			}
		}
	}
	if !foundLink {
		t.Error("expected link text 'docs' in output")
	}
	if !foundURL {
		t.Error("expected URL in muted style")
	}
}

func TestRenderPreviewWordWrap(t *testing.T) {
	long := strings.Repeat("word ", 30) // 150 chars
	lines := RenderPreview(long, 80)
	if len(lines) < 2 {
		t.Errorf("expected word wrapping to produce multiple lines, got %d", len(lines))
	}
	// Each line should be at most 80 runes
	for i, l := range lines {
		text := l.Text()
		if len([]rune(text)) > 80 {
			t.Errorf("line %d exceeds 80 chars: %d", i, len([]rune(text)))
		}
	}
}

func TestRenderPreviewEmpty(t *testing.T) {
	lines := RenderPreview("", 80)
	// Should produce at least an empty line, not panic
	if lines == nil {
		t.Error("expected non-nil result for empty input")
	}
}

func TestRenderPreviewMultipleParagraphs(t *testing.T) {
	input := "First paragraph.\n\nSecond paragraph."
	lines := RenderPreview(input, 80)
	foundFirst := false
	foundSecond := false
	for _, l := range lines {
		text := l.Text()
		if strings.Contains(text, "First paragraph") {
			foundFirst = true
		}
		if strings.Contains(text, "Second paragraph") {
			foundSecond = true
		}
	}
	if !foundFirst {
		t.Error("expected first paragraph")
	}
	if !foundSecond {
		t.Error("expected second paragraph")
	}
	// There should be blank lines separating them
	if len(lines) < 3 {
		t.Errorf("expected at least 3 lines (2 paragraphs + separator), got %d", len(lines))
	}
}
