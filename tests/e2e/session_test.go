package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/session"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func writeLines(t *testing.T, path string, n int) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Repeat("line\n", n)), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestAutoSessionSaveRestoreRoundTrip(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	a := filepath.Join(h.dir, "sess_a.txt")
	b := filepath.Join(h.dir, "sess_b.txt")
	writeLines(t, a, 10)
	writeLines(t, b, 10)

	h.app.Settings.Session.Auto = true
	h.app.EditorGroup.OpenFile(a)
	h.app.EditorGroup.CommitActiveTab()
	h.app.EditorGroup.GoToLineCol(4, 3)
	h.app.EditorGroup.OpenFile(b)
	h.app.EditorGroup.CommitActiveTab()
	h.app.EditorGroup.GoToLineCol(7, 1)
	h.app.EditorGroup.SwitchToTabByPath(a)
	h.redraw()

	h.app.SaveSession()

	h.app.EditorGroup.CloseAllTabs()
	if got := h.app.EditorGroup.OpenFilePaths(); len(got) != 0 {
		t.Fatalf("tabs remain after close-all: %v", got)
	}

	if !h.app.RestoreSession() {
		t.Fatal("RestoreSession reported nothing restored")
	}
	if want := []string{a, b}; !reflect.DeepEqual(h.app.EditorGroup.OpenFilePaths(), want) {
		t.Errorf("restored tabs = %v, want %v", h.app.EditorGroup.OpenFilePaths(), want)
	}
	if got := h.app.EditorGroup.ActiveFilePath(); got != a {
		t.Errorf("active tab = %q, want %q", got, a)
	}
	// Cursor of the active (restored first) tab.
	if line, col := h.app.EditorGroup.ActiveCursor(); line != 3 || col != 2 {
		t.Errorf("cursor a = (%d,%d), want (3,2)", line, col)
	}
	h.app.EditorGroup.SwitchToTabByPath(b)
	if line, col := h.app.EditorGroup.ActiveCursor(); line != 6 || col != 0 {
		t.Errorf("cursor b = (%d,%d), want (6,0)", line, col)
	}
	// Restored background tabs keep an unscrolled viewport.
	if top := h.app.EditorGroup.Editor.Viewport.TopLine; top != 0 {
		t.Errorf("restored viewport top = %d, want 0", top)
	}
}

func TestAutoSessionSkipsReadonlyTabs(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	ro := filepath.Join(h.dir, "ro.txt")
	writeLines(t, ro, 5)
	ok := filepath.Join(h.dir, "ok.txt")
	writeLines(t, ok, 5)
	h.app.Settings.Session.Auto = true
	h.app.EditorGroup.OpenFileReadOnly(ro, "ro")
	h.app.EditorGroup.OpenFile(ok)
	h.app.EditorGroup.CommitActiveTab()
	h.app.SaveSession()
	h.app.EditorGroup.CloseAllTabs()

	if !h.app.RestoreSession() {
		t.Fatal("RestoreSession reported nothing restored")
	}
	for _, p := range h.app.EditorGroup.OpenFilePaths() {
		if p == ro {
			t.Error("read-only viewer tab was persisted and reopened editable")
		}
	}
	if got := h.app.EditorGroup.OpenFilePaths(); len(got) != 1 || got[0] != ok {
		t.Errorf("restored tabs = %v, want [%s]", got, ok)
	}
}

func TestAutoSessionKeysByCwdWithoutFolders(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	// File-only launches create no workspace: the session belongs to the cwd.
	h.app.Workspace = workspace.New(nil)
	fp := filepath.Join(h.dir, "lone.txt")
	writeLines(t, fp, 3)
	h.app.Settings.Session.Auto = true
	h.app.EditorGroup.OpenFile(fp)
	h.app.EditorGroup.CommitActiveTab()
	h.app.SaveSession()

	cwd, _ := os.Getwd()
	sess, err := session.Load(filepath.Join(h.dir, "config"), cwd)
	if err != nil {
		t.Fatalf("no session keyed by cwd: %v", err)
	}
	if len(sess.Tabs) != 1 || sess.Tabs[0].Path != fp {
		t.Errorf("cwd session tabs = %+v, want [%s]", sess.Tabs, fp)
	}
}

func TestAutoSessionDisabledOrEmptySavesNothing(t *testing.T) {
	h := newTestHarness(t, 100, 30)
	defer h.stop()

	fp := filepath.Join(h.dir, "plain.txt")
	writeLines(t, fp, 3)
	h.app.EditorGroup.OpenFile(fp)
	h.app.EditorGroup.CommitActiveTab()

	// Auto off with tabs open: nothing written.
	h.app.SaveSession()
	sessionsDir := filepath.Join(h.dir, "config", "sessions")
	if _, err := os.Stat(sessionsDir); !os.IsNotExist(err) {
		t.Fatal("session data written while auto sessions are off")
	}

	// Auto on with no tabs open: a planted session must survive untouched.
	h.app.Settings.Session.Auto = true
	planted := session.Session{Folders: []string{h.dir}, Tabs: []session.TabState{{Path: fp}}}
	plantedData, _ := json.Marshal(planted)
	sessPath := session.PathForDir(filepath.Join(h.dir, "config"), h.dir)
	os.MkdirAll(filepath.Dir(sessPath), 0755)
	os.WriteFile(sessPath, plantedData, 0644)
	h.app.EditorGroup.CloseAllTabs()
	h.app.SaveSession()
	if data, err := os.ReadFile(sessPath); err != nil || string(data) != string(plantedData) {
		t.Errorf("empty save clobbered the planted session: %q, err %v", data, err)
	}
}
