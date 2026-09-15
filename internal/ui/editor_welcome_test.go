package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
)

func pristineTab(lines []string, dirty, virtual bool, content Widget) *editorTab {
	buf := &buffer.Buffer{Lines: lines, Dirty: dirty}
	return &editorTab{FilePath: "untitled", Buf: buf, Cur: &cursor.Cursor{}, Virtual: virtual, Content: content}
}

func TestIsPristineUntitled(t *testing.T) {
	if !isPristineUntitled(pristineTab([]string{""}, false, true, nil)) {
		t.Error("empty clean virtual tab should be pristine")
	}
	if isPristineUntitled(pristineTab([]string{"x"}, false, true, nil)) {
		t.Error("typed-in tab should not be pristine")
	}
	if isPristineUntitled(pristineTab([]string{""}, true, true, nil)) {
		t.Error("dirty tab should not be pristine")
	}
	fileTab := pristineTab([]string{""}, false, true, nil)
	fileTab.FilePath = "a.go"
	fileTab.Virtual = false
	if isPristineUntitled(fileTab) {
		t.Error("file tab should not be pristine")
	}
	if isPristineUntitled(nil) {
		t.Error("nil tab should not be pristine")
	}
}
