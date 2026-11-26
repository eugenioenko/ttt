package ui

import (
	"os"
	"path/filepath"
	"testing"
)

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
