package e2e

import (
	"github.com/gdamore/tcell/v3"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if left == right || left.Editor.DebugMarker(0) == 0 || left.Editor.DebugMarker(0) != right.Editor.DebugMarker(0) {
		t.Fatal("split did not show the shared breakpoint")
	}
	h.exec("debug.toggleBreakpoint")
	if right.Editor.DebugMarker(0) != 0 || left.Editor.DebugMarker(0) != 0 {
		t.Fatal("removal from split left a stale gutter marker")
	}
}
