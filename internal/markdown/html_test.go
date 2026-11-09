package markdown

import (
	"slices"
	"strings"
	"testing"
)

func TestRenderDropsHTMLComments(t *testing.T) {
	input := "<!-- ttt-pr-binaries -->\n\n## Binaries\n\n<!-- internal state start -->\n<!--\nYmFzZTY0\n-->\n<!-- internal state end -->\n\nBuilt from a0b2558."
	got := textLines(Render(input))
	want := []string{"## Binaries", "", "Built from a0b2558."}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderDropsInlineHTMLComment(t *testing.T) {
	got := textLines(Render("before <!-- hidden --> after"))
	if len(got) != 1 || got[0] != "before  after" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderImageTagAsAltText(t *testing.T) {
	input := `<a href="https://example.com"><picture><source media="(prefers-color-scheme: dark)" srcset="dark.svg"><img alt="Review change stack" src="light.svg"></picture></a>`
	got := textLines(Render(input))
	if len(got) != 1 || got[0] != "Review change stack" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderHTMLBlockKeepsTextWithoutTags(t *testing.T) {
	input := "<dl>\n<dd>First &amp; second</dd>\n<dd><strong>Bold</strong> text<br/>next line</dd>\n</dl>"
	got := textLines(Render(input))
	want := []string{"First & second", "Bold text", "next line"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestRenderInlineTagsKeepText(t *testing.T) {
	lines := Render("Some <strong>bold</strong> and <sup>raised</sup> text<br>second line")
	got := textLines(lines)
	want := []string{"Some bold and raised text", "second line"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if lines[0].Kind != KindParagraph {
		t.Fatalf("expected a paragraph, got %v", lines[0].Kind)
	}
}

func TestRenderKeepsHTMLInsideCode(t *testing.T) {
	got := strings.Join(textLines(Render("Use `<!-- x -->` here.\n\n```html\n<!-- keep -->\n<br>\n```")), "\n")
	for _, s := range []string{"<!-- x -->", "<!-- keep -->", "<br>"} {
		if !strings.Contains(got, s) {
			t.Fatalf("expected %q to stay literal in code, got %q", s, got)
		}
	}
}
