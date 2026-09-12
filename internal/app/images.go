package app

import (
	"log/slog"

	"github.com/eugenioenko/ttt/internal/term"
)

// Call next to every renderer.Clear.
func (a *App) invalidateImageLayer() {
	if a.ImageLayer != nil {
		a.ImageLayer.Invalidate()
	}
}

// Zero pixels per cell (no tty, or a failed query) disables graphics.
func (a *App) RefreshImageCellSize() {
	w, h := 0, 0
	if a.Screen != nil {
		if tty, ok := a.Screen.Tty(); ok && tty != nil {
			if ws, err := tty.WindowSize(); err == nil {
				w, h = ws.CellDimensions()
			}
		}
	}
	a.imageCellW, a.imageCellH = w, h
	if a.EditorGroup != nil {
		a.EditorGroup.SetImageCellSize(w, h)
	}
}

// Must run after the text grid is rendered.
func (a *App) commitImageLayer(screen *term.TcellScreen) bool {
	if a.ImageLayer == nil || screen == nil {
		return false
	}
	tty, ok := screen.Tty()
	if !ok || tty == nil {
		return false
	}
	if err := a.ImageLayer.Commit(tty); err != nil {
		slog.Warn("image commit failed", "err", err)
	}
	return true
}

func (a *App) CloseImageLayer() {
	if a.ImageLayer == nil || a.Screen == nil {
		return
	}
	tty, ok := a.Screen.Tty()
	if !ok || tty == nil {
		return
	}
	if err := a.ImageLayer.Close(tty); err != nil {
		slog.Warn("image close failed", "err", err)
	}
}
