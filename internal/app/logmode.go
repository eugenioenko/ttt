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
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"
)

type logModeState struct {
	app   *App
	views map[*ui.LogViewerWidget]*logView
}

func (a *App) logMode() *logModeState {
	if a.logs == nil {
		a.logs = &logModeState{app: a, views: make(map[*ui.LogViewerWidget]*logView)}
	}
	return a.logs
}

type logView struct {
	poster        atomic.Pointer[logPost]
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

type logPost struct{ post asynctask.Post }

func (l *logModeState) active() *logView {
	a := l.app
	if a.EditorGroup.ActiveLogViewer() == nil {
		return nil
	}
	return l.views[a.EditorGroup.ActiveLogViewer()]
}

func (a *App) OpenActiveInLogMode() {
	if !a.Root.HasOverlay() {
		a.logMode().open(a.EditorGroup.ActiveFilePath())
	}
}

func (a *App) ExplorerOpenInLogMode() {
	if a.Root.HasOverlay() {
		return
	}
	path := a.explorerNodePath()
	a.ExplorerContextNode = nil
	a.logMode().open(path)
}

func (l *logModeState) open(path string) {
	a := l.app
	if path == "" {
		a.StatusWarn(i18n.Text("logMode.noFile"))
		return
	}
	if a.isEditorPathDirty(path) {
		a.StatusWarn(i18n.Format("logMode.unsaved", i18n.Args{"filename": filepath.Base(path)}))
		return
	}
	if a.EditorGroup.HasLogTab(path) {
		a.EditorGroup.SwitchToTabByPath(path)
		return
	}
	v, err := l.newViewer(path)
	if err != nil {
		a.StatusWarn(err.Error())
		return
	}
	a.EditorGroup.OpenLogTab(path, v)
	a.RequestGitGutterForActiveFile()
	a.RefreshSymbols()
	delete(a.largeFiles.stale, path)
	a.SyncReloadSegment()
	a.SyncWatched()
	l.syncStatus()
}

func (l *logModeState) newViewer(path string) (*ui.LogViewerWidget, error) {
	a := l.app
	lv := &logView{current: -1}
	lv.poster.Store(&logPost{post: a.tasks.post})
	file, err := logfile.Open(path, func() {
		delivery := lv.poster.Load()
		if lv.closed.Load() || delivery == nil || delivery.post == nil || !lv.pending.CompareAndSwap(false, true) {
			return
		}
		if err := delivery.post(func() { l.progress(lv) }); err != nil {
			lv.pending.Store(false)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("%s", i18n.Format("logMode.openFailed", i18n.Args{"filename": filepath.Base(path), "error": err.Error()}))
	}
	lv.file = file
	lv.viewer = ui.NewLogViewer(file)
	lv.viewer.LineNumbers = a.Settings.Editor.LineNumbers
	lv.viewer.TabSize = a.Settings.Editor.TabSize
	lv.viewer.ControlModifiers = &a.Root.ControlModifiers
	lv.viewer.OnNotice = a.StatusNotify
	lv.viewer.OnClose = func() { l.closeView(lv) }
	lv.viewer.OnCopy = func() {
		start, end := lv.viewer.Selection()
		text, limited, err := file.Copy(start, end)
		if err != nil {
			a.StatusWarn(i18n.Format("logMode.copyFailed", i18n.Args{"error": err.Error()}))
			return
		}
		clipboard.Set(text)
		if limited {
			a.StatusNotify(i18n.Text("logMode.copyLimited"))
		}
	}
	lv.viewer.OnFindNext = func(delta int) { l.navigateMatch(lv, delta) }
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
	l.views[lv.viewer] = lv
	return lv.viewer, nil
}

func (l *logModeState) closeView(lv *logView) {
	a := l.app
	lv.closed.Store(true)
	if lv.search != nil {
		a.releaseTask(lv.search)
	}
	lv.file.Close()
	delete(l.views, lv.viewer)
}

func (l *logModeState) close() {
	for _, lv := range l.views {
		l.closeView(lv)
	}
}

func (l *logModeState) progress(lv *logView) {
	a := l.app
	if lv.closed.Load() || l.views[lv.viewer] != lv {
		return
	}
	lv.pending.Store(false)
	reset := lv.file.TakeReplaced()
	if reset {
		a.StatusNotify(i18n.Format("logMode.replaced", i18n.Args{"filename": filepath.Base(lv.file.Path())}))
	}
	lv.viewer.RefreshProgress(reset)
	if reset && lv.query != "" {
		l.search(lv, lv.query, lv.options, true)
	} else {
		l.extendSearch(lv)
	}
	l.syncStatus()
}

func (l *logModeState) paths() []string {
	paths := make([]string, 0, len(l.views))
	for _, lv := range l.views {
		paths = append(paths, lv.file.Path())
	}
	return paths
}

func (l *logModeState) syncStatus() {
	a := l.app
	lv := l.active()
	if lv == nil {
		a.Status.RemoveSegment("logmode")
		return
	}
	a.Status.RemoveSegment("indent")
	a.Status.RemoveSegment("eol")
	a.Status.RemoveSegment("language")
	p := lv.file.Progress()
	state := i18n.Text("logMode.paused")
	if lv.viewer.Following {
		state = i18n.Text("logMode.following")
	}
	text := i18n.Format("logMode.status", i18n.Args{"count": p.Lines, "state": state})
	if p.Indexing {
		text = i18n.Format("logMode.indexingStatus", i18n.Args{"count": p.Lines, "state": state, "percent": p.Scanned * 100 / max(p.Size, 1)})
	}
	if p.Err != nil {
		text = i18n.Text("logMode.readError")
	}

	a.Status.SetSegment(view.StatusSegment{ID: "logmode", Side: "right", Priority: 50, Text: text, OnClick: a.OpenActiveInEditor})
}

func (l *logModeState) openInEditor() {
	a := l.app
	lv := l.active()
	if lv == nil {
		return
	}
	path := lv.file.Path()
	info, err := os.Stat(path)
	if err != nil {
		a.StatusWarn(i18n.Format("logMode.editorFailed", i18n.Args{"error": err.Error()}))
		return
	}
	if a.Root.HasOverlay() {
		return
	}
	open := func() {
		a.DismissDialog()
		if err := a.EditorGroup.OpenFileInEditor(path); err != nil {
			return
		}
		a.Root.SetFocus(a.EditorGroup)
		a.RefreshSymbols()
		a.SyncWatched()
		l.syncStatus()
	}
	message := i18n.Text("logMode.confirmEditor")
	if limit := a.Settings.Editor.AutoReloadMaxBytes(); limit > 0 && info.Size() > limit {
		message = i18n.Format("logMode.confirmLargeEditor", i18n.Args{"size": fmt.Sprintf("%.1f", float64(info.Size())/(1<<20))})
	}
	a.ShowConfirmDialog(message, []string{i18n.Text("logMode.openEditor"), i18n.Text("general.cancel")}, []func(){open, a.DismissDialog})
}

func (l *logModeState) search(lv *logView, query string, opts ui.SearchOptions, force bool) []ui.FindMatch {
	a := l.app
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
		a.StatusWarn(i18n.Format("status.invalidRegex", i18n.Args{"error": err.Error()}))
		return nil
	}
	lv.searchRE = re
	l.startSearch(lv, 0, true)
	return logFindMatches(lv.result.Matches)
}

func (l *logModeState) extendSearch(lv *logView) {
	if lv.closed.Load() || lv.searchRE == nil || lv.result.Limited || lv.search.Pending() || lv.searchSize >= lv.file.Progress().Size {
		return
	}
	lv.searchVersion++
	l.startSearch(lv, lv.searchSize, false)
}

func (l *logModeState) startSearch(lv *logView, start int64, navigate bool) {
	a := l.app
	file, re, version, post := lv.file, lv.searchRE, lv.searchVersion, a.tasks.post
	lv.search.Schedule(func(ctx context.Context) (logfile.SearchResult, error) {
		return file.SearchFrom(ctx, re, start, func(result logfile.SearchResult) {
			if post != nil {
				post(func() { l.applySearch(lv, version, result, false, navigate) })
			}
		})
	}, func(result logfile.SearchResult, err error) {
		if lv.closed.Load() || lv.searchVersion != version {
			return
		}
		if err != nil {
			if bar, ok := a.Root.TopOverlayWidget().(*ui.FindBarWidget); ok && l.active() == lv {
				bar.Progress = ""
			}
			a.StatusWarn(i18n.Format("logMode.searchFailed", i18n.Args{"error": err.Error()}))
			return
		}
		l.applySearch(lv, version, result, true, navigate)
		l.extendSearch(lv)
	})
}

func logFindMatches(matches []logfile.Match) []ui.FindMatch {
	out := make([]ui.FindMatch, len(matches))
	for i, m := range matches {
		out[i] = ui.FindMatch{Line: i, Col: m.Col, Len: m.Len}
	}
	return out
}

func (l *logModeState) applySearch(lv *logView, version uint64, result logfile.SearchResult, done, navigate bool) {
	a := l.app
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
	if l.active() != lv {
		return
	}
	if bar, ok := a.Root.TopOverlayWidget().(*ui.FindBarWidget); ok && bar.Input.Text() == lv.query && bar.Options == lv.options {
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
		a.StatusNotify(i18n.Text("logMode.searchLimited"))
	}
}

func (l *logModeState) navigateMatch(lv *logView, delta int) {
	a := l.app
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

func (l *logModeState) autoViewer(path string) (ui.Widget, error) {
	a := l.app
	threshold, enabled := a.Settings.Editor.LogModeMinBytes()
	if !enabled {
		return nil, nil
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || threshold > 0 && info.Size() <= threshold {
		return nil, nil
	}
	viewer, err := l.newViewer(path)
	if err != nil {
		return nil, err
	}
	a.StatusNotify(i18n.Format("logMode.autoOpened", i18n.Args{"filename": filepath.Base(path)}))
	return viewer, nil
}

func (a *App) OpenActiveInEditor() { a.logMode().openInEditor() }

func (l *logModeState) refresh(path string) {
	for _, lv := range l.views {
		if lv.file.Path() != path {
			continue
		}
		if err := lv.file.Refresh(); err != nil {
			l.app.StatusWarn(i18n.Format("logMode.refreshFailed", i18n.Args{"error": err.Error()}))
		}
		l.progress(lv)
	}
}

func (l *logModeState) setPost(post asynctask.Post) {
	for _, lv := range l.views {
		lv.poster.Store(&logPost{post: post})
	}
}
