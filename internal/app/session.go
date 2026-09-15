package app

import (
	"log/slog"
	"os"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/session"
)

// sessionKey resolves the directory a session belongs to: the primary
// workspace folder, or the cwd when only files were opened (file-only
// launches intentionally create no workspace).
func (a *App) sessionKey() string {
	if a.Workspace != nil {
		if key := a.Workspace.Primary(); key != "" {
			return key
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return ""
}

// SaveSession persists open file tabs for the session key (primary workspace
// folder, else cwd). It is a no-op unless auto sessions are on, and never
// overwrites a session with an empty tab list.
func (a *App) SaveSession() {
	if a.Settings == nil || !a.Settings.Session.Auto || a.Workspace == nil {
		return
	}
	key := a.sessionKey()
	if key == "" {
		return
	}
	states, active := a.EditorGroup.FileTabStates()
	if len(states) == 0 {
		return
	}
	tabs := make([]session.TabState, 0, len(states))
	for _, st := range states {
		tabs = append(tabs, session.TabState{Path: st.Path, Line: st.Line, Col: st.Col})
	}
	s := session.Session{Folders: a.Workspace.Paths(), Tabs: tabs, Active: active}
	if err := session.Save(config.ConfigDir(), key, s); err != nil {
		slog.Warn("session: save failed", "error", err)
	}
}

// RestoreSession reopens the tabs saved for the session key and reactivates
// the saved tab. Missing files and folders are skipped. It reports whether
// anything was restored.
func (a *App) RestoreSession() bool {
	if a.Settings == nil || !a.Settings.Session.Auto || a.Workspace == nil {
		return false
	}
	key := a.sessionKey()
	if key == "" {
		return false
	}
	sess, err := session.Load(config.ConfigDir(), key)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("session: load failed", "error", err)
		}
		return false
	}
	for _, f := range sess.Folders {
		if f == "" {
			continue
		}
		if st, err := os.Stat(f); err != nil || !st.IsDir() {
			continue
		}
		a.Workspace.AddFolder(f)
	}
	opened := make(map[string]bool, len(sess.Tabs))
	for _, t := range sess.Tabs {
		if opened[t.Path] {
			continue
		}
		if st, err := os.Stat(t.Path); err != nil || st.IsDir() {
			continue
		}
		a.EditorGroup.OpenFile(t.Path)
		if a.EditorGroup.BufferForPath(t.Path) == nil {
			continue
		}
		// Fresh opens are preview tabs that would replace each other;
		// commit so every restored file keeps its own tab.
		a.EditorGroup.CommitActiveTab()
		a.EditorGroup.SetFileCursor(t.Path, t.Line, t.Col)
		opened[t.Path] = true
	}
	if sess.Active != "" && opened[sess.Active] {
		a.EditorGroup.SwitchToTabByPath(sess.Active)
	}
	restored := len(opened) > 0
	if restored {
		a.EditorGroup.ScrollToCursor()
	}
	return restored
}
