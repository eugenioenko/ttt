package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
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
	var tabs []config.EditorTabState
	seen := map[string]int{}
	var collect func(ui.Widget)
	collect = func(w ui.Widget) {
		if group, ok := w.(*ui.EditorGroupWidget); ok {
			for _, tab := range group.SessionTabs() {
				path, err := filepath.Abs(tab.Path)
				if err != nil {
					continue
				}
				tab.Path = path
				tab.Active = tab.Active && group == a.EditorGroup
				if i, exists := seen[tab.Path]; exists {
					pinned := tabs[i].Pinned || tab.Pinned
					if tab.Active {
						tabs[i] = tab
					}
					tabs[i].Pinned = pinned
					continue
				}
				seen[tab.Path] = len(tabs)
				tabs = append(tabs, tab)
			}
		} else if container, ok := w.(widgets.ContainerWidget); ok {
			for _, child := range container.WidgetChildren() {
				collect(child)
			}
		}
	}
	collect(a.ContentSplit.Top)
	return tabs
}

func (a *App) persistEditorSession() {
	if !a.Settings.Editor.RestoreTabs || a.sessionSavingPaused || a.sessionHasExplicitFiles || a.sessionWorkspaceKey == "" {
		return
	}
	session := config.EditorSessionState{}
	if a.EditorSplit != nil && a.ContentSplit.Top == a.EditorSplit {
		session.Layout = a.snapshotEditorSessionLayout(a.EditorSplit)
	} else {
		for _, tab := range a.editorSessionTabs() {
			if info, err := os.Stat(tab.Path); err == nil && info.Mode().IsRegular() {
				session.Tabs = append(session.Tabs, tab)
			}
		}
	}
	if err := config.SaveEditorWorkspaceSession(a.sessionWorkspaceKey, session); err != nil {
		a.StatusError(i18n.Format("session.saveFailed", i18n.Args{"error": err.Error()}))
	}
}

func (a *App) updateEditorSessionWorkspace() {
	a.persistEditorSession()
	key := a.editorSessionKey()
	if key != a.sessionWorkspaceKey {
		a.sessionHasExplicitFiles = false
	}
	a.sessionWorkspaceKey = key
}

func (a *App) restoreEditorSession() {
	a.sessionWorkspaceKey = a.editorSessionKey()
	if !a.Settings.Editor.RestoreTabs || a.sessionHasExplicitFiles || a.sessionWorkspaceKey == "" {
		return
	}
	if err := config.PruneEditorSessions(time.Now()); err != nil {
		a.StatusError(i18n.Format("session.cleanupFailed", i18n.Args{"error": err.Error()}))
	}
	session, err := config.LoadEditorWorkspaceSession(a.sessionWorkspaceKey)
	if err != nil {
		a.StatusError(i18n.Format("session.restoreFailed", i18n.Args{"error": err.Error()}))
		return
	}
	if session.Layout != nil {
		a.restoreEditorSessionLayout(session.Layout)
	} else {
		a.restoreEditorGroupTabs(a.EditorGroup, session.Tabs)
	}
	if session.Layout != nil || len(session.Tabs) > 0 {
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
