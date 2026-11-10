package ui

import "github.com/eugenioenko/ttt/internal/config"

func (g *EditorGroupWidget) SessionTabs() []config.EditorTabState {
	g.saveMultiState()
	var tabs []config.EditorTabState
	for i := range g.tabs {
		t := &g.tabs[i]
		if t.Content != nil || t.Buf == nil || t.Virtual || t.FilePath == "" {
			continue
		}
		tabs = append(tabs, config.EditorTabState{Path: t.FilePath, Line: t.Cur.Line, Col: t.Cur.Col,
			TopLine: t.Vp.TopLine, LeftCol: t.Vp.LeftCol, Pinned: i < g.pinnedCount, Active: i == g.active})
	}
	return tabs
}
