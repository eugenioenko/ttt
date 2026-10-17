package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/core/logfile"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func logFixture(t *testing.T, size int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.log")
	text := strings.Repeat("log entry\n", size/10) + "last marker\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
func waitLogIndex(t *testing.T, lv *logView) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for lv.file.Indexing() {
		if time.Now().After(deadline) {
			t.Fatal("log indexing did not complete")
		}
		time.Sleep(time.Millisecond)
	}
}
func TestAutomaticLogModeThresholdAndEditorBypass(t *testing.T) {
	small, large := logFixture(t, 100), logFixture(t, (1<<20)+100)
	for _, test := range []struct {
		name         string
		threshold    *config.MBThreshold
		small, large bool
	}{
		{"one MiB", &config.MBThreshold{MB: 1}, false, true},
		{"off", &config.MBThreshold{Off: true}, false, false},
		{"zero", &config.MBThreshold{}, true, true},
		{"default", nil, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			settings := config.DefaultSettings()
			settings.Editor.LargeFiles = &config.LargeFileSettings{LogModeMinMB: test.threshold}
			a := buildTestApp(t, settings)
			t.Cleanup(a.CloseTasks)
			t.Cleanup(a.closeLogViews)
			for _, file := range []struct {
				path string
				log  bool
			}{{small, test.small}, {large, test.large}} {
				a.EditorGroup.OpenFile(file.path)
				a.EditorGroup.CommitActiveTab()
				if got := a.activeLogView() != nil; got != file.log {
					t.Fatalf("log tab = %v, want %v", got, file.log)
				}
			}
			if a.activeLogView() != nil {
				a.OpenActiveInEditor()
				if !a.Root.HasOverlay() {
					t.Fatal("exiting log mode did not ask for confirmation")
				}
				a.Root.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))
				if a.Root.HasOverlay() || a.EditorGroup.ActiveBuffer() == nil || a.activeLogView() != nil {
					t.Fatal("Open in Editor bounced back to log mode")
				}
				a.Root.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "z", 0))
				if !a.EditorGroup.ActiveBuffer().Dirty {
					t.Fatal("confirmed editor did not accept typing")
				}
				a.EditorGroup.OpenFile(large)
				if a.EditorGroup.ActiveBuffer() == nil {
					t.Fatal("existing editor tab was converted automatically")
				}
			}
		})
	}
}
func TestAutomaticLogModeStrictBoundaryAndStartup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "boundary.log")
	if err := os.WriteFile(path, []byte(strings.Repeat("x\n", 1<<19)), 0o644); err != nil {
		t.Fatal(err)
	}
	settings := config.DefaultSettings()
	settings.Editor.LargeFiles = &config.LargeFileSettings{LogModeMinMB: &config.MBThreshold{MB: 1}}
	a := buildTestApp(t, settings)
	defer a.closeLogViews()
	a.EditorGroup.OpenFile(path)
	if a.activeLogView() != nil {
		t.Fatal("file exactly at threshold entered log mode")
	}
	settings.Editor.LargeFiles.LogModeMinMB = &config.MBThreshold{}
	cfg := config.AppConfig{Settings: settings, Theme: config.DefaultTheme(), Keybindings: config.DefaultKeybindings()}
	borders := BuildBorderSet(cfg.Theme.Borders)
	startup := BuildAppFromConfig(&cfg, &borders, a.Workspace, []FileTarget{{Path: path}})
	defer startup.closeLogViews()
	if startup.EditorGroup.ActiveBuffer() != nil || startup.activeLogView() == nil {
		t.Fatal("startup bypassed log mode hook")
	}
}
func TestLogSearchSelectionReadOnlyAndClose(t *testing.T) {
	clipboard.DisableSystem()
	a := buildTestApp(t, config.DefaultSettings())
	defer a.CloseTasks()
	defer a.closeLogViews()
	path := logFixture(t, 400000)
	a.EditorGroup.OpenFile(path)
	previousBuffer := a.EditorGroup.ActiveBuffer()
	a.Root.SetSize(80, 24)
	cells := make([][]term.Cell, 24)
	for i := range cells {
		cells[i] = make([]term.Cell, 80)
	}
	a.Root.Render(cells)
	a.openLogView(path)
	if a.EditorGroup.Editor.Buf == previousBuffer || a.EditorGroup.Editor.Highlighter != nil {
		t.Fatal("converted log retained the editor buffer")
	}
	lv := a.activeLogView()
	waitLogIndex(t, lv)
	a.OpenFind()
	bar := a.Root.TopOverlayWidget().(*ui.FindBarWidget)
	bar.Input.SetText("last marker")
	matches := bar.OnSearch("last marker", ui.SearchOptions{})
	if len(matches) != 1 {
		t.Fatalf("whole-file search found %d matches", len(matches))
	}
	bar.Current = 0
	bar.OnNavigate(matches[0])
	if lv.viewer.Cursor.Offset < 300000 {
		t.Fatal("find did not navigate past the old 20,000-line window")
	}
	a.DismissDialog()
	a.EditorGroup.MoveToFileEdge(false, false)
	a.EditorGroup.MoveToFileEdge(true, true)
	a.Copy()
	if got := clipboard.Get(); !strings.HasPrefix(got, "log entry\n") || !strings.HasSuffix(got, "last marker") {
		t.Fatalf("selection copied %d bytes with wrong ends", len(got))
	}
	if a.EditorGroup.ActiveBuffer() != nil || !a.EditorGroup.IsActiveReadOnly() {
		t.Fatal("log viewer acquired an editable buffer")
	}
	a.EditorGroup.CloseTab()
	if len(a.logViews) != 0 || !lv.closed.Load() {
		t.Fatal("closing log tab left its reader alive")
	}
	a.handleLogProgress(&LogFileProgress{Path: path, View: lv})
}
func TestLogFollowPauseRotationAndStatus(t *testing.T) {
	a := newListenTestApp(t)
	path := logFixture(t, 3000)
	if err := runExecMain(a, func() error { a.openLogView(path); a.Root.SetFocus(a.EditorGroup); return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := RunExecScript(a, `wait-for "log entry"; key ctrl+end; wait-for "last marker"`); err != nil {
		t.Fatal(err)
	}
	appendLine := func(text string) {
		t.Helper()
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		f.WriteString(text + "\n")
		f.Close()
		if err := runExecMain(a, func() error { a.HandleFileChanged(path); return nil }); err != nil {
			t.Fatal(err)
		}
	}
	appendLine("first appended marker")
	if _, err := RunExecScript(a, `wait-for "first appended marker"`); err != nil {
		t.Fatal(err)
	}
	if _, err := RunExecScript(a, `key pgup`); err != nil {
		t.Fatal(err)
	}
	appendLine("second appended marker")
	if _, err := RunExecScript(a, `wait-for "LOG 303 lines"`); err != nil {
		t.Fatal(err)
	}
	if err := runExecMain(a, func() error {
		if a.activeLogView().viewer.Following {
			return fmt.Errorf("Page Up did not pause following")
		}
		segments := a.Status.RightSegments()
		if len(segments) == 0 || segments[0].ID != "logmode" {
			return fmt.Errorf("LOG is not before position on the right")
		}
		for _, s := range segments {
			if s.ID == "indent" || s.ID == "eol" || s.ID == "language" {
				return fmt.Errorf("inapplicable status item %s is visible", s.ID)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RunExecScript(a, `key ctrl+end; wait-for "second appended marker"`); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("rotated marker\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runExecMain(a, func() error { a.HandleFileChanged(path); return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := RunExecScript(a, `wait-for "rotated marker"`); err != nil {
		t.Fatal(err)
	}
}
func TestLargeFileReloadAndStaleActions(t *testing.T) {
	settings := config.DefaultSettings()
	one := 1
	settings.Editor.LargeFiles = &config.LargeFileSettings{AutoReloadMaxMB: &one, LogModeMinMB: &config.MBThreshold{Off: true}}
	a := buildTestApp(t, settings)
	defer a.closeLogViews()
	path := logFixture(t, 100)
	a.EditorGroup.OpenFile(path)
	a.EditorGroup.CommitActiveTab()
	original := a.EditorGroup.ActiveBuffer()
	if err := os.WriteFile(path, []byte(strings.Repeat("changed\n", 150000)), 0o644); err != nil {
		t.Fatal(err)
	}
	a.HandleFileChanged(path)
	if !a.largeFiles.stale[path] || original.Lines[0] != "log entry" {
		t.Fatal("large changed file reloaded automatically")
	}
	original.Dirty = true
	a.ShowStaleFileActions()
	a.Root.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))
	if !a.Root.HasOverlay() || !original.Dirty {
		t.Fatal("stale Reload action discarded later edits without confirmation")
	}
	a.Root.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, "", 0))
	original.Dirty = false
	a.openLogView(path)
	if a.largeFiles.stale[path] || a.activeLogView() == nil {
		t.Fatal("stale file did not switch to log mode")
	}
}

func TestReloadFileFromDiskConfirmationAndDisabledLimit(t *testing.T) {
	settings := config.DefaultSettings()
	zero := 0
	settings.Editor.LargeFiles = &config.LargeFileSettings{AutoReloadMaxMB: &zero, LogModeMinMB: &config.MBThreshold{Off: true}}
	a := buildTestApp(t, settings)
	path := logFixture(t, 100)
	a.EditorGroup.OpenFile(path)
	if err := os.WriteFile(path, []byte("new contents\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.HandleFileChanged(path)
	if a.EditorGroup.ActiveBuffer().Lines[0] != "new contents" || a.largeFiles.stale[path] {
		t.Fatal("disabled limit did not reload")
	}
	a.EditorGroup.ActiveBuffer().Dirty = true
	a.ReloadActiveFile()
	if !a.Root.HasOverlay() {
		t.Fatal("dirty reload did not confirm")
	}
	a.Root.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))
	if a.Root.HasOverlay() || a.EditorGroup.ActiveBuffer().Dirty {
		t.Fatal("reload did not dismiss and refresh")
	}
}
func TestSearchPanelByteOffsetNavigation(t *testing.T) {
	settings := config.DefaultSettings()
	settings.Editor.LargeFiles = &config.LargeFileSettings{LogModeMinMB: &config.MBThreshold{}}
	a := buildTestApp(t, settings)
	defer a.closeLogViews()
	path := logFixture(t, 400000)
	a.NavigateToSearchResult(ui.SearchMatch{FilePath: path, LineNum: 40001, ColStart: 0, ByteOffset: 400000, HasByteOffset: true})
	lv := a.activeLogView()
	if lv == nil || lv.viewer.Cursor.Offset != 400000 {
		t.Fatal("search result did not use its byte offset in an automatically opened log")
	}
}

func TestDefaultLargeFileModeAndEmptyZeroThreshold(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	defer a.closeLogViews()
	path := filepath.Join(t.TempDir(), "large.log")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate((64 << 20) + 1); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte(strings.Repeat("log entry\n", 64)), 0); err != nil {
		t.Fatal(err)
	}
	f.Close()
	a.EditorGroup.OpenFile(path)
	if a.activeLogView() == nil || a.EditorGroup.ActiveBuffer() != nil {
		t.Fatal("file over 64 MiB did not default to log mode")
	}
	empty := filepath.Join(t.TempDir(), "empty.log")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	a.Settings.Editor.LargeFiles = &config.LargeFileSettings{LogModeMinMB: &config.MBThreshold{}}
	a.EditorGroup.OpenFile(empty)
	if a.activeLogView() == nil {
		t.Fatal("zero threshold excluded an empty file")
	}
}

func TestLogAppendSearchCompletesWithoutRestartingOrNavigating(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	defer a.CloseTasks()
	defer a.closeLogViews()
	path := filepath.Join(t.TempDir(), "append.log")
	if err := os.WriteFile(path, []byte("needle\nneed"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.openLogView(path)
	lv := a.activeLogView()
	var work, delivery []func()
	a.tasks.post = func(fn func()) error { delivery = append(delivery, fn); return nil }
	lv.search = NewTask[logfile.SearchResult](a, asynctask.TaskOptions{Run: func(fn func()) { work = append(work, fn) }})
	runWork := func() {
		t.Helper()
		if len(work) != 1 {
			t.Fatalf("queued searches=%d, want 1", len(work))
		}
		fn := work[0]
		work = nil
		fn()
	}
	apply := func() {
		for len(delivery) > 0 {
			fn := delivery[0]
			delivery = delivery[1:]
			fn()
		}
	}
	appendText := func(text string) {
		t.Helper()
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.WriteString(text); err != nil {
			t.Fatal(err)
		}
		f.Close()
		a.HandleFileChanged(path)
		a.handleLogProgress(&LogFileProgress{Path: path, View: lv})
	}
	a.OpenFind()
	bar := a.Root.TopOverlayWidget().(*ui.FindBarWidget)
	bar.Options = ui.SearchOptions{UseRegex: true}
	bar.Input.SetText("needle$")
	a.searchLog(lv, "needle$", bar.Options, false)
	runWork()
	version := lv.searchVersion
	appendText("le\n")
	if lv.searchVersion != version || len(work) != 0 {
		t.Fatal("append restarted a pending full-file search")
	}
	apply()
	if len(lv.result.Matches) != 1 || len(work) != 1 || lv.searchSize != 11 {
		t.Fatalf("initial search did not finish before the append scan: %+v", lv.result)
	}
	lv.viewer.MoveToFileEdge(true, false)
	version = lv.searchVersion
	appendText("needle\n")
	if lv.searchVersion != version || len(work) != 1 {
		t.Fatal("another append canceled the incremental search")
	}
	cursor := lv.viewer.Cursor
	runWork()
	if len(lv.viewer.Matches) != 1 {
		t.Fatal("running the worker cleared existing highlights")
	}
	apply()
	if len(lv.result.Matches) != 3 || lv.result.Start != 7 || len(work) != 0 {
		t.Fatalf("append matches were not merged from the unfinished line: %+v", lv.result)
	}
	if !lv.viewer.Following || lv.viewer.Cursor != cursor {
		t.Fatal("incremental search moved away from the followed tail")
	}
	if len(bar.Matches) != 3 || bar.Progress != "" {
		t.Fatal("find bar did not receive completed incremental results")
	}
	a.DismissDialog()
	appendText("needle\n")
	runWork()
	apply()
	if len(lv.result.Matches) != 4 || lv.result.Start != 21 || !lv.viewer.Following {
		t.Fatal("closed find bar triggered a full rescan or lost following")
	}
}

func TestLogAppendSearchReplacesChangedPartialMatchesAndHonorsCap(t *testing.T) {
	a := buildTestApp(t, config.DefaultSettings())
	defer a.CloseTasks()
	defer a.closeLogViews()
	path := filepath.Join(t.TempDir(), "partial.log")
	if err := os.WriteFile(path, []byte("needle\nneedle"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.openLogView(path)
	lv := a.activeLogView()
	a.searchLog(lv, "needle$", ui.SearchOptions{UseRegex: true}, false)
	if len(lv.result.Matches) != 2 {
		t.Fatal("fixture must match the unfinished line")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(" changed\n" + strings.Repeat("needle\n", logfile.MaxMatches))
	f.Close()
	a.HandleFileChanged(path)
	a.handleLogProgress(&LogFileProgress{Path: path, View: lv})
	if !lv.result.Limited || len(lv.result.Matches) != logfile.MaxMatches || lv.result.Matches[1].Offset == 7 {
		t.Fatal("incremental merge retained an invalid partial match or exceeded the match cap")
	}
}
