package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"
)

type completionState struct {
	debounce  *asynctask.Task[struct{}]
	delay     time.Duration
	read      *asynctask.Task[completionReply]
	signature *asynctask.Task[*SignatureHelpResult]
}

type completionReply struct {
	items    []ui.CompletionItem
	lspItems []lsp.CompletionItem
}

// completionTarget is the word a completion request was made for. A reply is
// shown only while the cursor is still in it, so a late one cannot open the
// popup after the cursor moved to another word, line, or file.
type completionTarget struct {
	buf         *buffer.Buffer
	line, start int
}

func (a *App) currentCompletionTarget() completionTarget {
	line, start, _ := a.identStart()
	return completionTarget{buf: a.EditorGroup.ActiveBuffer(), line: line, start: start}
}

// ScheduleAutocomplete runs auto-suggest once typing pauses, if the cursor is
// still in the word being typed. Navigation does not stop the debounce.
func (a *App) ScheduleAutocomplete() {
	if !a.Settings.Autocomplete.Enabled || !a.Settings.Autocomplete.AutoSuggest {
		return
	}
	delay := time.Duration(a.Settings.Autocomplete.Debounce) * time.Millisecond
	if a.completion.debounce == nil || a.completion.delay != delay {
		if a.completion.debounce != nil {
			a.completion.debounce.Close()
		}
		a.completion.debounce = NewTask[struct{}](a, asynctask.TaskOptions{Delay: delay})
		a.completion.delay = delay
	}
	target := a.currentCompletionTarget()
	a.completion.debounce.Schedule(func(context.Context) (struct{}, error) {
		return struct{}{}, nil
	}, func(_ struct{}, err error) {
		if err == nil && a.currentCompletionTarget() == target {
			a.HandleAutocompleteTrigger()
		}
	})
}

// HandleAutocompleteTrigger requests completions when auto-suggest fires.
func (a *App) HandleAutocompleteTrigger() {
	triggerChar := a.charBeforeCursor()
	isTrigger := a.isCompletionTrigger(triggerChar)
	if isTrigger && a.IsAutocompleteActive() {
		a.DismissAutocomplete()
	}
	if a.IsAutocompleteActive() {
		return
	}
	if prefix := a.currentPrefix(); len(prefix) >= 1 || isTrigger {
		path, lang := a.editorPathLang()
		line, col := a.EditorGroup.ActiveCursor()
		a.RequestCompletions(path, lang, line, col, triggerChar)
	}
}

// RequestCompletions asks the server for completions at line and col,
// cancelling any request still in flight.
func (a *App) RequestCompletions(path, lang string, line, col int, triggerChar string) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok || a.EditorGroup.ActiveBuffer() == nil {
		return
	}
	target := a.currentCompletionTarget()
	workDir := a.lspWorkDir(path)
	manager := a.LspManager
	if a.completion.read == nil {
		a.completion.read = NewTask[completionReply](a, asynctask.TaskOptions{})
	}
	a.completion.read.Schedule(func(ctx context.Context) (completionReply, error) {
		client, err := manager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			slog.Error("lsp client", "err", err)
			return completionReply{}, err
		}
		var trigger *lsp.CompletionContext
		if triggerChar != "" {
			for _, tc := range client.CompletionTriggerCharacters() {
				if triggerChar == tc {
					trigger = &lsp.CompletionContext{
						TriggerKind:      lsp.CompletionTriggerTriggerCharacter,
						TriggerCharacter: triggerChar,
					}
					break
				}
			}
		}
		if trigger == nil {
			trigger = &lsp.CompletionContext{TriggerKind: lsp.CompletionTriggerInvoked}
		}
		slog.Debug("lsp completion request", "path", path, "line", line, "col", col, "triggerChar", triggerChar)
		items, err := client.Completion(ctx, FileURI(path), line, col, trigger)
		if err != nil {
			if ctx.Err() == nil {
				slog.Error("lsp completion", "err", err)
			}
			return completionReply{}, err
		}
		slog.Debug("lsp completion response", "count", len(items))
		return completionReply{items: LspToUICompletions(items), lspItems: items}, nil
	}, func(reply completionReply, err error) {
		if err == nil {
			a.applyCompletionReply(target, reply)
		}
	})
}

// applyCompletionReply shows reply while the cursor is still in the word the
// request was made for.
func (a *App) applyCompletionReply(target completionTarget, reply completionReply) {
	if len(reply.items) == 0 || a.currentCompletionTarget() != target {
		return
	}
	a.ShowAutocomplete(reply.items, reply.lspItems)
}

// CloseAutocomplete dismisses the popup on the user's behalf and cancels the
// suggestion still on its way. Accepting inserts a word, which schedules
// auto-suggest for it; without this the popup reopened and Enter accepted
// again instead of breaking the line, and Escape did not stick.
func (a *App) CloseAutocomplete() {
	a.DismissAutocomplete()
	if a.completion.debounce != nil {
		a.completion.debounce.Stop()
	}
	if a.completion.read != nil {
		a.completion.read.Stop()
	}
}

// RequestSignatureHelp asks the server for the signature at line and col once
// changeDone, if non-nil, reports that the didChange for the same edit was
// sent. The reply is shown only while the cursor is still on that line, at or
// after col.
func (a *App) RequestSignatureHelp(path, lang string, line, col int, changeDone <-chan struct{}) {
	serverKey, _, ok := a.lspResolve(path, lang)
	buf := a.EditorGroup.ActiveBuffer()
	if !ok || buf == nil {
		return
	}
	workDir := a.lspWorkDir(path)
	manager := a.LspManager
	if a.completion.signature == nil {
		a.completion.signature = NewTask[*SignatureHelpResult](a, asynctask.TaskOptions{})
	}
	a.completion.signature.Schedule(func(ctx context.Context) (*SignatureHelpResult, error) {
		if changeDone != nil {
			select {
			case <-changeDone:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		client, err := manager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			slog.Error("lsp client", "err", err)
			return nil, err
		}
		sig, err := client.SignatureHelp(ctx, FileURI(path), line, col)
		if err != nil {
			if ctx.Err() == nil {
				slog.Error("lsp signatureHelp", "err", err)
			}
			return nil, err
		}
		if sig == nil || len(sig.Signatures) == 0 {
			return nil, nil
		}
		result := LspToSignatureHelpResult(sig)
		if result.Label != "" {
			slog.Debug("lsp signature help response", "label", result.Label)
		}
		return result, nil
	}, func(result *SignatureHelpResult, err error) {
		if err == nil {
			a.applySignatureHelp(buf, line, col, result)
		}
	})
}

// applySignatureHelp shows result while the cursor is still on the requested
// line, at or after the requested column.
func (a *App) applySignatureHelp(buf *buffer.Buffer, line, col int, result *SignatureHelpResult) {
	curLine, curCol := a.EditorGroup.ActiveCursor()
	if result == nil || result.Label == "" || a.EditorGroup.ActiveBuffer() != buf || curLine != line || curCol < col {
		return
	}
	a.ShowSignatureHelp(result)
}

// DismissSignatureHelp hides the popup and cancels a request still on its way.
func (a *App) DismissSignatureHelp() {
	a.EditorGroup.SignatureHelp = nil
	if a.completion.signature != nil {
		a.completion.signature.Stop()
	}
}
