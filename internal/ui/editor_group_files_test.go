package ui

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestEditorGroupOpenFileErrors(t *testing.T) {
	for _, state := range []string{"directory", "reload-directory", "reload-missing"} {
		t.Run(state, func(t *testing.T) {
			g := NewEditorGroupWidget(nil, 4, false, "extended")
			path := filepath.Join(t.TempDir(), "file.txt")
			if state != "directory" {
				if err := os.WriteFile(path, []byte("hello"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := g.OpenFile(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				g.NewFile()
			}
			if state != "reload-missing" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			err := g.OpenFile(path)
			if state == "reload-missing" {
				if err != nil || g.tabs[g.active].FilePath != path {
					t.Fatalf("missing file: error = %v, active path = %q", err, g.tabs[g.active].FilePath)
				}
			} else if err == nil || errors.Is(err, os.ErrNotExist) {
				t.Fatalf("OpenFile = %v, want a non-ENOENT error", err)
			}
		})
	}
}

func TestEditorGroupFailedReloadStillSwitchesTab(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	if err := g.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	g.NewFile()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod 0000 does not prevent reads on this platform or user")
	}
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0600) })
	if err := g.OpenFile(path); err == nil {
		t.Fatal("reload succeeded")
	}
	tab := g.tabs[g.active]
	if tab.FilePath != path || tab.Preview {
		t.Fatalf("active tab = %+v, want committed file tab", tab)
	}
}

func TestInitialTabIsVirtual(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	if !g.tabs[0].Virtual {
		t.Fatal("expected initial tab to be virtual")
	}
}

func TestNewFileAlwaysCreatesTab(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	if len(g.tabs) != 1 || g.tabs[0].FilePath != "untitled" {
		t.Fatal("expected initial untitled tab")
	}

	g.tabs[0].Buf.Lines = []string{"some content"}
	g.tabs[0].Buf.Dirty = true

	g.NewFile()
	if len(g.tabs) != 2 {
		t.Fatalf("expected 2 tabs, got %d", len(g.tabs))
	}
	if g.tabs[1].FilePath != "untitled-2" {
		t.Errorf("expected 'untitled-2', got %q", g.tabs[1].FilePath)
	}
	if g.active != 1 {
		t.Errorf("expected active tab 1, got %d", g.active)
	}
}

func TestNewFileFromTextFillsTheStartupPlaceholder(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")

	g.NewFileFromText("one\r\ntwo\r\n")
	if len(g.tabs) != 1 || g.tabs[0].FilePath != "untitled" {
		t.Fatalf("tabs = %d, first %q; want the untitled placeholder filled", len(g.tabs), g.tabs[0].FilePath)
	}
	buf := g.tabs[0].Buf
	if !slices.Equal(buf.Lines, []string{"one", "two"}) || buf.LineEnding != "\r\n" || buf.Dirty {
		t.Fatalf("buffer = %q ending %q dirty %v, want [one two] CRLF clean", buf.Lines, buf.LineEnding, buf.Dirty)
	}
	if !g.tabs[0].Virtual {
		t.Fatal("piped text should stay unsaved (virtual)")
	}
}

func TestNewFileFromTextKeepsOpenTabs(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.tabs[0].Buf.Lines = []string{"typed"}
	g.tabs[0].Buf.Dirty = true

	g.NewFileFromText("piped\n")
	if len(g.tabs) != 2 || g.tabs[1].FilePath != "untitled-2" || g.active != 1 {
		t.Fatalf("tabs = %d, active %d; want a new active untitled-2 tab", len(g.tabs), g.active)
	}
	if g.tabs[0].Buf.Lines[0] != "typed" || g.tabs[1].Buf.Lines[0] != "piped" {
		t.Fatalf("tab contents = %q, %q", g.tabs[0].Buf.Lines, g.tabs[1].Buf.Lines)
	}
}

func TestNewFileSequentialNaming(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")

	g.NewFile()
	g.NewFile()
	g.NewFile()

	if len(g.tabs) != 4 {
		t.Fatalf("expected 4 tabs, got %d", len(g.tabs))
	}
	expected := []string{"untitled", "untitled-2", "untitled-3", "untitled-4"}
	for i, want := range expected {
		if g.tabs[i].FilePath != want {
			t.Errorf("tab %d: got %q, want %q", i, g.tabs[i].FilePath, want)
		}
	}
}

func TestNewFileReusesNameAfterClose(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.NewFile() // untitled-2

	g.SwitchTab(0)
	g.CloseTab()

	if g.tabs[0].FilePath != "untitled-2" {
		t.Fatalf("expected remaining tab 'untitled-2', got %q", g.tabs[0].FilePath)
	}

	g.NewFile()
	names := make(map[string]bool)
	for _, tab := range g.tabs {
		names[tab.FilePath] = true
	}
	if !names["untitled"] {
		t.Error("expected 'untitled' to be reused after close")
	}
}

func TestEditorGroupOpenFileNotFound(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	var errMsg string
	g.OnError = func(msg string) { errMsg = msg }

	path := "/nonexistent/path/file.txt"
	requireNoError(t, g.OpenFile(path))

	if errMsg != "" {
		t.Fatalf("unexpected error for new file: %s", errMsg)
	}
	if g.tabs[g.active].FilePath != path {
		t.Fatalf("expected tab with path %q, got %q", path, g.tabs[g.active].FilePath)
	}
	if g.tabs[g.active].Buf.Lines[0] != "" {
		t.Fatal("expected empty buffer for new file")
	}
}

func TestEditorGroupOpenFileSuccess(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	requireNoError(t, os.WriteFile(path, []byte("hello\nworld"), 0644))

	g := NewEditorGroupWidget(nil, 4, false, "extended")
	var errMsg string
	g.OnError = func(msg string) { errMsg = msg }

	requireNoError(t, g.OpenFile(path))

	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if len(g.tabs) != 1 {
		t.Fatalf("expected 1 tab replacing untitled, got %d", len(g.tabs))
	}
	if g.tabs[g.active].FilePath != path {
		t.Fatalf("expected active tab to be %s, got %s", path, g.tabs[g.active].FilePath)
	}
}

func TestEditorGroupOpenFileKeepsTouchedUntitled(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	requireNoError(t, os.WriteFile(path, []byte("hello"), 0644))

	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.tabs[0].Buf.Lines = []string{"typed text"}
	g.tabs[0].Buf.Dirty = true

	requireNoError(t, g.OpenFile(path))

	if len(g.tabs) != 2 {
		t.Fatalf("expected 2 tabs when untitled is dirty, got %d", len(g.tabs))
	}
	if g.tabs[0].FilePath != "untitled" || g.tabs[1].FilePath != path {
		t.Fatalf("expected [untitled, %s], got [%s, %s]", path, g.tabs[0].FilePath, g.tabs[1].FilePath)
	}
}

func TestEditorGroupOpenFileKeepsNewFileTab(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	requireNoError(t, os.WriteFile(path, []byte("hello"), 0644))

	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.NewFile()
	g.SwitchTab(0)
	g.CloseTab()

	requireNoError(t, g.OpenFile(path))

	if len(g.tabs) != 2 {
		t.Fatalf("expected 2 tabs when user created new file, got %d", len(g.tabs))
	}
	if g.tabs[0].FilePath != "untitled-2" || g.tabs[1].FilePath != path {
		t.Fatalf("expected [untitled-2, %s], got [%s, %s]", path, g.tabs[0].FilePath, g.tabs[1].FilePath)
	}
}

func TestEditorGroupOpenFileKeepsPinnedPlaceholder(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	requireNoError(t, os.WriteFile(path, []byte("hello"), 0644))

	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.TogglePinTab()

	requireNoError(t, g.OpenFile(path))

	if len(g.tabs) != 2 {
		t.Fatalf("expected 2 tabs when placeholder is pinned, got %d", len(g.tabs))
	}
	if g.tabs[0].FilePath != "untitled" || g.tabs[1].FilePath != path {
		t.Fatalf("expected [untitled, %s], got [%s, %s]", path, g.tabs[0].FilePath, g.tabs[1].FilePath)
	}
	if g.pinnedCount != 1 {
		t.Fatalf("expected pinnedCount to remain 1, got %d", g.pinnedCount)
	}
	if g.IsActiveTabPinned() {
		t.Fatal("expected newly opened tab to not be pinned")
	}
}
