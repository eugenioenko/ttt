package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/gdamore/tcell/v3"
)

func (h *testHarness) rowContaining(text string) int {
	h.t.Helper()
	_, height := h.screen.Size()
	for y := range height {
		if strings.Contains(h.screenRow(y), text) {
			return y
		}
	}
	h.t.Fatalf("no row contains %q:\n%s", text, h.screenText())
	return -1
}

func TestExplorerRightClickTargetsCompactRowSegment(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()
	clipboard.DisableSystem()

	if err := os.MkdirAll(filepath.Join(h.dir, "pkg", "inner", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.dir, "pkg", "inner", "deep", "file.go"), []byte("package deep"), 0o644); err != nil {
		t.Fatal(err)
	}
	h.exec("sidebar.explorer")
	h.app.Explorer.Reload()
	h.redraw()

	y := h.rowContaining("pkg")
	h.click(displayColumnOf(h.screenRow(y), "pkg"), y)
	h.assertContains("pkg/inner/deep")
	h.assertContains("file.go")

	for _, tc := range []struct {
		segment string
		want    string
	}{
		{"pkg", filepath.Join(h.dir, "pkg")},
		{"inner", filepath.Join(h.dir, "pkg", "inner")},
		{"deep", filepath.Join(h.dir, "pkg", "inner", "deep")},
	} {
		row := h.screenRow(y)
		h.rightClick(displayColumnOf(row, "pkg/inner/deep")+strings.Index("pkg/inner/deep", tc.segment), y)
		h.exec("explorer.copyAbsolutePath")
		if got := clipboard.Get(); got != tc.want {
			t.Errorf("right-click on %q targeted %q, want %q", tc.segment, got, tc.want)
		}
		h.pressKey(tcell.KeyEscape, tcell.ModNone)
	}
}
