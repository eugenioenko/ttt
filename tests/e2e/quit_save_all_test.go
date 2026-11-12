package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

func TestMouseQuitRunsOnRelease(t *testing.T) {
	h := newTestHarness(t, 100, 28)
	defer h.stop()

	h.click(h.app.MenuBar.ItemAnchorX(0), h.app.MenuBar.GetRect().Y)
	menu := focusedContextMenu(t, h)
	quitIndex := -1
	for i, item := range menu.Items {
		if item.Command == "editor.quit" {
			quitIndex = i
			break
		}
	}
	if quitIndex < 0 {
		t.Fatal("File menu does not contain Quit")
	}
	r := menu.GetRect()
	x, y := r.X+2, r.Y+1+quitIndex

	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
	if !*h.app.Running {
		t.Fatal("mouse-down exited the editor before the mouse-up event")
	}
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone))
	if *h.app.Running {
		t.Fatal("mouse-up did not exit the editor")
	}
}

func TestQuitSaveAllVisibility(t *testing.T) {
	for _, tc := range []struct {
		name      string
		enabled   bool
		exception bool
		unnamed   bool
		dirty     bool
		want      bool
	}{
		{name: "default"},
		{name: "exception alone", exception: true},
		{name: "enabled", enabled: true, want: true},
		{name: "named only", enabled: true, exception: true, want: true},
		{name: "clean unnamed", enabled: true, unnamed: true, want: true},
		{name: "dirty unnamed", enabled: true, unnamed: true, dirty: true, want: true},
		{name: "except clean unnamed", enabled: true, exception: true, unnamed: true},
		{name: "except dirty unnamed", enabled: true, exception: true, unnamed: true, dirty: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t, 100, 28)
			defer h.stop()
			h.app.Settings.Editor.SaveAllOnQuit = tc.enabled
			h.app.Settings.Editor.SaveAllOnQuitExceptUnnamed = tc.exception
			h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
			h.typeText("edited")
			if tc.unnamed {
				h.exec("file.new")
				if tc.dirty {
					h.typeText("scratch")
				}
			}
			h.exec("editor.quit")
			h.assertContains("Unsaved changes")
			if got := strings.Contains(h.screenText(), "Save All & Quit"); got != tc.want {
				t.Fatalf("Save All & Quit visible = %v, want %v:\n%s", got, tc.want, h.screenText())
			}
			h.pressKey(tcell.KeyEnter, tcell.ModNone)
			if !*h.app.Running || h.app.Root.HasOverlay() {
				t.Fatal("Cancel must remain the default action")
			}
		})
	}
}

func TestQuitSaveAllNamedBuffers(t *testing.T) {
	h := newTestHarness(t, 100, 28)
	defer h.stop()
	h.app.Settings.Editor.SaveAllOnQuit = true
	h.app.EditorGroup.TrimTrailingWhitespace = true
	for _, name := range []string{"alpha.txt", "beta.txt"} {
		h.app.EditorGroup.OpenFile(filepath.Join(h.dir, name))
		h.pressKey(tcell.KeyEnd, tcell.ModNone)
		h.typeText("edited   ")
	}
	h.exec("settings.openUI")
	h.exec("editor.quit")
	h.pressRune('s')
	if *h.app.Running || h.app.EditorGroup.AnyDirty() {
		t.Fatal("Save All & Quit must save every dirty buffer and exit")
	}
	for _, name := range []string{"alpha.txt", "beta.txt"} {
		if got := readFile(t, filepath.Join(h.dir, name)); !strings.Contains(got, "edited") || !strings.HasSuffix(got, "\n") || strings.Contains(got, "   ") {
			t.Fatalf("%s was not saved with save cleanups: %q", name, got)
		}
	}
}

func TestQuitSaveAllUnnamedBuffers(t *testing.T) {
	h := newTestHarness(t, 100, 28)
	defer h.stop()
	h.app.Settings.Editor.SaveAllOnQuit = true
	h.exec("file.new")
	firstName := h.app.EditorGroup.ActiveFileName()
	h.typeText("first scratch")
	h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
	h.typeText("named edit")
	h.exec("file.new")
	secondName := h.app.EditorGroup.ActiveFileName()
	h.typeText("second scratch")
	h.exec("file.new")
	h.exec("editor.quit")
	h.pressRune('s')
	h.assertContains("Save As")
	if got := readFile(t, filepath.Join(h.dir, "alpha.txt")); !strings.Contains(got, "named edit") {
		t.Fatal("named buffers must be saved before prompting for unnamed buffers")
	}
	for i, text := range []string{"first scratch", "second scratch"} {
		h.assertContains("Save As: " + []string{firstName, secondName}[i])
		if !strings.Contains(strings.Join(h.app.EditorGroup.ActiveBuffer().Lines, "\n"), text) {
			t.Fatalf("Save As did not switch to buffer %d", i)
		}
		path := filepath.Join(h.dir, text+".txt")
		h.typeText(path)
		h.pressKey(tcell.KeyEnter, tcell.ModNone)
		if got := readFile(t, path); strings.TrimSpace(got) != text {
			t.Fatalf("saved buffer %d = %q, want %q", i, got, text)
		}
		if i == 0 {
			if !*h.app.Running {
				t.Fatal("quit before the second unnamed buffer was saved")
			}
			h.assertContains("Save As")
		}
	}
	if *h.app.Running || h.app.Root.HasOverlay() {
		t.Fatal("must quit after saving the last modified buffer without prompting for the clean unnamed buffer")
	}
}

func TestQuitSaveAllCancellation(t *testing.T) {
	for _, cancel := range []string{"escape", "button", "replace"} {
		t.Run(cancel, func(t *testing.T) {
			h := newTestHarness(t, 100, 28)
			defer h.stop()
			h.app.Settings.Editor.SaveAllOnQuit = true
			named := filepath.Join(h.dir, "alpha.txt")
			h.app.EditorGroup.OpenFile(named)
			h.typeText("named edit")
			h.exec("file.new")
			h.typeText("scratch")
			h.exec("editor.quit")
			h.pressRune('s')
			h.assertContains("Save As")
			if cancel == "replace" {
				h.typeText(named)
				h.pressKey(tcell.KeyEnter, tcell.ModNone)
				h.assertContains("already exists")
			}
			if cancel == "escape" {
				h.pressKey(tcell.KeyEscape, tcell.ModNone)
			} else {
				clickRowControl(t, h, "Cancel", "Cancel")
			}
			if !*h.app.Running || h.app.Root.HasOverlay() || !h.app.EditorGroup.IsDirty() {
				t.Fatal("cancelling a save must abort quitting and preserve the dirty buffer")
			}
			if got := readFile(t, named); !strings.Contains(got, "named edit") || strings.Contains(got, "scratch") {
				t.Fatalf("cancel must preserve the earlier save: %q", got)
			}
			h.exec("editor.quit")
			if !*h.app.Running {
				t.Fatal("a new quit attempt must prompt again")
			}
			h.assertContains("Unsaved changes")
		})
	}
}

func TestQuitSaveAllSaveGuards(t *testing.T) {
	for _, guard := range []string{"disk change", "read only", "named failure", "unnamed failure", "unnamed failure long path", "replace"} {
		t.Run(guard, func(t *testing.T) {
			h := newTestHarness(t, 100, 28)
			defer h.stop()
			h.app.Settings.Editor.SaveAllOnQuit = true
			path := filepath.Join(h.dir, "alpha.txt")
			h.app.EditorGroup.OpenFile(path)
			h.typeText("edited")
			switch guard {
			case "disk change":
				modifyOnDisk(t, path, "external\n")
			case "read only":
				h.app.EditorGroup.ActiveBuffer().ReadOnlyOnDisk = true
			case "named failure":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(path, 0755); err != nil {
					t.Fatal(err)
				}
			case "unnamed failure", "unnamed failure long path", "replace":
				h.exec("file.new")
				h.typeText("scratch")
			}
			h.exec("editor.quit")
			h.pressRune('s')
			switch guard {
			case "disk change":
				h.assertContains("modified on disk")
				h.pressRune('c')
				if got := readFile(t, path); got != "external\n" {
					t.Fatalf("cancel overwrote external changes: %q", got)
				}
			case "read only":
				h.assertContains("read-only")
				h.pressRune('c')
			case "named failure":
				h.pressRune('o')
			case "unnamed failure", "unnamed failure long path":
				dir := filepath.Join(h.dir, "missing")
				if guard == "unnamed failure long path" {
					// The space before the suffix forces "does not exist" across rows.
					dir = filepath.Join(h.dir, "long temporary directory "+strings.Repeat("x", 40))
				}
				buf := h.app.EditorGroup.ActiveBuffer()
				h.typeText(filepath.Join(dir, "scratch.txt"))
				h.pressKey(tcell.KeyEnter, tcell.ModNone)
				adapter, ok := h.app.Root.TopOverlayWidget().(*ui.WidgetAdapter)
				if !ok {
					t.Fatalf("expected a missing-directory dialog, got %T", h.app.Root.TopOverlayWidget())
				}
				dialog, ok := adapter.Inner().(*widgets.DialogWidget)
				if !ok {
					t.Fatalf("expected DialogWidget, got %T", adapter.Inner())
				}
				message, ok := dialog.Content.(*widgets.ParagraphWidget)
				if !ok {
					t.Fatalf("expected a dialog message, got %T", dialog.Content)
				}
				want := "The folder " + dir + " does not exist. Create it and save the file?"
				if message.Text != want {
					t.Fatalf("missing-directory message = %q, want %q", message.Text, want)
				}
				if guard == "unnamed failure long path" && strings.Contains(h.screenText(), "does not exist") {
					t.Fatal("long-path fixture must split the missing-directory phrase across rows")
				}
				clickRowControl(t, h, "Cancel", "Cancel")
				if h.app.EditorGroup.ActiveBuffer() != buf || !h.app.EditorGroup.IsDirty() || strings.Join(buf.Lines, "\n") != "scratch" {
					t.Fatal("cancelling the missing-directory prompt must preserve the dirty buffer and its contents")
				}
			case "replace":
				h.typeText(path)
				h.pressKey(tcell.KeyEnter, tcell.ModNone)
				h.assertContains("already exists")
				h.pressRune('r')
				if *h.app.Running || strings.TrimSpace(readFile(t, path)) != "scratch" {
					t.Fatal("confirming Replace must save and finish quitting")
				}
				return
			}
			if !*h.app.Running || !h.app.EditorGroup.AnyDirty() || h.app.Root.HasOverlay() {
				t.Fatal("a cancelled or failed save must abort quitting")
			}
		})
	}
}

func TestQuitEscapeResetsConfirmation(t *testing.T) {
	h := newTestHarness(t, 100, 28)
	defer h.stop()
	h.exec("file.new")
	h.typeText("scratch")
	h.exec("editor.quit")
	h.pressKey(tcell.KeyEscape, tcell.ModNone)
	h.exec("editor.quit")
	if !*h.app.Running {
		t.Fatal("Escape must cancel the pending quit")
	}
	h.assertContains("Unsaved changes")
	h.pressCtrl(tcell.KeyCtrlQ)
	if *h.app.Running {
		t.Fatal("second Ctrl+Q must still force quit")
	}
}

func TestQuitWithOpenOverlay(t *testing.T) {
	for _, tc := range []struct {
		name  string
		dirty bool
	}{
		{name: "clean session"},
		{name: "dirty session", dirty: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHarness(t, 100, 28)
			defer h.stop()
			h.app.EditorGroup.OpenFile(filepath.Join(h.dir, "alpha.txt"))
			if tc.dirty {
				h.typeText("edited")
			}
			h.pressCtrl(tcell.KeyCtrlP)
			if len(h.app.Root.Overlays) != 1 {
				t.Fatal("expected the command palette overlay")
			}
			overlay := h.app.Root.Overlays[0].Widget
			h.pressCtrl(tcell.KeyCtrlQ)
			if *h.app.Running != tc.dirty {
				t.Fatalf("running = %v, want %v", *h.app.Running, tc.dirty)
			}
			if tc.dirty {
				h.assertContains("Unsaved changes")
				if len(h.app.Root.Overlays) != 2 || h.app.Root.Overlays[0].Widget != overlay {
					t.Fatal("quit confirmation must appear above the existing overlay")
				}
				h.pressKey(tcell.KeyEscape, tcell.ModNone)
				if h.app.Root.TopOverlayWidget() != overlay || h.app.Root.Focused != overlay {
					t.Fatal("canceling quit must restore the existing overlay and focus")
				}
			}
		})
	}
}

func TestQuitWithSearchOverlayOpen(t *testing.T) {
	for _, key := range []tcell.Key{tcell.KeyCtrlF, tcell.KeyCtrlR, tcell.KeyCtrlP} {
		for _, action := range []string{"force", "save"} {
			t.Run(tcell.KeyNames[key]+"/"+action, func(t *testing.T) {
				h := newTestHarness(t, 120, 30)
				defer h.stop()
				h.app.Settings.Editor.SaveAllOnQuit = true
				h.exec("file.new")
				if key == tcell.KeyCtrlP {
					h.typeText("unsaved scratch")
				}
				h.pressCtrl(key)
				bar := h.app.Root.TopOverlayWidget()
				input := bar.(ui.InputHolder).FocusedInput()
				h.typeText("needle")
				query := input.Text()
				if key != tcell.KeyCtrlP {
					r := h.app.EditorGroup.Editor.GetRect()
					h.click(r.X+6, r.Y+2)
					h.typeText("unsaved scratch")
				}
				h.pressCtrl(tcell.KeyCtrlQ)
				h.assertContains("Unsaved changes")
				if !*h.app.Running || !h.app.Root.HasModalOverlay() {
					t.Fatal("Ctrl+Q must ask before discarding the dirty buffer")
				}
				h.pressKey(tcell.KeyEscape, tcell.ModNone)
				if !*h.app.Running || h.app.Root.TopOverlayWidget() != bar || input.Text() != query || editorText(h) != "unsaved scratch" {
					t.Fatal("canceling quit must preserve the buffer and search overlay")
				}
				h.pressCtrl(tcell.KeyCtrlQ)
				h.assertContains("Unsaved changes")
				if action == "force" {
					h.pressCtrl(tcell.KeyCtrlQ)
				} else {
					h.pressRune('s')
					h.assertContains("Save As")
					path := filepath.Join(h.dir, "saved-scratch.txt")
					h.typeText(path)
					h.pressKey(tcell.KeyEnter, tcell.ModNone)
					if got := readFile(t, path); strings.TrimSpace(got) != "unsaved scratch" {
						t.Fatalf("saved text = %q", got)
					}
				}
				if *h.app.Running {
					t.Fatal("confirmed quit did not exit")
				}
			})
		}
	}
}
