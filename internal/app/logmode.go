package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync/atomic"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/core/logfile"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/gdamore/tcell/v3"
)

type logView struct {
	poster        atomic.Pointer[term.TcellScreen]
	searchSize    int64
	searchRE      *regexp.Regexp
	file          *logfile.File
	viewer        *ui.LogViewerWidget
	pending       atomic.Bool
	closed        atomic.Bool
	query         string
	options       ui.SearchOptions
	result        logfile.SearchResult
	current       int
	searchVersion uint64
	search        *asynctask.Task[logfile.SearchResult]
}
type LogFileProgress struct {
	Path string
	View *logView
}

func (a *App) activeLogView() *logView {
	if a.EditorGroup.ActiveLogViewer() == nil {
		return nil
	}
	return a.logViews[a.EditorGroup.ActiveFilePath()]
}
func (a *App) OpenActiveInLogMode()   { a.openLogView(a.EditorGroup.ActiveFilePath()) }
func (a *App) ExplorerOpenInLogMode() { a.openLogView(a.explorerNodePath()) }
func (a *App) openLogView(path string) {
	if path == "" {
		a.StatusWarn("No file to open in log mode")
		return
	}
	if buf := a.EditorGroup.BufferForPath(path); buf != nil && buf.Dirty {
		a.StatusWarn(filepath.Base(path) + " has unsaved changes")
		return
	}
	if lv := a.logViews[path]; lv != nil && a.EditorGroup.HasLogTab(path) {
		a.EditorGroup.SwitchToTabByPath(path)
		return
	}
	v := a.newLogViewer(path)
	if v == nil {
		return
	}
	a.EditorGroup.OpenLogTab(path, v)
	a.RequestGitGutterForActiveFile()
	a.RefreshSymbols()
	delete(a.largeFiles.stale, path)
	a.SyncReloadSegment()
	a.SyncWatched()
	a.syncLogStatus()
}
func (a *App) newLogViewer(path string) *ui.LogViewerWidget {
	lv := &logView{current: -1}
	lv.poster.Store(a.Screen)
	file, err := logfile.Open(path, func() {
		if screen := lv.poster.Load(); !lv.closed.Load() && screen != nil && lv.pending.CompareAndSwap(false, true) {
			if err := screen.PostEvent(tcell.NewEventInterrupt(&LogFileProgress{Path: path, View: lv})); err != nil {
				lv.pending.Store(false)
			}
		}
	})
	if err != nil {
		a.StatusWarn(fmt.Sprintf("Failed to open %s: %v", filepath.Base(path), err))
		return nil
	}
	lv.file = file
	lv.viewer = ui.NewLogViewer(file)
	lv.viewer.LineNumbers = a.Settings.Editor.LineNumbers
	lv.viewer.TabSize = a.Settings.Editor.TabSize
	lv.viewer.OnNotice = a.StatusNotify
	lv.viewer.OnClose = func() { a.closeLogView(path, lv) }
	lv.viewer.OnCopy = func() {
		start, end := lv.viewer.Selection()
		text, limited, err := file.Copy(start, end)
		if err != nil {
			a.StatusWarn("Cannot copy log: " + err.Error())
			return
		}
		clipboard.Set(text)
		if limited {
			a.StatusNotify("Log selection was capped at 4 MiB or the displayed line limit")
		}
	}
	lv.viewer.OnFindNext = func(delta int) { a.navigateLogMatch(lv, delta) }
	lv.viewer.OnClearFind = func() {
		lv.searchVersion++
		if lv.search != nil {
			lv.search.Stop()
		}
		lv.query = ""
		lv.searchRE = nil
		lv.result = logfile.SearchResult{}
		lv.viewer.Matches = nil
	}
	if a.logViews == nil {
		a.logViews = make(map[string]*logView)
	}
	a.logViews[path] = lv
	return lv.viewer
}
func (a *App) closeLogView(path string, lv *logView) {
	lv.closed.Store(true)
	if lv.search != nil {
		lv.search.Close()
	}
	lv.file.Close()
	if a.logViews[path] == lv {
		delete(a.logViews, path)
	}
}
func (a *App) closeLogViews() {
	for path, lv := range a.logViews {
		a.closeLogView(path, lv)
	}
}
func (a *App) handleLogProgress(ev *LogFileProgress) {
	lv := a.logViews[ev.Path]
	if lv == nil || lv != ev.View {
		return
	}
	lv.pending.Store(false)
	reset := lv.file.TakeReplaced()
	if reset {
		a.StatusNotify(filepath.Base(ev.Path) + " was truncated or replaced; reading it again")
	}
	lv.viewer.RefreshProgress(reset)
	if reset && lv.query != "" {
		a.searchLog(lv, lv.query, lv.options, true)
	} else {
		a.extendLogSearch(lv)
	}
	a.syncLogStatus()
}
func (a *App) logPaths() []string {
	paths := make([]string, 0, len(a.logViews))
	for path := range a.logViews {
		paths = append(paths, path)
	}
	return paths
}
func (a *App) syncLogStatus() {
	for _, view := range a.logViews {
		view.poster.Store(a.Screen)
	}
	lv := a.activeLogView()
	if lv == nil {
		a.Status.RemoveSegment("logmode")
		return
	}
	a.Status.RemoveSegment("indent")
	a.Status.RemoveSegment("eol")
	a.Status.RemoveSegment("language")
	p := lv.file.Progress()
	text := fmt.Sprintf("LOG %d lines", p.Lines)
	if lv.viewer.Following {
		text += " · following"
	} else {
		text += " · paused"
	}
	if p.Indexing {
		text += fmt.Sprintf(" · indexing %d%%", p.Scanned*100/max(p.Size, 1))
	}
	if p.Err != nil {
		text += " · read error"
	}
	a.Status.SetSegment(view.StatusSegment{ID: "logmode", Side: "right", Priority: 50, Text: text, OnClick: a.ConfirmExitLogMode})
}
func (a *App) OpenActiveInEditor() {
	lv := a.activeLogView()
	if lv == nil {
		return
	}
	path := lv.file.Path()
	info, err := os.Stat(path)
	if err != nil {
		a.StatusWarn("Cannot open in editor: " + err.Error())
		return
	}
	if a.Root.HasOverlay() {
		return
	}
	open := func() {
		a.DismissDialog()
		a.EditorGroup.OpenFileInEditor(path)
		a.Root.SetFocus(a.EditorGroup)
		a.RefreshSymbols()
		a.SyncWatched()
		a.syncLogStatus()
	}
	message := "Turn off log mode? The file will be loaded into the editor."
	if limit := a.Settings.Editor.AutoReloadMaxBytes(); limit > 0 && info.Size() > limit {
		message += fmt.Sprintf(" This %.1f MiB file can be slow to load and use a lot of memory.", float64(info.Size())/(1<<20))
	}
	a.ShowConfirmDialog(message, []string{"Open in Editor", "Cancel"}, []func(){open, a.DismissDialog})
}
func (a *App) searchLog(lv *logView, query string, opts ui.SearchOptions, force bool) []ui.FindMatch {
	if !force && lv.query == query && lv.options == opts {
		return logFindMatches(lv.result.Matches)
	}
	lv.searchVersion++
	if lv.search == nil {
		lv.search = NewTask[logfile.SearchResult](a, asynctask.TaskOptions{})
	}
	lv.search.Stop()
	lv.query, lv.options, lv.current = query, opts, -1
	lv.result = logfile.SearchResult{}
	lv.searchSize = 0
	lv.searchRE = nil
	lv.viewer.Matches = nil
	if query == "" {
		return nil
	}
	pattern := query
	if !opts.UseRegex {
		pattern = regexp.QuoteMeta(pattern)
	}
	if !opts.CaseSensitive {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		a.StatusWarn("Invalid regex: " + err.Error())
		return nil
	}
	lv.searchRE = re
	a.startLogSearch(lv, 0, true)
	return logFindMatches(lv.result.Matches)
}
func (a *App) extendLogSearch(lv *logView) {
	if lv.closed.Load() || lv.searchRE == nil || lv.result.Limited || lv.search.Pending() || lv.searchSize >= lv.file.Progress().Size {
		return
	}
	lv.searchVersion++
	a.startLogSearch(lv, lv.searchSize, false)
}
func (a *App) startLogSearch(lv *logView, start int64, navigate bool) {
	file, re, version, post := lv.file, lv.searchRE, lv.searchVersion, a.tasks.post
	lv.search.Schedule(func(ctx context.Context) (logfile.SearchResult, error) {
		return file.SearchFrom(ctx, re, start, func(result logfile.SearchResult) {
			if post != nil {
				post(func() { a.applyLogSearch(lv, version, result, false, navigate) })
			}
		})
	}, func(result logfile.SearchResult, err error) {
		if lv.closed.Load() || lv.searchVersion != version {
			return
		}
		if err != nil {
			if bar, ok := a.Root.TopOverlayWidget().(*ui.FindBarWidget); ok && a.activeLogView() == lv {
				bar.Progress = ""
			}
			a.StatusWarn("Log search: " + err.Error())
			return
		}
		a.applyLogSearch(lv, version, result, true, navigate)
		a.extendLogSearch(lv)
	})
}

func logFindMatches(matches []logfile.Match) []ui.FindMatch {
	out := make([]ui.FindMatch, len(matches))
	for i, m := range matches {
		out[i] = ui.FindMatch{Line: i, Col: m.Col, Len: m.Len}
	}
	return out
}
func (a *App) applyLogSearch(lv *logView, version uint64, result logfile.SearchResult, done, navigate bool) {
	if lv.closed.Load() || lv.searchVersion != version {
		return
	}
	first := navigate && len(lv.result.Matches) == 0
	if done || navigate {
		prefix := sort.Search(len(lv.result.Matches), func(i int) bool { return lv.result.Matches[i].Offset >= result.Start })
		matches := make([]logfile.Match, 0, min(prefix+len(result.Matches), logfile.MaxMatches))
		matches = append(matches, lv.result.Matches[:prefix]...)
		count := min(len(result.Matches), logfile.MaxMatches-prefix)
		matches = append(matches, result.Matches[:count]...)
		result.Limited = result.Limited || count < len(result.Matches)
		result.Matches = matches
		lv.result = result
		lv.viewer.Matches = matches
		if lv.current >= len(matches) {
			lv.current = max(len(matches)-1, 0)
		}
	}
	if done {
		lv.searchSize = result.Size
	}
	if a.activeLogView() != lv {
		return
	}
	if bar, ok := a.Root.TopOverlayWidget().(*ui.FindBarWidget); ok && bar.Input.Text == lv.query && bar.Options == lv.options {
		bar.Matches = logFindMatches(lv.result.Matches)
		bar.Current = max(lv.current, 0)
		bar.Progress = ""
		if !done {
			bar.Progress = fmt.Sprintf(" %d%%", (result.Scanned-result.Start)*100/max(result.Size-result.Start, 1))
		}
		if first && len(result.Matches) > 0 {
			lv.current = 0
			bar.Current = 0
			lv.viewer.Navigate(result.Matches[0])
		}
	}
	if done && result.Limited {
		a.StatusNotify("Log search stopped at 10,000 matches; refine the query")
	}
}
func (a *App) navigateLogMatch(lv *logView, delta int) {
	n := len(lv.result.Matches)
	if n == 0 {
		return
	}
	lv.current = (lv.current + delta + n) % n
	lv.viewer.Navigate(lv.result.Matches[lv.current])
	if bar, ok := a.Root.TopOverlayWidget().(*ui.FindBarWidget); ok {
		bar.Current = lv.current
	}
}

func (a *App) autoLogViewer(path string) ui.Widget {
	threshold, enabled := a.Settings.Editor.LogModeMinBytes()
	if !enabled {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || threshold > 0 && info.Size() <= threshold {
		return nil
	}
	viewer := a.newLogViewer(path)
	if viewer == nil {
		return nil
	}
	a.StatusNotify(filepath.Base(path) + " opened in log mode because of its size; use Open in Editor to edit")
	return viewer
}

func (a *App) ConfirmExitLogMode() { a.OpenActiveInEditor() }
