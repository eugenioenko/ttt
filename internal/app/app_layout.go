package app

import (
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) ShowSidebar() {
	a.showSidebar()
	a.persistSidebarLayout()
}

func (a *App) showSidebar() {
	a.Sidebar.Visible = true
	a.SplitPanel.ShowLeft = true
	if a.SplitPanel.DividerPos < ui.MinSidebarWidth {
		a.SplitPanel.DividerPos = ui.DefaultSidebarWidth
	}
	a.applySearchHighlights()
	a.syncRepositoryObservation()
}

func (a *App) HideSidebar() {
	a.hideSidebar()
	a.persistSidebarLayout()
}

func (a *App) hideSidebar() {
	// Else keys keep going to the hidden panel. Checked before Visible flips.
	if a.focusedRegion() == "sidebar" {
		a.FocusEditor()
	}
	a.Sidebar.InvalidatePointerInteraction()
	a.Sidebar.Visible = false
	a.SplitPanel.ShowLeft = false
	a.EditorGroup.ClearSearch()
	a.syncRepositoryObservation()
}

func (a *App) applySearchHighlights() {
	if !a.EditorGroup.IsEditorActive() {
		return
	}
	if a.Sidebar.ActivePanel == "search" && a.Search.Input.Text() != "" {
		matches, _ := ui.FindInLines(a.EditorGroup.Editor.Buf.Lines, a.Search.Input.Text(), a.Search.Options)
		a.EditorGroup.SetSearch(a.Search.Input.Text(), a.Search.Options, matches)
	} else {
		a.EditorGroup.Editor.RefreshSearch()
	}
}

func (a *App) ShowPanel(id string, widget ui.Widget) {
	a.Sidebar.SetActivePanel(id)
	if !a.Sidebar.Visible {
		a.ShowSidebar()
	}
	a.Root.SetFocus(widget)
}

func (a *App) ToggleSidebar() {
	if a.Sidebar.Visible {
		a.HideSidebar()
	} else {
		a.ShowSidebar()
	}
}

func (a *App) SetSidebarWidth(w int) {
	a.resizeSidebar(w)
	a.persistSidebarLayout()
}

func (a *App) resizeSidebar(w int) {
	if w <= 0 {
		a.hideSidebar()
		return
	}
	if !a.Sidebar.Visible {
		a.showSidebar()
	}
	a.SplitPanel.DividerPos = w
}

// Widths below MinSidebarWidth are not saved: dragging the sidebar closed
// passes through them, and restoring one would reopen it as a sliver.
func (a *App) persistSidebarLayout() {
	next := a.State
	next.SidebarHidden = !a.Sidebar.Visible
	if w := a.SplitPanel.DividerPos; w >= ui.MinSidebarWidth {
		next.SidebarWidth = w
	}
	if next.SidebarHidden == a.State.SidebarHidden && next.SidebarWidth == a.State.SidebarWidth {
		return
	}
	a.State = next
	if err := config.SaveState(a.State); err != nil {
		a.StatusError(i18n.Format("status.failedToSaveSidebarLayout", i18n.Args{"error": err.Error()}))
	}
}

func (a *App) FocusSidebar() {
	if !a.Sidebar.Visible {
		a.ShowSidebar()
	}
	if w := a.Sidebar.ActiveWidget(); w != nil {
		a.Root.SetFocus(w)
	}
}

// The panel's new rect only exists after the next render, so the terminal
// resize rides along with the caller's redraw.
func (a *App) SetPanelPosition(pos ui.SplitPosition) {
	if a.ContentSplit.Position == pos {
		return
	}
	a.ContentSplit.Position = pos

	name := "bottom"
	if pos == ui.SplitRight {
		name = "right"
	}
	a.State.PanelPosition = name
	if err := config.SaveState(a.State); err != nil {
		a.StatusError(i18n.Format("status.failedToSavePanelPosition", i18n.Args{"error": err.Error()}))
	}
}

func (a *App) ensureUsablePanelSize() {
	r := a.ContentSplit.GetRect()
	maxH := r.H - 4
	if a.ContentSplit.BottomH < ui.MinPanelHeight || a.ContentSplit.BottomH > maxH {
		a.ContentSplit.BottomH = min(r.H/2, maxH)
	}
	if a.ContentSplit.RightW < ui.MinPanelWidth {
		a.ContentSplit.RightW = ui.DefaultPanelWidth
	}
}

func (a *App) resizePanel(size int) {
	if !a.panelResizing {
		a.panelResizing = true
		a.panelResizeFromClosed = !a.ContentSplit.ShowBottom
	}
	if size <= 0 {
		a.ContentSplit.ShowBottom = false
		return
	}
	a.ContentSplit.ShowBottom = true
	if a.ContentSplit.Position == ui.SplitRight {
		a.ContentSplit.RightW = size
	} else {
		a.ContentSplit.BottomH = size
	}
	resizeTerminals(a)
}

// Spawning a terminal moves focus, and a focus change cancels the pointer
// capture, so it must wait until the drag is over.
func (a *App) finishPanelResize() {
	fromClosed := a.panelResizeFromClosed
	a.panelResizing = false
	a.panelResizeFromClosed = false
	if fromClosed && a.ContentSplit.ShowBottom && len(a.Terminals) == 0 {
		a.SpawnTerminal()
	}
	a.persistPanelSize()
}

func (a *App) persistPanelSize() {
	next := a.State
	if h := a.ContentSplit.BottomH; h >= ui.MinPanelHeight {
		next.PanelHeight = h
	}
	if w := a.ContentSplit.RightW; w >= ui.MinPanelWidth {
		next.PanelWidth = w
	}
	if next.PanelHeight == a.State.PanelHeight && next.PanelWidth == a.State.PanelWidth {
		return
	}
	a.State = next
	if err := config.SaveState(a.State); err != nil {
		a.StatusError(i18n.Format("status.failedToSavePanelSize", i18n.Args{"error": err.Error()}))
	}
}

func (a *App) ShowBottomPanel() {
	a.ContentSplit.ShowBottom = true
}

func (a *App) showTerminalPanel() {
	a.ContentSplit.ShowBottom = true
	if len(a.Terminals) == 0 {
		a.SpawnTerminal()
	} else {
		a.BottomPanel.SetActivePanel("terminal")
		a.Root.SetFocus(a.TerminalPanel)
	}
}

func (a *App) HideBottomPanel() {
	a.ContentSplit.ShowBottom = false
	a.FocusEditor()
}

func (a *App) ToggleBottomPanel() {
	if a.ContentSplit.ShowBottom {
		a.HideBottomPanel()
	} else {
		a.ShowBottomPanel()
	}
}
