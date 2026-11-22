package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"
)

const (
	pathCompletionLimit = 100
	pathCompletionBatch = 256
)

type pathCompletionTarget struct {
	buf              *buffer.Buffer
	line, start, col int
	version          uint64
	dir, prefix      string
	separator        byte
	quote            rune
}

func (a *App) currentPathCompletionTarget() pathCompletionTarget {
	settings := a.Settings.Autocomplete
	if !settings.Enabled || !settings.PathCompletion || !a.EditorGroup.IsEditorActive() {
		return pathCompletionTarget{}
	}
	editor := a.EditorGroup.Editor
	if editor.Selection != nil && editor.Selection.Active {
		return pathCompletionTarget{}
	}
	line, col := editor.Cursor.Line, editor.Cursor.Col
	if line < 0 || line >= len(editor.Buf.Lines) {
		return pathCompletionTarget{}
	}
	text := editor.Buf.Lines[line]
	runes := []rune(text)
	if col <= 0 || col > len(runes) {
		return pathCompletionTarget{}
	}
	dir, prefix, separator, quote := splitPathCompletion(runes[:col])
	if separator == 0 {
		return pathCompletionTarget{}
	}
	if filepath.IsAbs(dir) {
		if !settings.AbsolutePathCompletion {
			return pathCompletionTarget{}
		}
	} else {
		root := a.Workspace.Primary()
		if folder := a.Workspace.FolderForFile(a.EditorGroup.ActiveFilePath()); folder != nil {
			root = folder.Path
		}
		if root == "" {
			return pathCompletionTarget{}
		}
		dir = filepath.Join(root, dir)
	}
	return pathCompletionTarget{
		buf: editor.Buf, line: line, col: col, version: editor.Buf.Version(),
		start: col - len([]rune(prefix)), dir: dir, prefix: prefix, separator: separator, quote: quote,
	}
}

func splitPathCompletion(before []rune) (dir, prefix string, separator byte, quote rune) {
	start := len(before)
	for start > 0 {
		r := before[start-1]
		if pathTokenBoundary(r) {
			break
		}
		start--
	}
	seenQuotes := make(map[rune]bool, 3)
	for i := len(before) - 1; i >= 0; i-- {
		r := before[i]
		if (r != '"' && r != '\'' && r != '`') || seenQuotes[r] {
			continue
		}
		seenQuotes[r] = true
		if dir, prefix, separator = explicitPath(string(before[i+1:])); separator != 0 {
			return dir, prefix, separator, r
		}
	}
	dir, prefix, separator = explicitPath(string(before[start:]))
	return dir, prefix, separator, 0
}

func pathTokenBoundary(r rune) bool {
	return unicode.IsSpace(r) || strings.ContainsRune("\"'`([{=,;<>)]}", r)
}

func explicitPath(path string) (dir, prefix string, separator byte) {
	if path == "" {
		return "", "", 0
	}
	if !filepath.IsAbs(path) {
		relative := len(path) >= 2 && path[0] == '.' && os.IsPathSeparator(path[1])
		parent := len(path) >= 3 && path[:2] == ".." && os.IsPathSeparator(path[2])
		if !relative && !parent {
			return "", "", 0
		}
	}
	if strings.HasPrefix(path, "//") && runtime.GOOS != "windows" {
		return "", "", 0
	}
	for i := len(path) - 1; i >= 0; i-- {
		if os.IsPathSeparator(path[i]) {
			return path[:i+1], path[i+1:], path[i]
		}
	}
	return "", "", 0
}

func (a *App) requestPathCompletions(target pathCompletionTarget) {
	if target.buf == nil || a.currentPathCompletionTarget() != target {
		return
	}
	if a.completion.pathRead == nil {
		a.completion.pathRead = NewTask[[]ui.CompletionItem](a, asynctask.TaskOptions{})
	}
	a.completion.pathRead.Schedule(func(ctx context.Context) ([]ui.CompletionItem, error) {
		return readPathCompletions(ctx, target)
	}, func(items []ui.CompletionItem, err error) {
		if err == nil && len(items) > 0 && !a.IsAutocompleteActive() && a.currentPathCompletionTarget() == target {
			a.showPathCompletions(target, items)
		}
	})
}

// readPathCompletions keeps the lexicographically first pathCompletionLimit matches of a
// directory. Directory names arrive in readdir order, so matches are inserted into a sorted
// slice that never grows past the limit instead of being collected and sorted afterwards.
// That bounds the memory to the limit plus one read batch, keeps the ordering predictable,
// and lets cancellation stop the walk between batches. The walk still visits the whole
// directory, and the limit applies after prefix filtering, so a longer prefix reaches entries
// whose names sort after the first pathCompletionLimit matches.
func readPathCompletions(ctx context.Context, target pathCompletionTarget) ([]ui.CompletionItem, error) {
	dir, err := os.Open(target.dir)
	if err != nil {
		return nil, err
	}
	defer dir.Close()

	prefix := target.prefix
	if runtime.GOOS == "windows" {
		prefix = strings.ToLower(prefix)
	}
	items := make([]ui.CompletionItem, 0, pathCompletionLimit)
	for {
		entries, readErr := dir.ReadDir(pathCompletionBatch)
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if item, ok := pathCompletionItem(target, prefix, entry); ok {
				items = insertPathCompletion(items, item)
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return items, nil
			}
			return nil, readErr
		}
		if len(entries) == 0 {
			return items, nil
		}
	}
}

// insertPathCompletion adds item to the ascending items slice, drops the largest entry when the
// slice is already at the limit, and returns the result.
func insertPathCompletion(items []ui.CompletionItem, item ui.CompletionItem) []ui.CompletionItem {
	at := sort.Search(len(items), func(i int) bool { return items[i].Label >= item.Label })
	if at == len(items) {
		if len(items) < pathCompletionLimit {
			return append(items, item)
		}
		return items
	}
	if len(items) < pathCompletionLimit {
		items = append(items, ui.CompletionItem{})
	}
	copy(items[at+1:], items[at:])
	items[at] = item
	return items
}

func pathCompletionItem(target pathCompletionTarget, prefix string, entry os.DirEntry) (ui.CompletionItem, bool) {
	name := entry.Name()
	match := name
	if runtime.GOOS == "windows" {
		match = strings.ToLower(match)
	}
	if !strings.HasPrefix(match, prefix) || strings.ContainsAny(name, "\r\n\t") ||
		(target.quote == 0 && strings.ContainsFunc(name, pathTokenBoundary)) ||
		(target.quote != 0 && strings.ContainsRune(name, target.quote)) {
		return ui.CompletionItem{}, false
	}
	isDir := entry.IsDir()
	if entry.Type()&os.ModeSymlink != 0 {
		info, err := os.Stat(filepath.Join(target.dir, name))
		isDir = err == nil && info.IsDir()
	}
	kind := ui.CompletionVariable
	if isDir {
		name += string(target.separator)
		kind = ui.CompletionModule
	}
	return ui.CompletionItem{Label: name, InsertText: name, Kind: kind}, true
}

func (a *App) showPathCompletions(target pathCompletionTarget, items []ui.CompletionItem) {
	popup := ui.NewAutocompleteWidget(items, 0, 0)
	popup.Icons = a.Settings.Appearance.Icons
	popup.OnDismiss = a.CloseAutocomplete
	popup.OnSelect = func(item ui.CompletionItem) {
		a.CloseAutocomplete()
		if a.currentPathCompletionTarget() != target {
			return
		}
		start := lsp.Position{Line: target.line, Character: target.start}
		end := lsp.Position{Line: target.line, Character: target.col}
		cursor := lsp.Position{Line: target.line, Character: target.start + len([]rune(item.InsertText))}
		a.applyRuneTextEdits([]lsp.TextEdit{{Range: lsp.Range{Start: start, End: end}, NewText: item.InsertText}}, &cursor)
		a.CloseAutocomplete()
		if item.Kind == ui.CompletionModule {
			path, lang := a.editorPathLang()
			a.RequestCompletions(path, lang, cursor.Line, cursor.Character, string(target.separator))
		}
	}
	a.completion.pathPopup = &target
	a.EditorGroup.Autocomplete = popup
}
