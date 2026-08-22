package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenPluginTabReplacementClosesPreviousContent(t *testing.T) {
	group := NewEditorGroupWidget(nil, 4, true, "relative")
	closed := false
	notified := ""
	group.OnContentTabClose = func(id string) { notified = id }
	first := NewCommitDetailWidget("/repo", "ref", "abcdef0", false)
	first.OnClose = func() { closed = true }
	group.OpenPluginTab("commit", "Commit", first)
	group.OpenPluginTab("commit", "Replacement", NewCommitDetailWidget("/repo", "other", "1234567", false))
	if !closed {
		t.Fatal("replacing a content tab did not close its previous incarnation")
	}
	if notified != "commit" {
		t.Fatalf("replacement close notification = %q, want commit", notified)
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
	g.OpenFile(path)

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
	os.WriteFile(path, []byte("hello\nworld"), 0644)

	g := NewEditorGroupWidget(nil, 4, false, "extended")
	var errMsg string
	g.OnError = func(msg string) { errMsg = msg }

	g.OpenFile(path)

	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if g.tabs[g.active].FilePath != path {
		t.Fatalf("expected active tab to be %s, got %s", path, g.tabs[g.active].FilePath)
	}
}

func TestEditorGroupSaveError(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	var errMsg string
	g.OnError = func(msg string) { errMsg = msg }

	g.tabs[0].FilePath = "/nonexistent/dir/file.txt"
	g.tabs[0].Virtual = false
	g.tabs[0].Buf.Lines = []string{"test"}

	ok := g.Save()
	if ok {
		t.Fatal("Save should return false on error")
	}
	if errMsg == "" {
		t.Fatal("expected OnError to be called for save failure")
	}
}

func TestEditorGroupSaveSuccess(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "out.txt")

	g := NewEditorGroupWidget(nil, 4, false, "extended")
	var errMsg string
	g.OnError = func(msg string) { errMsg = msg }

	g.tabs[0].FilePath = path
	g.tabs[0].Virtual = false
	g.tabs[0].Buf.Lines = []string{"saved content"}

	ok := g.Save()
	if !ok {
		t.Fatal("Save should return true on success")
	}
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "saved content" {
		t.Fatalf("unexpected file content: %q", string(data))
	}
}

func TestEditorGroupSaveAsError(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	var errMsg string
	g.OnError = func(msg string) { errMsg = msg }

	g.SaveAs("/nonexistent/dir/file.txt")

	if errMsg == "" {
		t.Fatal("expected OnError to be called for save-as failure")
	}
	if g.tabs[0].FilePath != "untitled" {
		t.Fatal("FilePath should not change on failed SaveAs")
	}
}

func TestEditorGroupSaveAsSuccess(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "saved.go")

	g := NewEditorGroupWidget(nil, 4, false, "extended")
	var errMsg string
	g.OnError = func(msg string) { errMsg = msg }

	g.tabs[0].Buf.Lines = []string{"package main"}
	g.SaveAs(path)

	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if g.tabs[0].FilePath != path {
		t.Fatalf("expected FilePath to be %s, got %s", path, g.tabs[0].FilePath)
	}
	if g.tabs[0].Virtual {
		t.Fatal("SaveAs should clear Virtual flag")
	}
}
