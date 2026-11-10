package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
)

func (a *App) editorSessionKey() string {
	if a.Workspace == nil {
		return ""
	}
	if a.Workspace.FilePath != "" {
		return "workspace:" + filepath.Clean(a.Workspace.FilePath)
	}
	paths := a.Workspace.Paths()
	if len(paths) == 0 {
		return ""
	}
	slices.Sort(paths)
	key, _ := json.Marshal(paths)
	return string(key)
}

func (a *App) editorSessionTabs() []config.EditorTabState {
	return a.EditorGroup.SessionTabs()
}

func (a *App) persistEditorSession() {
	if !a.Settings.Editor.RestoreTabs || a.sessionSavingPaused || a.sessionWorkspaceKey == "" {
		return
	}
	var tabs []config.EditorTabState
	for _, tab := range a.editorSessionTabs() {
		if info, err := os.Stat(tab.Path); err == nil && info.Mode().IsRegular() {
			tabs = append(tabs, tab)
		}
	}
	if err := config.SaveEditorSession(a.sessionWorkspaceKey, tabs); err != nil {
		a.StatusError(i18n.Format("session.saveFailed", i18n.Args{"error": err.Error()}))
	}
}

func (a *App) updateEditorSessionWorkspace() {
	a.persistEditorSession()
	a.sessionWorkspaceKey = a.editorSessionKey()
}

func (a *App) restoreEditorSession() {
	a.sessionWorkspaceKey = a.editorSessionKey()
	if !a.Settings.Editor.RestoreTabs || a.sessionWorkspaceKey == "" {
		return
	}
	if err := config.PruneEditorSessions(time.Now()); err != nil {
		a.StatusError(i18n.Format("session.cleanupFailed", i18n.Args{"error": err.Error()}))
	}
	if !a.restoreSessionOnStartup {
		return
	}
	tabs, err := config.LoadEditorSession(a.sessionWorkspaceKey)
	if err != nil {
		a.StatusError(i18n.Format("session.restoreFailed", i18n.Args{"error": err.Error()}))
		return
	}
	active, seen := "", map[string]bool{}
	for _, tab := range tabs {
		if !filepath.IsAbs(tab.Path) || seen[tab.Path] {
			continue
		}
		seen[tab.Path] = true
		if info, err := os.Stat(tab.Path); err != nil || !info.Mode().IsRegular() {
			continue
		}
		g := a.EditorGroup
		if err := g.OpenFile(tab.Path); err != nil {
			continue
		}
		g.CommitActiveTab()
		g.GoToLineCol(max(tab.Line, 0)+1, max(tab.Col, 0)+1)
		if buf := g.ActiveBuffer(); buf != nil {
			g.Editor.Viewport.TopLine = max(0, min(tab.TopLine, len(buf.Lines)-1))
			g.Editor.Viewport.LeftCol = max(tab.LeftCol, 0)
		}
		if tab.Pinned {
			g.TogglePinTab()
		}
		if tab.Active {
			active = tab.Path
		}
	}
	if active != "" {
		a.EditorGroup.SwitchToTabByPath(active)
	}
	if len(seen) > 0 {
		a.FocusEditor()
		a.persistEditorSession()
	}
}

func (a *App) ClearEditorSessions() {
	if err := config.ClearEditorSessions(); err != nil {
		a.StatusError(i18n.Format("session.cleanupFailed", i18n.Args{"error": err.Error()}))
		return
	}
	a.sessionSavingPaused = true
	a.StatusNotify(i18n.Text("session.cleared"))
}
