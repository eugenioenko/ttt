package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/gdamore/tcell/v3"
)

func openSettings(t *testing.T) *testHarness {
	t.Helper()
	h := newTestHarness(t, 120, 40)
	h.exec("settings.openUI")
	return h
}

func TestSettingsViewOpensAsTab(t *testing.T) {
	h := openSettings(t)

	screen := h.screenText()
	for _, want := range []string{
		"Settings",
		"Editor", "Appearance", "Completion", "Advanced",
		"Tab size", "Word wrap", "Insert spaces",
		"Cancel", "Apply",
	} {
		if !strings.Contains(screen, want) {
			t.Errorf("expected %q in settings view:\n%s", want, screen)
		}
	}
}

func TestSettingsViewReflectsCurrentValues(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	h.app.Settings.Editor.WordWrap = true
	h.app.Settings.Editor.LineNumbers = false
	h.exec("settings.openUI")

	if !rowHas(h, "Word wrap", checkedBox) {
		t.Errorf("word wrap should render checked:\n%s", h.screenText())
	}
	if !rowHas(h, "Line numbers", uncheckedBox) {
		t.Errorf("line numbers should render unchecked:\n%s", h.screenText())
	}
}

const (
	checkedBox   = "[x]"
	uncheckedBox = "[ ]"
)

// rowHas reports whether a single rendered row contains both the setting label
// and the given control state.
func rowHas(h *testHarness, label, state string) bool {
	for _, line := range strings.Split(h.screenText(), "\n") {
		if strings.Contains(line, label) && strings.Contains(line, state) {
			return true
		}
	}
	return false
}

func TestSettingsViewReopenReusesTab(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	base := h.app.EditorGroup.TabCount()

	h.exec("settings.openUI")
	before := h.app.EditorGroup.TabCount()
	if before != base+1 {
		t.Fatalf("opening settings did not add a tab: %d -> %d", base, before)
	}

	h.exec("settings.openUI")
	if after := h.app.EditorGroup.TabCount(); after != before {
		t.Errorf("reopening settings created a new tab: %d -> %d", before, after)
	}
}

// Edits stay in the working copy until Apply, so a stray toggle cannot change
// the editor behind the tab.
func TestSettingsEditsDeferUntilApply(t *testing.T) {
	h := openSettings(t)
	if h.app.Settings.Editor.WordWrap {
		t.Fatal("expected wordWrap off by default")
	}

	toggleWordWrap(t, h)
	if h.app.Settings.Editor.WordWrap {
		t.Error("wordWrap was applied before Apply was pressed")
	}
}

func TestSettingsApplyPersistsAndLiveApplies(t *testing.T) {
	h := openSettings(t)
	toggleWordWrap(t, h)
	h.exec("settings.apply")

	if !h.app.Settings.Editor.WordWrap {
		t.Fatal("wordWrap not written to settings after Apply")
	}
	if !h.app.EditorGroup.WordWrap {
		t.Error("wordWrap not live-applied to the editor group")
	}
	if !strings.Contains(h.screenText(), "Settings applied") {
		t.Errorf("expected confirmation in the status line:\n%s", h.screenText())
	}
}

func TestSettingsMouseTogglesAndApplies(t *testing.T) {
	h := openSettings(t)

	clickRowControl(t, h, "Word wrap", uncheckedBox)
	if !rowHas(h, "Word wrap", checkedBox) {
		t.Fatalf("clicking the checkbox did not toggle it:\n%s", h.screenText())
	}
	if h.app.Settings.Editor.WordWrap {
		t.Error("wordWrap applied before Apply")
	}

	clickRowControl(t, h, "Apply", "Apply")
	if !h.app.Settings.Editor.WordWrap {
		t.Error("clicking Apply did not persist wordWrap")
	}
}

func TestSettingsEnumSelectOpensPopup(t *testing.T) {
	h := openSettings(t)
	clickRowControl(t, h, "Appearance", "Appearance")

	// The theme select shows "Default" until it is opened.
	if strings.Contains(h.screenText(), "default-dark") {
		t.Fatal("theme list visible before the select was opened")
	}

	clickRowControl(t, h, "Default", "Default")

	screen := h.screenText()
	if !strings.Contains(screen, "▲") {
		t.Errorf("chevron should point up while open:\n%s", screen)
	}
	if !strings.Contains(screen, "default-dark") {
		t.Fatalf("expected the theme list in the open popup:\n%s", screen)
	}

	// Picking an item closes the popup and updates the displayed value.
	clickRowControl(t, h, "default-dark", "default-dark")
	screen = h.screenText()
	if strings.Contains(screen, "dracula") {
		t.Errorf("popup should close after picking an item:\n%s", screen)
	}
	if !rowHas(h, "default-dark", "▼") {
		t.Errorf("select should display the picked value:\n%s", screen)
	}

	h.exec("settings.apply")
	if h.app.Settings.Theme != "default-dark" {
		t.Errorf("theme = %q, want default-dark", h.app.Settings.Theme)
	}
}

func TestSettingsAppearanceOwnsDiffContextControl(t *testing.T) {
	h := openSettings(t)
	defer h.stop()
	clickRowControl(t, h, "Editor", "Editor")
	if rowHas(h, "Diff context", "Changes Only") {
		t.Fatalf("Editor still contains the Diff context control:\n%s", h.screenText())
	}
	clickRowControl(t, h, "Appearance", "Appearance")
	if !rowHas(h, "Diff context", "Changes Only") {
		t.Fatalf("Appearance is missing the normalized Diff context control:\n%s", h.screenText())
	}
	clickRowControl(t, h, "Diff context", "Changes Only")
	clickRowControl(t, h, "Full File", "Full File")
	h.exec("settings.apply")
	if h.app.Settings.Editor.DiffContext != config.DiffContextFull {
		t.Fatalf("Diff context = %q, want full", h.app.Settings.Editor.DiffContext)
	}
}

func TestSettingsCollapsedDiffEmphasisLiveAppliesFromAppearance(t *testing.T) {
	h := openSettings(t)
	defer h.stop()
	clickRowControl(t, h, "Editor", "Editor")
	clickRowControl(t, h, "Appearance", "Appearance")
	if !rowHas(h, "Emphasize collapsed diff rows", uncheckedBox) {
		t.Fatalf("Appearance is missing the collapsed-row emphasis setting:\n%s", h.screenText())
	}
	clickRowControl(t, h, "Emphasize collapsed diff rows", uncheckedBox)
	if h.app.Settings.Editor.DiffCollapsedEmphasis || h.app.EditorGroup.DiffCollapsedEmphasis {
		t.Fatal("collapsed-row emphasis applied before Apply")
	}
	h.exec("settings.apply")
	if !h.app.Settings.Editor.DiffCollapsedEmphasis || !h.app.EditorGroup.DiffCollapsedEmphasis {
		t.Fatalf("collapsed-row emphasis did not live-apply: settings=%v group=%v", h.app.Settings.Editor.DiffCollapsedEmphasis, h.app.EditorGroup.DiffCollapsedEmphasis)
	}
}

func TestSettingsGitFileViewLiveAppliesOnlyAfterApply(t *testing.T) {
	h := openSettings(t)
	clickRowControl(t, h, "Advanced", "Advanced")
	if !rowHas(h, "Git: file view", "List") {
		t.Fatalf("Git file view should default to List:\n%s", h.screenText())
	}
	clickRowControl(t, h, "Git: file view", "List")
	clickRowControl(t, h, "Tree", "Tree")
	if h.app.Settings.Git.FileView != config.GitFileViewList || h.app.Changes.FileView() != config.GitFileViewList {
		t.Fatal("Git file view applied before Apply")
	}
	h.exec("settings.apply")
	if h.app.Settings.Git.FileView != config.GitFileViewTree || h.app.Changes.FileView() != config.GitFileViewTree {
		t.Fatalf("Git file view did not live-apply: settings=%q panel=%q", h.app.Settings.Git.FileView, h.app.Changes.FileView())
	}
}

// runeIndex returns the screen column of sub within line. strings.Index would
// give a byte offset, which is wrong on rows containing box-drawing characters.
func runeIndex(line, sub string) int {
	b := strings.Index(line, sub)
	if b < 0 {
		return -1
	}
	return len([]rune(line[:b]))
}

// clickRowControl clicks target on the row containing label.
func clickRowControl(t *testing.T, h *testHarness, label, target string) {
	t.Helper()
	for y, line := range strings.Split(h.screenText(), "\n") {
		if !strings.Contains(line, label) {
			continue
		}
		if x := runeIndex(line, target); x >= 0 {
			h.click(x+1, y)
			return
		}
	}
	t.Fatalf("row %q with control %q not found:\n%s", label, target, h.screenText())
}

// Clicks the Word wrap checkbox. Coordinates are derived from the rendered row
// rather than a tab count, so adding fields cannot silently retarget it.
func toggleWordWrap(t *testing.T, h *testHarness) {
	t.Helper()
	clickRowControl(t, h, "Word wrap", uncheckedBox)
	if !rowHas(h, "Word wrap", checkedBox) {
		t.Fatalf("Word wrap did not toggle:\n%s", h.screenText())
	}
}

// focusTabSizeInput clicks the Tab size field and confirms keystrokes land in
// it. The enclosing scroll view is focusable too, so this doubles as a
// regression test that a click focuses the innermost control, not the container.
func focusTabSizeInput(t *testing.T, h *testHarness) {
	t.Helper()
	clickRowControl(t, h, "Tab size", "❯")
	h.pressKey(tcell.KeyEnd, tcell.ModNone)
	h.pressRune('9')
	if !rowHas(h, "Tab size", "❯ 49") {
		t.Fatalf("clicking the Tab size field did not focus it:\n%s", h.screenText())
	}
	h.pressKey(tcell.KeyBackspace2, tcell.ModNone)
}

func TestSettingsInvalidIntRevertsAndBlocksApply(t *testing.T) {
	h := openSettings(t)
	before := h.app.Settings.Editor.TabSize

	focusTabSizeInput(t, h)
	h.pressKey(tcell.KeyBackspace2, tcell.ModNone)
	// 0 is below Min, and omitempty would drop it on save if it were accepted.
	h.pressRune('0')
	h.exec("settings.apply")

	if h.app.Settings.Editor.TabSize != before {
		t.Errorf("invalid tab size was applied: %d", h.app.Settings.Editor.TabSize)
	}
	screen := h.screenText()
	if !strings.Contains(screen, "Invalid value for") {
		t.Errorf("expected a validation message:\n%s", screen)
	}
	if !strings.Contains(screen, "Editor → Tab size") {
		t.Errorf("message should name the category and field:\n%s", screen)
	}
	// The field snaps back to the last good value rather than keeping "0".
	if !rowHas(h, "Tab size", fmt.Sprintf("❯ %d ", before)) {
		t.Errorf("field did not revert to %d:\n%s", before, screen)
	}
}

func TestSettingsCancelClosesAndDiscardsPendingEdits(t *testing.T) {
	h := openSettings(t)
	tabsBefore := h.app.EditorGroup.TabCount()

	toggleWordWrap(t, h)
	clickRowControl(t, h, "Cancel", "Cancel")

	if h.app.Settings.Editor.WordWrap {
		t.Error("Cancel should not have written anything to settings")
	}
	if got := h.app.EditorGroup.TabCount(); got != tabsBefore-1 {
		t.Errorf("Cancel left the tab open: %d -> %d", tabsBefore, got)
	}

	// Reopening starts from the saved settings, not the discarded working copy.
	h.exec("settings.openUI")
	if rowHas(h, "Word wrap", checkedBox) {
		t.Errorf("discarded edit survived into the reopened tab:\n%s", h.screenText())
	}
}

// The apply/cancel commands are reachable from the palette with no settings tab
// open, so they must not panic.
func TestSettingsCommandsWithoutOpenTab(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	h.exec("settings.apply")
	h.exec("settings.cancel")
	if !strings.Contains(h.screenText(), "No settings editor open") {
		t.Errorf("expected a notice when no settings tab is open:\n%s", h.screenText())
	}
}

// EditorGroup.CursorPosition delegates to content tabs; without it the text
// inputs in the settings form render with no cursor and look inert.
func TestSettingsInputShowsCursor(t *testing.T) {
	h := openSettings(t)
	focusTabSizeInput(t, h)

	x, y, visible := h.app.Root.CursorPosition()
	if !visible {
		t.Fatalf("no cursor while a settings input is focused:\n%s", h.screenText())
	}
	line := strings.Split(h.screenText(), "\n")[y]
	if runeIndex(line, "Tab size") < 0 {
		t.Errorf("cursor on row %d, which is not the Tab size row:\n%s", y, h.screenText())
	}
	if x <= runeIndex(line, "❯") {
		t.Errorf("cursor at col %d is not inside the input:\n%s", x, h.screenText())
	}
}
