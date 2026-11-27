package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/scm"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"
)

func navigationTestApp(t *testing.T) (*App, *plugin.Plugin, string) {
	t.Helper()
	previous := config.OverrideConfigDir
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = previous })
	settings := config.DefaultSettings()
	settings.Editor.FocusOnOpen = true
	a := buildTestApp(t, settings)
	a.Reg = command.NewRegistry()
	registerWidgetCallbacks(a)
	a.EditorGroup.OnError = a.StatusError
	a.PluginManager = &plugin.Manager{}
	p := &plugin.Plugin{Name: "navigation-test"}
	a.WirePlugin(p)
	t.Cleanup(func() {
		a.CloseTasks()
		a.Repository.Close()
		a.EditorGroup.CloseAllTabs()
	})
	path := filepath.Join(t.TempDir(), "current.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.EditorGroup.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	a.EditorGroup.GoToLineCol(2, 2)
	a.Root.SetFocus(a.Search)
	return a, p, path
}

func TestFailedFileNavigationPreservesCurrentEditor(t *testing.T) {
	for _, tc := range []struct {
		name     string
		navigate func(*App, *plugin.Plugin, string)
	}{
		{"search", func(a *App, _ *plugin.Plugin, path string) { a.NavigateToSearchMatch(path, 3, 2) }},
		{"location", func(a *App, _ *plugin.Plugin, path string) { a.navigateToLocation(path, 2, 2) }},
		{"plugin", func(_ *App, p *plugin.Plugin, path string) { p.OpenFile(path, 3, false) }},
		{"readonly plugin", func(_ *App, p *plugin.Plugin, path string) { p.OpenFile(path, 3, true) }},
		{"explorer", func(a *App, _ *plugin.Plugin, path string) { a.Explorer.OnOpenFile(path) }},
		{"changes", func(a *App, _ *plugin.Plugin, path string) { a.Changes.OnOpenFile(path) }},
		{"untracked diff", func(a *App, _ *plugin.Plugin, path string) {
			a.OpenChangeDiff(filepath.Dir(path), scm.Change{Path: filepath.Base(path), Kind: scm.Untracked}, false)
		}},
		{"conflict", func(a *App, _ *plugin.Plugin, path string) { a.openConflictedFile(path, filepath.Base(path)) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, p, path := navigationTestApp(t)
			a.EditorGroup.SetSearch("one", ui.SearchOptions{}, []ui.FindMatch{{Line: 0, Col: 0, Len: 3}})
			a.Search.Input.SetText("three")
			editor := a.EditorGroup.Editor
			beforeCursor := *editor.Cursor
			beforeMatches := append([]ui.FindMatch(nil), editor.SearchMatches...)
			failedPath := t.TempDir()
			tc.navigate(a, p, failedPath)
			if a.EditorGroup.ActiveFilePath() != path || a.EditorGroup.Editor != editor {
				t.Fatal("failed open changed the active editor")
			}
			if *editor.Cursor != beforeCursor {
				t.Fatalf("cursor moved: %+v -> %+v", beforeCursor, *editor.Cursor)
			}
			if editor.SearchQuery != "one" || !reflect.DeepEqual(editor.SearchMatches, beforeMatches) {
				t.Fatal("failed open changed search highlights")
			}
			if a.Root.Focused != a.Search {
				t.Fatal("failed open stole focus")
			}
			if a.Status.NotifyLevel != view.NotifyError || a.Status.Notification == "" {
				t.Fatalf("open error was lost: %q", a.Status.Notification)
			}
		})
	}
}

func TestFileNavigationSucceedsAfterFailedOpen(t *testing.T) {
	for _, mode := range []string{"search", "location", "plugin", "readonly plugin"} {
		t.Run(mode, func(t *testing.T) {
			a, p, _ := navigationTestApp(t)
			target := filepath.Join(t.TempDir(), "target.txt")
			if err := os.Mkdir(target, 0700); err != nil {
				t.Fatal(err)
			}
			navigate := func() {
				switch mode {
				case "search":
					a.NavigateToSearchMatch(target, 3, 2)
				case "location":
					a.navigateToLocation(target, 2, 2)
				default:
					p.OpenFile(target, 3, mode == "readonly plugin")
				}
			}
			navigate()
			if err := os.Remove(target); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte("first\nsecond\né match\n"), 0600); err != nil {
				t.Fatal(err)
			}
			a.Search.Input.SetText("match")
			navigate()
			if a.EditorGroup.ActiveFilePath() != target || a.EditorGroup.Editor.Cursor.Line != 2 {
				t.Fatalf("navigation did not reach target: %s, %+v", a.EditorGroup.ActiveFilePath(), a.EditorGroup.Editor.Cursor)
			}
			if a.EditorGroup.IsActiveReadOnly() != (mode == "readonly plugin") {
				t.Fatal("incorrect read-only state")
			}
			if mode == "search" {
				if a.EditorGroup.Editor.Cursor.Col != 1 || a.EditorGroup.Editor.SearchQuery != "match" || len(a.EditorGroup.Editor.SearchMatches) != 1 {
					t.Fatal("search navigation lost UTF-8 column or highlights")
				}
			}
		})
	}
}

func TestFailedReloadPreservesInlineDiff(t *testing.T) {
	a, _, path := navigationTestApp(t)
	if !a.EditorGroup.EnableInlineDiff(path, []string{"old"}) {
		t.Fatal("could not enable inline diff")
	}
	before := a.EditorGroup.ActiveInlineDiff()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	a.Changes.OnOpenFile(path)
	if a.EditorGroup.ActiveInlineDiff() != before {
		t.Fatal("failed reload removed the existing diff")
	}
}

func TestFailedInlineDiffOpenDoesNotUseStaleBuffer(t *testing.T) {
	repo := &baselineRepository{testSCMRepository: testSCMRepository{root: t.TempDir()}, baseline: scm.Baseline{Content: "old\n", Exists: true}}
	a, _ := newBaselineTestApp(t, repo)
	path := a.EditorGroup.ActiveFilePath()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	opened, conflict, err := a.openInlineDiff(path, repo.Root(), filepath.Base(path))
	if opened || conflict || err == nil || a.EditorGroup.IsInlineDiffActive() {
		t.Fatalf("failed reload opened a diff: opened=%v conflict=%v err=%v", opened, conflict, err)
	}
}
