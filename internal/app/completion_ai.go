package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/eugenioenko/ttt/internal/ai"
	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/ui"
)

func (a *App) aiCompletionAvailable() bool {
	return a.Settings != nil && a.Settings.AI.Enabled && a.AIProvider != nil
}

// RequestAICompletions asks the configured AI provider for a completion. The
// request captures the buffer version so a slow reply cannot reopen the popup
// after the user has edited the document.
func (a *App) RequestAICompletions(path, lang string, line, col int) {
	target := a.currentCompletionTarget()
	a.startCompletionSources(target)
	a.requestAICompletions(path, lang, line, col, target, a.completion.generation)
}

func (a *App) requestAICompletions(path, lang string, line, col int, target completionTarget, generation uint64) {
	if !a.aiCompletionAvailable() {
		a.LogOutput("debug", "ai", "completion skipped: AI provider is disabled or unavailable")
		return
	}
	if !a.EditorGroup.IsEditorActive() {
		a.LogOutput("debug", "ai", "completion skipped: editor is not active")
		return
	}
	buf := a.EditorGroup.ActiveBuffer()
	if buf == nil || line < 0 || line >= len(buf.Lines) {
		a.LogOutput("debug", "ai", "completion skipped: active buffer or cursor is invalid")
		return
	}
	version := buf.Version()
	request := ai.CompletionRequest{
		Language: lang,
		FilePath: path,
		Text:     aiContext(buf, line, a.Settings.AI.MaxContextLines),
		Line:     line,
		Column:   col,
		Prefix:   a.currentPrefix(),
	}
	a.LogOutput("debug", "ai", fmt.Sprintf("completion request queued: language=%s line=%d column=%d", lang, line, col))
	if a.completion.aiRead == nil {
		a.completion.aiRead = NewTask[completionReply](a, asynctask.TaskOptions{})
	}
	a.completion.aiRead.Schedule(func(ctx context.Context) (completionReply, error) {
		a.LogOutputAsync("debug", "ai", fmt.Sprintf("calling provider: language=%s line=%d column=%d", request.Language, request.Line, request.Column))
		items, err := a.AIProvider.Complete(ctx, request)
		if err != nil {
			if ctx.Err() == nil {
				a.LogOutputAsync("error", "ai", "completion provider error: "+err.Error())
			}
			return completionReply{}, err
		}
		a.LogOutputAsync("debug", "ai", fmt.Sprintf("completion provider returned %d item(s)", len(items)))
		return completionReply{items: aiCompletionItems(items, request.Prefix)}, nil
	}, func(reply completionReply, err error) {
		if err != nil || generation != a.completion.generation || buf.Version() != version || a.EditorGroup.ActiveBuffer() != buf {
			return
		}
		a.completion.aiItems = reply.items
		a.applyMergedCompletionReply(target, generation)
	})
}

func aiContext(buf *buffer.Buffer, cursorLine, maxLines int) string {
	lines := buf.Lines
	if maxLines <= 0 || len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	half := maxLines / 2
	start := cursorLine - half
	if start < 0 {
		start = 0
	}
	if start+maxLines > len(lines) {
		start = len(lines) - maxLines
	}
	return strings.Join(lines[start:start+maxLines], "\n")
}

func aiCompletionItems(completions []ai.Completion, prefix string) []ui.CompletionItem {
	items := make([]ui.CompletionItem, 0, len(completions))
	for _, completion := range completions {
		if strings.TrimSpace(completion.Text) == "" {
			continue
		}
		label := strings.SplitN(completion.Text, "\n", 2)[0]
		if label == "" {
			label = completion.Text
		}
		items = append(items, ui.CompletionItem{
			Label:      label,
			Detail:     completion.Detail,
			InsertText: completion.Text,
			FilterText: prefix + label,
			SortText:   "\x00ttt-ai:" + label,
		})
	}
	return items
}
