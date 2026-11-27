package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/search"
	"github.com/eugenioenko/ttt/internal/ui"
)

type searchReplaceState struct {
	task    *asynctask.Task[map[string][]search.Match]
	request *ui.SearchReplaceRequest
	scan    func(context.Context, search.Options, []string) (map[string][]search.Match, error)
	dialog  ui.Widget
}

func (a *App) cancelSearchReplace() {
	if a.searchReplace.dialog != nil {
		a.Root.RemoveOverlay(a.searchReplace.dialog)
		if a.Root.Focused == a.searchReplace.dialog {
			a.FocusEditor()
		}
		a.searchReplace.dialog = nil
	}
	if a.searchReplace.task != nil {
		a.searchReplace.task.Stop()
	}
	a.searchReplace.request = nil
	a.Search.SetReplacementPending(false)
}

func (a *App) searchReplaceCurrent(req *ui.SearchReplaceRequest) bool {
	return !a.tasks.closed && a.searchReplace.request == req && a.Search.ReplacementCurrent(*req)
}

func (a *App) startSearchReplace(req ui.SearchReplaceRequest) {
	a.cancelSearchReplace()
	if a.tasks.closed || !a.Search.ReplacementCurrent(req) {
		return
	}
	a.searchReplace.request = &req
	if a.searchReplace.task == nil {
		a.searchReplace.task = NewTask[map[string][]search.Match](a, asynctask.TaskOptions{})
	}
	scan := a.searchReplace.scan
	if scan == nil {
		scan = search.FindAllMatches
	}
	a.Search.SetReplacementPending(true)
	a.searchReplace.task.Schedule(func(ctx context.Context) (map[string][]search.Match, error) {
		if req.Options.Query == "" {
			return nil, ctx.Err()
		}
		found, err := scan(ctx, req.Options, req.Paths)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return found, err
	}, func(found map[string][]search.Match, err error) {
		if !a.searchReplaceCurrent(&req) {
			a.cancelSearchReplace()
			return
		}
		a.Search.SetReplacementPending(false)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			a.cancelSearchReplace()
			return
		}
		if errors.Is(err, search.ErrRecordTooLarge) {
			a.cancelSearchReplace()
			a.StatusNotify(i18n.Text("search.recordLimitReached"))
			return
		}
		matches := make(map[string][]ui.SearchMatch)
		for _, path := range req.Paths {
			ms := found[filepath.Clean(path)]
			// Preserve panel-only callers with no query. A successful empty
			// rescan stays authoritative even when stale LineText is empty.
			if err != nil || req.Options.Query == "" {
				ms = req.Fallback[path]
			}
			if len(ms) > 0 {
				matches[path] = ms
			}
		}
		if len(matches) == 0 {
			a.StatusNotify(i18n.Text("status.noMatchesToReplace"))
			return
		}
		opts := ui.SearchOptions{CaseSensitive: req.Options.CaseSensitive, UseRegex: req.Options.UseRegex}
		if req.AllFiles {
			a.confirmSearchReplaceAll(matches, req.Replacement, opts, func() bool { return a.searchReplaceCurrent(&req) })
			a.searchReplace.dialog = a.Root.TopOverlayWidget()
		} else {
			path := req.Paths[0]
			a.ApplySearchReplace(path, matches[path], req.Replacement, opts)
		}
	})
}

func (a *App) PreviewSearchReplace(filePath string, matches []ui.SearchMatch, replacement string, opts ui.SearchOptions) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		a.StatusWarn(i18n.Format("status.cannotReadFile", i18n.Args{"error": err.Error()}))
		return
	}
	fd := ui.BuildReplaceDiff(filepath.Base(filePath), splitSearchLines(data), matches, replacement, opts)
	a.EditorGroup.OpenDiff(filePath, fd, nil, nil, false)
	a.Root.SetFocus(a.EditorGroup)
}

// splitSearchLines reads file bytes into lines the way rg reports them: a
// UTF-8 BOM stays out of the first line, matching Buffer.Lines, and a trailing
// newline does not produce a phantom last line.
func splitSearchLines(data []byte) []string {
	text, _ := strings.CutPrefix(string(data), buffer.UTF8ByteOrderMark)
	lines := strings.Split(text, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// staleSearchMatches reports whether any match's line no longer reads as it did
// when the search ran, so its columns may point at other text.
func staleSearchMatches(lines []string, matches []ui.SearchMatch) bool {
	for _, m := range matches {
		idx := m.LineNum - 1
		if idx < 0 || idx >= len(lines) {
			return true
		}
		// rg sends non-UTF-8 lines as bytes, which leaves LineText empty.
		if m.LineText != "" && strings.TrimRight(lines[idx], "\r") != m.LineText {
			return true
		}
	}
	return false
}

// joinReplacedLines preserves the original's UTF-8 BOM and ends the text with
// a newline only if the original did.
func joinReplacedLines(lines []string, original []byte) []byte {
	text := strings.Join(lines, "\n")
	if strings.HasSuffix(string(original), "\n") {
		text += "\n"
	}
	if strings.HasPrefix(string(original), buffer.UTF8ByteOrderMark) {
		text = buffer.UTF8ByteOrderMark + text
	}
	return []byte(text)
}

func (a *App) ApplySearchReplace(filePath string, matches []ui.SearchMatch, replacement string, opts ui.SearchOptions) {
	if a.isEditorPathDirty(filePath) {
		a.StatusWarn(i18n.Format("status.hasUnsavedChangesSaveItBeforeReplacing", i18n.Args{"filename": filepath.Base(filePath)}))
		return
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		a.StatusWarn(i18n.Format("status.cannotReadFile", i18n.Args{"error": err.Error()}))
		return
	}
	lines := splitSearchLines(data)
	if staleSearchMatches(lines, matches) {
		a.StatusWarn(i18n.Format("status.changedSinceTheSearchNothingReplaced", i18n.Args{"filename": filepath.Base(filePath)}))
		a.Search.Refresh()
		return
	}
	newLines := ui.ApplyReplacements(lines, matches, replacement, opts)
	if err := os.WriteFile(filePath, joinReplacedLines(newLines, data), 0644); err != nil {
		a.StatusWarn(i18n.Format("status.cannotWriteFile", i18n.Args{"error": err.Error()}))
		return
	}
	a.invalidateRepositoryPath(filePath, RepositoryWorktree)
	a.reloadFile(filePath)
	a.Search.Refresh()
	a.StatusNotify(i18n.Format("status.replacedMatchesIn", i18n.Args{"matchesCount": len(matches), "filename": filepath.Base(filePath)}))
}

func (a *App) ApplySearchReplaceAll(allMatches map[string][]ui.SearchMatch, replacement string, opts ui.SearchOptions) {
	a.confirmSearchReplaceAll(allMatches, replacement, opts, nil)
}

func (a *App) confirmSearchReplaceAll(allMatches map[string][]ui.SearchMatch, replacement string, opts ui.SearchOptions, current func() bool) {
	totalFiles := len(allMatches)
	totalMatches := 0
	for _, m := range allMatches {
		totalMatches += len(m)
	}
	msg := i18n.Format("search.confirmReplaceAll", i18n.Args{"totalMatches": totalMatches, "totalFiles": totalFiles})
	a.ShowConfirmDialog(msg, []string{i18n.Text("general.cancel"), i18n.Text("general.replaceAll")}, []func(){
		func() {
			a.DismissDialog()
			if current != nil {
				a.cancelSearchReplace()
			}
		},
		func() {
			if current != nil && !current() {
				return
			}
			a.DismissDialog()
			invalidatePath := ""
			replacedMatches, replacedFiles := 0, 0
			var skipped []string
			for filePath, matches := range allMatches {
				if a.isEditorPathDirty(filePath) {
					skipped = append(skipped, filepath.Base(filePath))
					continue
				}
				data, err := os.ReadFile(filePath)
				if err != nil {
					continue
				}
				lines := splitSearchLines(data)
				if staleSearchMatches(lines, matches) {
					skipped = append(skipped, filepath.Base(filePath))
					continue
				}
				newLines := ui.ApplyReplacements(lines, matches, replacement, opts)
				if err := os.WriteFile(filePath, joinReplacedLines(newLines, data), 0644); err != nil {
					continue
				}
				replacedMatches += len(matches)
				replacedFiles++
				if invalidatePath == "" {
					invalidatePath = filePath
				}
				a.reloadFile(filePath)
			}
			a.invalidateRepositoryPath(invalidatePath, RepositoryWorktree)
			a.Search.Refresh()
			msg := i18n.Format("search.replacedAcrossFiles", i18n.Args{"replacedMatches": replacedMatches, "replacedFiles": replacedFiles})
			if len(skipped) > 0 {
				sort.Strings(skipped)
				a.StatusWarn(i18n.Format("status.skippedUnsavedOrChangedSinceTheSearch", i18n.Args{"msg": msg, "items": strings.Join(skipped, ", ")}))
				return
			}
			a.StatusNotify(msg)
		},
	})
}
