package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestDebugSaveFailurePreventsSession(t *testing.T) {
	for _, request := range []string{"launch", "attach"} {
		t.Run(request, func(t *testing.T) {
			h := newTestHarness(t, 120, 40)
			defer h.stop()
			h.app.EditorGroup.OnError = h.app.StatusError
			path := filepath.Join(h.dir, "alpha.txt")
			h.openFile(path)
			h.pressRune('x')
			requireNoError(t, os.Remove(path))
			requireNoError(t, os.Mkdir(path, 0755))
			requireNoError(t, os.Mkdir(filepath.Join(h.dir, ".ttt"), 0755))
			fields := `"request":"launch","program":"."`
			if request == "attach" {
				fields = `"request":"attach","mode":"remote","port":1`
			}
			requireNoError(t, os.WriteFile(filepath.Join(h.dir, ".ttt", "launch.json"), []byte(`{"configurations":[{"name":"test","type":"go",`+fields+`}]}`), 0600))
			h.exec("debug.start")
			if h.app.Root.HasModalOverlay() {
				h.pressKey(tcell.KeyEnter, tcell.ModNone)
			}
			lines := h.app.Output.Lines
			if len(lines) == 0 || !strings.Contains(lines[len(lines)-1].Message, "Failed to save") {
				t.Fatalf("missing save error in Output: %v", lines)
			}
			if !h.app.EditorGroup.IsDirtyPath(path) || h.app.Sidebar.ActivePanel == "debug" {
				t.Fatal("failed save started debugger or discarded edits")
			}
		})
	}
}

func TestDebugBreakpointSharedAcrossSplits(t *testing.T) {
	h := newTestHarness(t, 140, 32)
	defer h.stop()
	path := filepath.Join(h.dir, "alpha.txt")
	h.openFile(path)
	left := h.app.EditorGroup
	h.exec("debug.toggleBreakpoint")
	h.exec("editor.split")
	right := h.app.EditorGroup
	requireNoError(t, right.OpenFile(path))
	l, lok := left.Editor.VisibleBookmark(0)
	r, rok := right.Editor.VisibleBookmark(0)
	if left == right || !lok || !rok || l != r {
		t.Fatal("split did not show the shared breakpoint")
	}
	h.exec("debug.toggleBreakpoint")
	_, lok = left.Editor.VisibleBookmark(0)
	_, rok = right.Editor.VisibleBookmark(0)
	if lok || rok {
		t.Fatal("removal from split left a stale gutter marker")
	}
}

func TestDebugPanelAvailableBeforeConfiguration(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	defer h.stop()
	if !h.app.Sidebar.HasPanel("debug") {
		t.Fatal("Debug missing from sidebar on startup")
	}
	h.exec("sidebar.debug")
	h.assertContains("[Run & Debug]")
	h.assertContains("Breakpoints")
	h.pressKey(tcell.KeyEnter, tcell.ModNone)
	if lines := h.app.Output.Lines; len(lines) == 0 || !strings.Contains(lines[len(lines)-1].Message, ".ttt/launch.json") {
		t.Fatal("Start action did not explain where to define a target")
	}
}

func TestDebugSidebarSectionDividers(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	defer h.stop()
	h.openFile(filepath.Join(h.dir, "alpha.txt"))
	h.exec("sidebar.debug")
	row := func(label string) int {
		t.Helper()
		for y := 0; y < 40; y++ {
			if strings.Contains(h.screenRow(y), label) {
				return y
			}
		}
		t.Fatalf("missing %s", label)
		return -1
	}
	controls, variables := row("[Run & Debug]"), row("Variables")
	for _, label := range []string{"Call Stack", "Breakpoints"} {
		from := row(label) - 1
		for _, event := range []struct {
			y      int
			button tcell.ButtonMask
		}{{from, tcell.Button1}, {from - 2, tcell.Button1}, {from - 2, tcell.ButtonNone}} {
			h.app.Root.HandleEvent(tcell.NewEventMouse(8, event.y, event.button, tcell.ModNone))
			h.redraw()
		}
		h.exec("debug.toggleBreakpoint")
		if row(label) != from-1 || row("[Run & Debug]") != controls || row("Variables") != variables {
			t.Fatalf("dragging divider above %s moved toolbar or failed to resize section", label)
		}
	}
}

func TestDebugIconsSwitchInOpenSidebar(t *testing.T) {
	h := newTestHarness(t, 120, 40)
	defer h.stop()
	defer icons.Configure("", nil)
	h.openFile(filepath.Join(h.dir, "alpha.txt"))
	h.exec("sidebar.debug")
	h.exec("debug.toggleBreakpoint")
	for _, dot := range []rune{'∙', '\uf111'} {
		h.assertContains("[Run & Debug]")
		mark, ok := h.app.EditorGroup.Editor.VisibleBookmark(0)
		if !ok || mark.Icon != dot || mark.Style != term.StyleDebugBreakpoint || strings.Count(h.screenText(), string(dot)) < 2 {
			t.Fatalf("expected a red %c in the gutter and the matching breakpoint list icon", dot)
		}
		h.exec("options.toggleFontIcons")
	}
}
