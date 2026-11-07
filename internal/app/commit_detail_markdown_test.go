package app

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
)

func markdownDetailText(d *ui.CommitDetailWidget) string {
	r := ui.Rect{W: 100, H: 30}
	d.SetRect(r)
	cells := make([][]term.Cell, r.H)
	for y := range cells {
		cells[y] = make([]term.Cell, r.W)
	}
	d.Render(ui.NewRenderSurface(cells, r))
	var text strings.Builder
	for _, row := range cells {
		for _, cell := range row {
			text.WriteRune(cell.Ch)
		}
		text.WriteByte('\n')
	}
	return text.String()
}

func TestCommitDetailPRMarkdownAndSettings(t *testing.T) {
	settings := config.DefaultSettings()
	settings.Markdown.WrapWidth = 12
	a := buildTestApp(t, settings)
	body := "**bold** alpha beta gamma"
	a.Changes.AddPRGroup("PR #1: Example", "https://github.com/owner/repo/pull/1", "owner", "repo", "base", "head", body, nil, nil)
	group := a.Changes.toUIChangesGroup(&a.Changes.PRGroups[0])
	a.OpenPRDetail(group)
	detail := a.EditorGroup.ActiveCommitDetailWidget()
	if detail == nil || detail.Message != body || !strings.Contains(detail.Metadata, "PR #1: Example") {
		t.Fatal("PR description or metadata was lost between group and detail")
	}
	if text := markdownDetailText(detail); !strings.Contains(text, "bold alpha") || strings.Contains(text, "**bold**") {
		t.Fatalf("initial Markdown configuration not applied:\n%s", text)
	}
	settings.Markdown.WrapWidth = 5
	a.ApplySettings(settings)
	if text := markdownDetailText(detail); strings.Contains(text, "bold alpha") {
		t.Fatalf("open PR did not rewrap after settings update:\n%s", text)
	}
	a.OpenPRDetail(group)
	if a.EditorGroup.ActiveCommitDetailWidget() != detail {
		t.Fatal("reopening PR replaced its detail widget")
	}
	group.PRURL += "0"
	group.PRBody = ""
	a.OpenPRDetail(group)
	empty := a.EditorGroup.ActiveCommitDetailWidget()
	empty.SetMarkdownMaxWidth(80)
	if text := markdownDetailText(empty); !strings.Contains(text, "(No pull request description)") || strings.Contains(text, "(No commit message)") {
		t.Fatalf("wrong empty PR description:\n%s", text)
	}
}
