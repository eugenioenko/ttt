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
