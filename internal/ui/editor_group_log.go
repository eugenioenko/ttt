package ui

import (
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/view"
)

func (g *EditorGroupWidget) ActiveLogViewer() *LogViewerWidget {
	t := g.activeTab()
	if t == nil {
		return nil
	}
	v, _ := t.Content.(*LogViewerWidget)
	return v
}
func (g *EditorGroupWidget) HasLogTab(path string) bool {
	for _, t := range g.tabs {
		if t.FilePath == path {
			_, ok := t.Content.(*LogViewerWidget)
			return ok
		}
	}
	return false
}
func (g *EditorGroupWidget) OpenLogTab(path string, viewer *LogViewerWidget) {
	for i, t := range g.tabs {
		if t.FilePath != path {
			continue
		}
		if g.OnFileClose != nil && t.Highlighter != nil && !t.Virtual {
			g.OnFileClose(path, t.Highlighter.Language())
		}
		g.notifyContentTabClose(t)
		g.tabs[i] = editorTab{ID: t.ID, FilePath: path, IconPath: path, Title: filepath.Base(path) + " (log)", Content: viewer, ReadOnly: true, Virtual: true}
		g.SwitchTab(i)
		g.syncTabs()
		return
	}
	g.openContentFileTab(path, viewer)
	g.CommitActiveTab()
}

func (g *EditorGroupWidget) wrapsTab(t *editorTab) bool {
	return g.WordWrap && (g.WordWrapMaxBytes <= 0 || t.Buf == nil || t.Buf.DiskSize() <= g.WordWrapMaxBytes)
}
func (g *EditorGroupWidget) ActiveFileWordWrap() bool {
	if t := g.activeTab(); t != nil && t.Content == nil {
		return g.wrapsTab(t)
	}
	return g.WordWrap
}

func (g *EditorGroupWidget) detachLogEditor() {
	e := g.Editor
	if g.FoldTask != nil {
		g.FoldTask.Stop()
	}
	if g.FoldDelay != nil {
		g.FoldDelay.Stop()
	}
	e.mouseDown = false
	e.cancelDragAutoScroll()
	e.Buf = &buffer.Buffer{Lines: []string{""}}
	e.Cursor = &cursor.Cursor{}
	e.Viewport = &view.Viewport{}
	e.Selection = &selection.Selection{}
	e.Undo = nil
	e.Multi = nil
	e.Highlighter = nil
	e.Folds = nil
	e.Diagnostics = nil
	e.LineChanges = nil
	e.SearchMatches = nil
	e.searchByLine = nil
	e.diagByLine = nil
	e.layoutCache = [4]*rowLayout{}
	e.gitChanges = gitChangeIndex{}
	e.bracketMatchCache = bracketMatch{}
	e.bracketColorCache = nil
	e.runeScratch = nil
	e.lineScratch = nil
	e.rowMap = nil
	e.SetDiffOverlay(nil)
	e.ReadOnly = true
	e.WordWrap = false
	e.bufferDirty = false
	g.find.matchesBuffer = nil
}
