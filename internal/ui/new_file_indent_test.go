package ui

import "testing"

func TestNewUntitledFilesUseConfiguredIndent(t *testing.T) {
	for _, insertSpaces := range []bool{false, true} {
		g := NewEditorGroupWidget(nil, 2, true, "relative")
		g.TabSize = 2
		g.InsertSpaces = insertSpaces

		g.NewFile()
		if g.Editor.UseTabs == insertSpaces || g.Editor.TabSize != 2 {
			t.Errorf("NewFile with insertSpaces=%v: UseTabs=%v TabSize=%d", insertSpaces, g.Editor.UseTabs, g.Editor.TabSize)
		}

		g.NewFileFromText("abc")
		if g.Editor.UseTabs == insertSpaces || g.Editor.TabSize != 2 {
			t.Errorf("NewFileFromText with insertSpaces=%v: UseTabs=%v TabSize=%d", insertSpaces, g.Editor.UseTabs, g.Editor.TabSize)
		}
	}
}
