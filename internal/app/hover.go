package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/lsp"
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
	server, _, _ := a.lspResolve(path, lang)
	key := hoverKey{Path: path, Language: lang, Server: server, WorkDir: a.lspWorkDir(path),
		Line: line, Col: col, AnchorX: anchorX, AnchorY: anchorY, Diagnostic: diagText,
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
	if key.Diagnostic != "" {
		if text != "" {
			text = key.Diagnostic + "\n---\n" + text
		} else {
			text = key.Diagnostic
		}
	}
	return text, nil
}

func (a *App) stopHoverTask() {
	if a.hover.task != nil {
		a.hover.task.Stop()
	}
}

func (a *App) DismissHover() {
	a.EditorGroup.Hover = nil
	a.stopHoverTask()
}
