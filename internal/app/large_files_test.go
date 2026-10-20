package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/gdamore/tcell/v3"
)

func reloadLimitApp(t *testing.T, limit int) (*App, string) {
	t.Helper()
	s := config.DefaultSettings()
	s.Editor.LargeFiles = &config.LargeFileSettings{AutoReloadMaxMB: &limit}
	a := buildTestApp(t, s)
	path := filepath.Join(t.TempDir(), "watched.txt")
	writeReloadFixture(t, path, "original\n")
	a.EditorGroup.OpenFile(path)
	a.EditorGroup.CommitActiveTab()
	return a, path
}

func writeReloadFixture(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

func reloadSegment(a *App) *view.StatusSegment {
	for _, segment := range a.Status.LeftSegments() {
		if segment.ID == "reload" {
			return segment
		}
	}
	return nil
}

func TestAutoReloadSizeLimit(t *testing.T) {
	for _, test := range []struct {
		name        string
		limit, size int
		stale       bool
	}{
		{"small", 1, 100, false},
		{"boundary", 1, 1 << 20, false},
		{"large", 1, (1 << 20) + 1, true},
		{"unlimited", 0, (1 << 20) + 1, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			a, path := reloadLimitApp(t, test.limit)
			text := "updated\n" + strings.Repeat("x", test.size-8)
			writeReloadFixture(t, path, text)
			a.HandleFileChanged(path)
			if got := reloadSegment(a) != nil; got != test.stale {
				t.Fatalf("reload action shown = %v, want %v", got, test.stale)
			}
			want := "updated"
			if test.stale {
				want = "original"
			}
			if got := a.EditorGroup.ActiveBuffer().Lines[0]; got != want {
				t.Fatalf("buffer starts with %q, want %q", got, want)
			}
		})
	}
}

func TestStaleReloadActionTracksTabsAndReloads(t *testing.T) {
	a, path := reloadLimitApp(t, 1)
	text := "updated\n" + strings.Repeat("x", 1<<20)
	writeReloadFixture(t, path, text)
	a.HandleFileChanged(path)
	a.HandleFileChanged(path)
	if a.Status.Notification != "" {
		t.Fatalf("repeated changes produced a notification: %q", a.Status.Notification)
	}
	other := filepath.Join(t.TempDir(), "other.txt")
	writeReloadFixture(t, other, "other\n")
	a.EditorGroup.OpenFile(other)
	a.SyncReloadSegment()
	if reloadSegment(a) != nil {
		t.Fatal("another tab displayed the stale file's reload action")
	}
	a.EditorGroup.SwitchTab(0)
	a.SyncReloadSegment()
	segment := reloadSegment(a)
	if segment == nil || segment.OnClick == nil {
		t.Fatal("returning to the stale tab did not restore its reload action")
	}
	segment.OnClick()
	if reloadSegment(a) != nil || a.EditorGroup.ActiveBuffer().Lines[0] != "updated" {
		t.Fatal("clicking Reload did not load the new contents and clear the action")
	}
	writeReloadFixture(t, path, text+"more")
	a.HandleFileChanged(path)
	if !a.EditorGroup.Save() {
		t.Fatal("saving the current buffer failed")
	}
	a.SyncReloadSegment()
	if reloadSegment(a) != nil {
		t.Fatal("saving the file kept its stale reload action")
	}
	writeReloadFixture(t, path, text+"more again")
	a.HandleFileChanged(path)
	a.EditorGroup.CloseTab()
	a.SyncReloadSegment()
	if a.largeFiles.stale[path] || reloadSegment(a) != nil {
		t.Fatal("closing the stale tab kept its reload state")
	}
}

func TestReloadProtectsUnsavedChanges(t *testing.T) {
	a, path := reloadLimitApp(t, 1)
	a.Root.SetSize(80, 24)
	a.EditorGroup.Editor.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "Z", 0))
	buf := a.EditorGroup.ActiveBuffer()
	before := buf.Lines[0]
	writeReloadFixture(t, path, "updated\n"+strings.Repeat("x", 1<<20))
	a.HandleFileChanged(path)
	if buf.Lines[0] != before || reloadSegment(a) != nil || a.Status.Notification == "" {
		t.Fatal("external change did not preserve the dirty-buffer policy")
	}
	a.ReloadActiveFile()
	if !a.Root.HasOverlay() || buf.Lines[0] != before {
		t.Fatal("manual reload did not ask before discarding unsaved changes")
	}
	a.Root.HandleEvent(tcell.NewEventKey(tcell.KeyEscape, "", 0))
	if a.Root.HasOverlay() || buf.Lines[0] != before || !buf.Dirty {
		t.Fatal("cancelling reload changed the dirty buffer")
	}
	a.ReloadActiveFile()
	a.Root.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", 0))
	if a.Root.HasOverlay() || buf.Lines[0] != "updated" || buf.Dirty {
		t.Fatal("confirmed reload did not replace the dirty buffer")
	}
}

func TestFailedReloadPreservesBufferAndUndo(t *testing.T) {
	a, path := reloadLimitApp(t, 1)
	a.EditorGroup.Editor.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "Z", 0))
	buf := a.EditorGroup.ActiveBuffer()
	before := buf.Lines[0]
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if a.EditorGroup.ReloadFile(path) {
		t.Fatal("reloading a deleted file succeeded")
	}
	if buf.Lines[0] != before || !buf.Dirty {
		t.Fatal("failed reload changed the buffer")
	}
	a.EditorGroup.Undo()
	if buf.Lines[0] != "original" {
		t.Fatal("failed reload discarded undo history")
	}
}
