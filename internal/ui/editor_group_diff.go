package ui

import (
	"github.com/eugenioenko/ttt/internal/core/diff"
)

type inlineDiffState struct {
	Base    []string
	Overlay *DiffOverlay
}

func (s *inlineDiffState) overlay() *DiffOverlay {
	if s == nil {
		return nil
	}
	return s.Overlay
}

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
	t.InlineDiff = &inlineDiffState{
		Base:    base,
		Overlay: NewDiffOverlay(diff.FullDiffLines(base, t.Buf.Lines)),
	}
	if i == g.active {
		g.syncTabs()
	}
	return true
}

func (g *EditorGroupWidget) DisableInlineDiff(path string) {
	i := g.tabIndexByPath(path)
	if i < 0 || g.tabs[i].InlineDiff == nil {
		return
	}
	g.tabs[i].InlineDiff = nil
	if i == g.active {
		g.syncTabs()
	}
}

func (g *EditorGroupWidget) SetInlineDiff(path string, base []string, overlay *DiffOverlay) {
	i := g.tabIndexByPath(path)
	if i < 0 || g.tabs[i].InlineDiff == nil {
		return
	}
	g.tabs[i].InlineDiff.Base = base
	g.tabs[i].InlineDiff.Overlay = overlay
	if i == g.active {
		g.Editor.SetDiffOverlay(overlay)
	}
}

func (g *EditorGroupWidget) IsInlineDiffPath(path string) bool {
	i := g.tabIndexByPath(path)
	return i >= 0 && g.tabs[i].InlineDiff != nil
}

func (g *EditorGroupWidget) IsInlineDiffActive() bool {
	t := g.activeTab()
	return t != nil && t.Content == nil && t.InlineDiff != nil
}
