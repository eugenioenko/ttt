package ui

import (
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/fold"
)

func (g *EditorGroupWidget) tabIndexByPath(path string) int {
	for i := range g.tabs {
		if g.tabs[i].FilePath == path && g.tabs[i].Content == nil && g.tabs[i].Buf != nil {
			return i
		}
	}
	return -1
}

func (g *EditorGroupWidget) EnableInlineDiff(path string, base []string) bool {
	i := g.tabIndexByPath(path)
	if i < 0 {
		return false
	}
	t := &g.tabs[i]
	t.Preview = false
	d := NewEditableDiffWidget(t.FilePath, base, diff.FullDiffLines(base, t.Buf.Lines))
	d.SetSyntaxHighlight(g.SyntaxHighlight)
	g.ApplyDiffDefaults(d)
	t.Diff = d
	if i == g.active {
		g.syncTabs()
	}
	return true
}

func (g *EditorGroupWidget) DisableInlineDiff(path string) {
	i := g.tabIndexByPath(path)
	if i < 0 || g.tabs[i].Diff == nil {
		return
	}
	t := &g.tabs[i]
	t.Diff = nil
	if t.Folds != nil {
		t.Folds.SetRanges(fold.ComputeIndentRanges(t.Buf.Lines))
	}
	if i == g.active {
		g.Editor.SetDiffOverlay(nil)
		g.syncTabs()
	}
}

func (g *EditorGroupWidget) SetInlineDiff(path string, base []string, lines []diff.DiffLine, ver uint64, snap []string) {
	i := g.tabIndexByPath(path)
	if i < 0 || g.tabs[i].Diff == nil {
		return
	}
	g.tabs[i].Diff.SetLiveDiff(base, lines, ver, snap)
}

func (g *EditorGroupWidget) IsInlineDiffPath(path string) bool {
	i := g.tabIndexByPath(path)
	return i >= 0 && g.tabs[i].Diff != nil
}

func (g *EditorGroupWidget) IsInlineDiffActive() bool {
	return g.activeInlineDiff() != nil
}

func (g *EditorGroupWidget) ActiveInlineDiff() *DiffEditorWidget {
	return g.activeInlineDiff()
}

func (g *EditorGroupWidget) activeInlineDiff() *DiffEditorWidget {
	t := g.activeTab()
	if t == nil || t.Content != nil {
		return nil
	}
	return t.Diff
}
