package app

import (
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func (a *App) Copy() {
	if tw := a.focusedTerminalWidget(); tw != nil {
		tw.CopySelection()
		return
	}
	if sel := a.EditorGroup.ActiveTextSelection(); sel != nil && sel.CopyTextSelection() {
		return
	}
	if holder, ok := a.Root.Focused.(ui.InputHolder); ok {
		if inp := holder.FocusedInput(); inp != nil {
			inp.CopySelection()
			return
		}
	}
	if inp := a.focusedWidgetInput(); inp != nil {
		inp.CopySelection()
		return
	}
	a.EditorGroup.Copy()
}

func (a *App) Cut() {
	if tw := a.focusedTerminalWidget(); tw != nil {
		tw.CopySelection()
		return
	}
	if holder, ok := a.Root.Focused.(ui.InputHolder); ok {
		if inp := holder.FocusedInput(); inp != nil {
			inp.CutSelection()
			return
		}
	}
	if inp := a.focusedWidgetInput(); inp != nil {
		inp.CutSelection()
		return
	}
	a.EditorGroup.Cut()
}

func (a *App) Paste() {
	if holder, ok := a.Root.TopOverlayWidget().(ui.InputHolder); ok {
		if inp := holder.FocusedInput(); inp != nil {
			inp.PasteClipboard()
			return
		}
	}
	if tw := a.focusedTerminalWidget(); tw != nil {
		text := clipboard.Get()
		if text != "" {
			tw.PasteText(text)
		} else {
			tw.ClearSelection()
		}
		return
	}
	if holder, ok := a.Root.Focused.(ui.InputHolder); ok {
		if inp := holder.FocusedInput(); inp != nil {
			inp.PasteClipboard()
			return
		}
	}
	if inp := a.focusedWidgetInput(); inp != nil {
		inp.PasteText(clipboard.Get())
		return
	}
	a.EditorGroup.Paste()
}

// focusedWidgetInput returns the widgets.InputWidget with keyboard focus
// inside a focused widget adapter or plugin editor tab, or nil. Copy, Cut
// and Paste use it so clipboard commands act on that input, not the editor.
func (a *App) focusedWidgetInput() *widgets.InputWidget {
	var w widgets.Widget
	switch f := a.Root.Focused.(type) {
	case *ui.WidgetAdapter:
		w = f.FocusedWidget()
	case *ui.EditorGroupWidget:
		w = f.TabContent(f.ActiveTabIndex())
	}
	switch w := w.(type) {
	case *widgets.InputWidget:
		return w
	case ui.InputHolder:
		return w.FocusedInput()
	}
	return nil
}

func (a *App) PasteText(text string) {
	if holder, ok := a.Root.TopOverlayWidget().(ui.InputHolder); ok {
		if inp := holder.FocusedInput(); inp != nil {
			inp.PasteText(text)
			return
		}
	}
	if tw := a.focusedTerminalWidget(); tw != nil && tw.WantsRawKeys() {
		tw.PasteText(text)
		return
	}
	if holder, ok := a.Root.Focused.(ui.InputHolder); ok {
		if inp := holder.FocusedInput(); inp != nil {
			inp.PasteText(text)
			return
		}
	}
	if inp := a.focusedWidgetInput(); inp != nil {
		inp.PasteText(text)
		return
	}
	a.EditorGroup.PasteText(text)
}
