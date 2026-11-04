package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

func TestEditorLinkNavigationThroughApp(t *testing.T) {
	h := newTestHarness(t, 100, 25)
	defer h.stop()
	source := filepath.Join(h.dir, "subdir", "links.txt")
	target := filepath.Join(h.dir, "subdir", "target.txt")
	for path, text := range map[string]string{
		source: "https://example.com\ntarget.txt:2:3",
		target: "first\nsecond target\nthird",
	} {
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.app.EditorGroup.OpenFile(source); err != nil {
		t.Fatal(err)
	}
	h.app.EditorGroup.CommitActiveTab()
	h.app.FocusEditor()
	var urls []string
	h.app.EditorGroup.OnOpenURL = func(s string) { urls = append(urls, s) }
	h.redraw()
	e := h.app.EditorGroup.Editor
	r := e.GetRect()
	x, y := r.X+e.GutterWidth()+2, r.Y
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, 0, tcell.ModCtrl))
	h.redraw()
	if shape := h.app.Root.PointerShapeAt(x, y); shape != term.PointerHand {
		t.Fatalf("link pointer = %q", shape)
	}
	for range 2 {
		h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModCtrl))
		h.redraw()
	}
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, 0, tcell.ModCtrl))
	if len(urls) != 1 || urls[0] != "https://example.com" || e.Selection.Active {
		t.Fatalf("URLs=%v selection=%+v", urls, e.Selection)
	}
	h.app.Root.HandleEvent(tcell.NewEventMouse(0, 0, 0, tcell.ModCtrl))
	h.redraw()
	if shape := h.app.Root.PointerShapeAt(x, y); shape == term.PointerHand {
		t.Fatal("link hover survived leaving the pane")
	}
	h.app.Screen.SetPointerShapesEnabled(false)
	y++
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, 0, 0))
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModCtrl))
	h.redraw()
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModCtrl))
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, 0, tcell.ModCtrl))
	if got := h.app.EditorGroup.ActiveFilePath(); got != target {
		t.Fatalf("opened %q, want source-relative %q", got, target)
	}
	cur := h.app.EditorGroup.Editor.Cursor
	if cur.Line != 1 || cur.Col != 2 {
		t.Fatalf("file link cursor = %d:%d", cur.Line, cur.Col)
	}
	if h.app.Root.Focused != h.app.EditorGroup {
		t.Fatal("file link did not focus editor")
	}
}

func TestUntitledEditorLinkUsesWorkspace(t *testing.T) {
	h := newTestHarness(t, 100, 25)
	defer h.stop()
	h.app.EditorGroup.NewFile()
	h.app.PasteText("subdir/nested.txt:1:2")
	h.redraw()
	e := h.app.EditorGroup.Editor
	r := e.GetRect()
	x, y := r.X+e.GutterWidth()+1, r.Y
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModCtrl))
	h.redraw()
	h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, 0, tcell.ModCtrl))
	if got := h.app.EditorGroup.ActiveFilePath(); got != filepath.Join(h.dir, "subdir", "nested.txt") {
		t.Fatalf("workspace-relative file link opened %q", got)
	}
	if col := e.Cursor.Col; col != 1 {
		t.Fatalf("file link column = %d", col)
	}
}

func TestEditorLinksInReadOnlyDiffTabs(t *testing.T) {
	for _, tt := range []struct {
		name string
		mode ui.DiffMode
	}{{"unified", ui.DiffModeUnified}, {"split", ui.DiffModeSplit}} {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHarness(t, 120, 25)
			defer h.stop()
			old := []string{"https://old.test", "unchanged"}
			cur := []string{"https://new.test", "extra", "unchanged"}
			g := h.app.EditorGroup
			g.DiffMode = tt.mode
			g.OpenDiff(filepath.Join(h.dir, "links.txt"), diff.FileDiff{}, old, cur, true)
			var opened []string
			g.OnOpenURL = func(url string) { opened = append(opened, url) }
			h.redraw()
			for _, url := range []string{"https://old.test", "https://new.test"} {
				x, y := -1, -1
				for row := 0; row < h.app.Root.Height; row++ {
					if strings.Contains(h.screenRow(row), url) {
						x, y = displayColumnOf(h.screenRow(row), url)+2, row
						break
					}
				}
				if x < 0 {
					t.Fatalf("missing %s in diff:\n%s", url, h.screenText())
				}
				h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModCtrl))
				h.redraw()
				h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, 0, tcell.ModCtrl))
				if len(opened) != 1 || opened[0] != url {
					t.Fatalf("diff link opened %v, want %q", opened, url)
				}
				opened = nil
			}
		})
	}
}
