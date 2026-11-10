package app

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

type hoverKey struct {
	Path, Language, Server, WorkDir, Diagnostic string
	Line, Col, AnchorX, AnchorY                 int
	Version                                     uint64
	Manager                                     *lsp.Manager
}

type hoverState struct {
	task  *asynctask.Task[string]
	delay time.Duration
	// tab is the tab bar index whose cut name the popup (shown or pending)
	// belongs to; tabOwned is false for LSP hovers.
	tab      int
	tabOwned bool
}

type tabHoverKey struct {
	Index            int
	Name             string
	AnchorX, AnchorY int
}

func (a *App) hoverDiagnostic(line, col int) string {
	if a.EditorGroup.Editor != nil {
		if d := a.EditorGroup.Editor.DiagnosticAt(line, col); d != nil {
			return d.Message
		}
	}
	return ""
}

func (a *App) ShowCursorHover() {
	path, lang := a.editorPathLang()
	line, col := a.EditorGroup.ActiveCursor()
	ax, ay, _ := a.EditorGroup.CursorPosition()
	a.RequestHover(path, lang, line, col, ax, ay, a.hoverDiagnostic(line, col))
}

func (a *App) RequestHover(path, lang string, line, col, anchorX, anchorY int, diagText string) {
	if a.hover.task != nil {
		a.hover.task.Invalidate()
	}
	a.scheduleHover(path, lang, line, col, anchorX, anchorY, diagText)
	if a.hover.task != nil {
		a.hover.task.Flush()
	}
}

func (a *App) scheduleHover(path, lang string, line, col, anchorX, anchorY int, diagText string) {
	if a.EditorGroup.ActiveBuffer() == nil || a.EditorGroup.IsActiveVirtual() {
		a.DismissHover()
		return
	}
	a.ensureHoverTask()
	server, _, _ := a.lspResolve(path, lang)
	key := hoverKey{Path: path, Language: lang, Server: server, WorkDir: a.lspWorkDir(path),
		Line: line, Col: a.lspColumn(path, line, col), AnchorX: anchorX, AnchorY: anchorY, Diagnostic: diagText,
		Version: a.EditorGroup.ActiveBuffer().Version(), Manager: a.LspManager}
	a.hover.task.ScheduleKey(key, func(ctx context.Context) (string, error) {
		return readHover(ctx, key)
	}, func(text string, err error) {
		buf := a.EditorGroup.ActiveBuffer()
		// Reloads and async edits can change the buffer without a mouse event.
		if buf == nil || a.EditorGroup.ActiveFilePath() != key.Path || buf.Version() != key.Version {
			return
		}
		a.EditorGroup.Hover = nil
		if err == nil {
			a.ShowHover(text, key.AnchorX, key.AnchorY)
		}
	})
	if a.hover.task.Pending() {
		a.EditorGroup.Hover = nil
	}
}

func (a *App) ensureHoverTask() {
	delay := a.Settings.LSP.HoverDelay
	if delay <= 0 {
		delay = 400
	}
	wait := time.Duration(delay) * time.Millisecond
	if a.hover.task == nil || a.hover.delay != wait {
		if a.hover.task != nil {
			a.hover.task.Close()
		}
		a.hover.task = NewTask[string](a, asynctask.TaskOptions{Delay: wait})
		a.hover.delay = wait
	}
}

// TrackMouseHover shows, keeps or dismisses hover popups for a mouse event
// before it is routed to the widget tree.
func (a *App) TrackMouseHover(ev *tcell.EventMouse) {
	mx, my := ev.Position()
	btn := ev.Buttons()
	// Dismiss before routing: an existing hover can consume the Ctrl mouse event.
	if term.ControlModifierHeld(ev.Modifiers(), &a.Root.ControlModifiers) && a.EditorGroup.GetRect().Contains(mx, my) {
		a.DismissHover()
	} else if btn == 0 && a.checkTabHover(mx, my) {
		return
	} else if btn != 0 && a.hover.tabOwned && !a.isMouseInsideHover(mx, my) {
		a.DismissHover()
	} else if a.EditorGroup.Hover == nil {
		if btn == 0 {
			a.checkMouseHover(mx, my)
		}
	} else if !a.isMouseOverHover(mx, my) && !a.EditorGroup.Hover.IsDragging() {
		a.DismissHover()
	}
}

// checkTabHover handles pointer motion for cut tab names and reports whether
// the pointer is on the tab bar. A tab popup has no grace margin: moving to
// another tab or off the tab bar (other than onto the popup) dismisses it.
func (a *App) checkTabHover(mx, my int) bool {
	if a.isMouseInsideHover(mx, my) {
		return false
	}
	tabBar := a.EditorGroup.TabBar
	if !tabBar.GetRect().Contains(mx, my) {
		if a.hover.tabOwned {
			a.DismissHover()
		}
		return false
	}
	index, name, ax, ay, ok := tabBar.CutTabAt(mx, my)
	if ok && a.hover.tabOwned && a.hover.tab == index &&
		(a.EditorGroup.Hover != nil || (a.hover.task != nil && a.hover.task.Pending())) {
		return true
	}
	a.DismissHover()
	if ok {
		a.scheduleTabHover(tabHoverKey{Index: index, Name: name, AnchorX: ax, AnchorY: ay})
	}
	return true
}

func (a *App) isMouseInsideHover(mx, my int) bool {
	h := a.EditorGroup.Hover
	return h != nil && h.GetRect().Contains(mx, my)
}

func (a *App) scheduleTabHover(key tabHoverKey) {
	a.ensureHoverTask()
	a.hover.tab, a.hover.tabOwned = key.Index, true
	a.hover.task.ScheduleKey(key, func(context.Context) (string, error) {
		return escapeMarkdown(key.Name), nil
	}, func(text string, err error) {
		if err == nil && a.hover.tabOwned && a.hover.tab == key.Index {
			a.ShowHover(text, key.AnchorX, key.AnchorY)
		}
	})
}

func escapeMarkdown(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x80 && (unicode.IsPunct(r) || unicode.IsSymbol(r)) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func joinHoverText(diagnostic, text string) string {
	switch {
	case diagnostic == "":
		return text
	case text == "":
		return diagnostic
	default:
		return diagnostic + "\n\n---\n\n" + text
	}
}

func readHover(ctx context.Context, key hoverKey) (string, error) {
	text := ""
	if key.Server != "" {
		client, err := key.Manager.ClientForLanguage(key.Server, key.WorkDir)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if err != nil {
			slog.Error("lsp client", "err", err)
		} else {
			hover, err := client.HoverContext(ctx, FileURI(key.Path), key.Line, key.Col)
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			if err != nil {
				slog.Error("lsp hover", "err", err)
			} else if hover != nil {
				text = hover.Contents.Value
				slog.Debug("lsp hover response", "length", len(text))
			}
		}
	}
	return joinHoverText(key.Diagnostic, text), nil
}

func (a *App) stopHoverTask() {
	if a.hover.task != nil {
		a.hover.task.Stop()
	}
}

func (a *App) DismissHover() {
	a.EditorGroup.Hover = nil
	a.hover.tabOwned = false
	a.stopHoverTask()
}
