package ui

import (
	"github.com/eugenioenko/ttt/internal/i18n"
	"path/filepath"
	"slices"

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
		g.notifyContentTabClose(t)
		g.tabs[i] = editorTab{ID: t.ID, FilePath: path, IconPath: path, Title: i18n.Format("logMode.tabTitle", i18n.Args{"filename": filepath.Base(path)}), Content: viewer, ReadOnly: true, Virtual: true}
		g.SwitchTab(i)
		g.syncTabs()
		return
	}
	g.openContentFileTab(path, viewer)
	g.CommitActiveTab()
}

func (g *EditorGroupWidget) detachLogEditor() {
	e := g.Editor
	if g.FoldTask != nil {
		g.FoldTask.Stop()
	}
	if g.FoldDelay != nil {
		g.FoldDelay.Stop()
	}
	g.foldBuf = nil
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
	e.lineScratch = nil
	e.rowMap = nil
	e.SetDiffOverlay(nil)
	e.autoClosing = nil
	e.goalContext = cursorGoalContext{}
	e.links = editorLinks{generation: e.links.generation + 1, detect: e.links.detect}
	e.ReadOnly = true
	e.WordWrap = false
	e.bufferDirty = false
	g.find.matchesBuffer = nil
}

func (g *EditorGroupWidget) reuseLogDocument(path string) bool {
	oldIndex := -1
	for i, tab := range g.tabs {
		if tab.FilePath == path {
			if _, ok := tab.Content.(*LogViewerWidget); ok {
				oldIndex = i
				break
			}
		}
	}
	if oldIndex < 0 {
		return false
	}
	old := g.tabs[oldIndex]
	for _, source := range g.relatedGroups() {
		if source == g || !g.OpenSharedFileFrom(source, path) {
			continue
		}
		replacement := g.tabs[g.active]
		g.tabs = slices.Delete(g.tabs, g.active, g.active+1)
		g.notifyContentTabClose(old)
		replacement.ID, replacement.Preview = old.ID, old.Preview
		g.tabs[oldIndex] = replacement
		g.active = oldIndex
		g.syncTabs()
		return true
	}
	return false
}
