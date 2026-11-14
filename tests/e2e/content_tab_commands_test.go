package e2e

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

// Regression test for issue #414: pressing Ctrl+C (editor.copy) while a
// non-editor content tab (the Settings UI) was active panicked with a nil
// dereference, because such tabs have no cursor/buffer/selection.
func TestCopyOnSettingsTabDoesNotPanic(t *testing.T) {
	h := openSettings(t)

	h.pressKey(tcell.KeyCtrlC, tcell.ModCtrl)

	// The app must still be responsive and the settings tab still rendered.
	if screen := h.screenText(); !strings.Contains(screen, "Settings") {
		t.Errorf("settings view no longer rendered after ctrl+c:\n%s", screen)
	}
}

// Sibling editor commands routed through global keybindings or the palette
// must be safe no-ops while a content tab (settings) is active.
func TestEditorCommandsOnSettingsTabAreSafeNoOps(t *testing.T) {
	h := openSettings(t)

	for _, cmd := range []string{
		"editor.copy", "editor.cut", "editor.paste", "editor.selectAll",
		"editor.undo", "editor.redo",
		"editor.moveLineUp", "editor.moveLineDown", "editor.duplicateLine",
		"editor.deleteLine", "editor.joinLines",
		"editor.insertLineBelow", "editor.insertLineAbove",
		"editor.toggleComment",
		"editor.sortLinesAsc", "editor.sortLinesDesc",
		"editor.reverseLines", "editor.uniqueLines",
		"editor.upperCase", "editor.lowerCase", "editor.titleCase",
		"editor.goToMatchingBracket",
		"multicursor.selectNext", "multicursor.selectAll", "multicursor.undoCursor",
		"multicursor.addCursorAbove", "multicursor.addCursorBelow",
		"editor.splitSelectionToLines",
		"editor.moveWordLeft", "editor.moveWordRight",
		"editor.deleteWordLeft", "editor.deleteWordRight",
		"fold.toggle", "fold.collapseAll", "fold.expandAll",
		"bookmark.toggle", "bookmark.next", "bookmark.previous",
		"diff.nextHunk", "diff.prevHunk",
	} {
		h.exec(cmd)
	}

	if screen := h.screenText(); !strings.Contains(screen, "Settings") {
		t.Errorf("settings view no longer rendered after editor commands:\n%s", h.screenText())
	}
}
