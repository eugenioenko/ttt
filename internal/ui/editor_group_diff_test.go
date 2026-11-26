package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/fileicons"
	"github.com/eugenioenko/ttt/internal/term"
)

func TestEditorGroupTabIconUsesFilePathInsteadOfCustomTitle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Dockerfile")
	group := NewEditorGroupWidget(nil, 4, false, "extended")
	group.TabBar.Icons = config.IconsNerdFont
	group.OpenDiffTab(path+" (diff)", "custom tab title", path, diff.FileDiff{}, nil, nil, false)
	group.syncTabs()

	group.TabBar.SetRect(Rect{W: 80, H: 3})
	grid := makeGrid(80, 3)
	group.TabBar.Render(NewRenderSurface(grid, Rect{W: 80, H: 3}))
	want := []rune(fileicons.ForFile("Dockerfile").Glyph)[0]
	for _, cell := range grid[1] {
		if cell.Ch == want && cell.Style == term.StyleFileIconBlue {
			return
		}
	}
	t.Fatalf("diff-tab icon for underlying path %q missing:\n%+v", path, group.TabBar.Tabs)
}

func TestEditorGroupDiffSearchSourceKeepsFileIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Dockerfile")
	group := NewEditorGroupWidget(nil, 4, false, "extended")
	group.OpenDiffTab(path+" (diff)", "custom title", path, diff.FileDiff{}, nil, nil, false)

	sources := group.DiffTabSources()
	if len(sources) != 1 {
		t.Fatalf("diff sources = %d, want 1", len(sources))
	}
	if sources[0].TabName != path+" (diff)" || sources[0].IconPath != path {
		t.Fatalf("diff source = %+v, want display ID and underlying file path", sources[0])
	}
}

func TestEditorGroupDiffTabLabelsPreserveViewType(t *testing.T) {
	name := "release_notes_for_the_terminal_tab_context_menu.md"
	cut := "release_notes_for_the_terminal_…"
	path := filepath.Join(t.TempDir(), name)
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	if err := g.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	check := func(want, fullName string, cut bool) {
		t.Helper()
		tb := g.TabBar
		tb.SetRect(Rect{W: 200, H: 3})
		grid := makeGrid(200, 3)
		tb.Render(NewRenderSurface(grid, Rect{W: 200, H: 3}))
		span := tb.tabSpans[g.active]
		if !strings.Contains(span.label.text, " "+want+" ") {
			t.Fatalf("label = %q, want %q", span.label.text, want)
		}
		if !strings.Contains(gridRow(grid, 1, 200), want) {
			t.Fatalf("rendered tab row is missing %q", want)
		}
		_, got, _, _, ok := tb.CutTabAt(span.start+1, 1)
		if ok != cut || (ok && got != fullName) {
			t.Fatalf("hover = (%q, %v), want (%q, %v)", got, ok, fullName, cut)
		}
	}
	check(cut, name, true)
	for _, suffix := range []string{" (diff)", " (staged)"} {
		g.OpenDiffTab(filepath.Join("notes", name)+suffix, "", path, diff.FileDiff{}, nil, nil, false)
		check(cut+suffix, name+suffix, true)
	}
	g.SwitchTab(0)
	if !g.EnableInlineDiff(path, nil) {
		t.Fatal("could not enable inline diff")
	}
	check(cut+" (diff)", name+" (diff)", true)
	g.DisableInlineDiff(path)
	check(cut, name, true)

	g.OpenDiffTab(path+" (diff)", name+" (diff)", path, diff.FileDiff{}, nil, nil, false)
	check(cut, name+" (diff)", true)
	g = NewEditorGroupWidget(nil, 4, false, "extended")
	if err := g.OpenFile(path + " (diff)"); err != nil {
		t.Fatal(err)
	}
	check(cut, name+" (diff)", true)
	g.OpenDiffTab("main.go (staged)", "", "main.go", diff.FileDiff{}, nil, nil, false)
	check("main.go (staged)", "main.go (staged)", false)
}
