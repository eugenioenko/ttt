package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config/debugadapter"
	"github.com/eugenioenko/ttt/internal/term"
)

func TestDebugSidebarGutterRoutesToClickedSplit(t *testing.T) {
	h := newTestHarness(t, 140, 32)
	defer h.stop()
	h.app.Settings.Debug.Adapters["fixture"] = debugadapter.Config{Languages: []string{"c"}}
	left := h.app.EditorGroup
	for _, name := range []string{"left.c", "right.c"} {
		path := filepath.Join(h.dir, name)
		if err := os.WriteFile(path, []byte("int main(void) {}\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if name == "right.c" {
			h.app.SplitEditor()
		}
		if err := h.app.EditorGroup.OpenFile(path); err != nil {
			t.Fatal(err)
		}
	}
	right := h.app.EditorGroup
	r := left.Editor.GetRect()
	h.click(r.X, r.Y)
	if h.app.EditorGroup != left {
		t.Fatal("gutter click did not activate the left split")
	}
	if b, ok := left.Editor.BookmarkOverlay(0); !ok || b.Style != term.StyleDebugBreakpoint {
		t.Fatal("missing left breakpoint")
	}
	if _, ok := right.Editor.BookmarkOverlay(0); ok {
		t.Fatal("breakpoint appeared in the right split")
	}
	h.exec("sidebar.debug")
	h.assertContains("left.c:1")
	h.exec("debug.clearBreakpoints")
	if _, ok := left.Editor.BookmarkOverlay(0); ok {
		t.Fatal("cleared breakpoint still visible")
	}
}
