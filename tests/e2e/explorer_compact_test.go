package e2e

import (
	"fmt"
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

func TestExplorerContextOpenTargetsFolderSegment(t *testing.T) {
	for _, compact := range []bool{true, false} {
		for _, segment := range []string{"pkg", "inner", "deep"} {
			for _, expanded := range []bool{true, false} {
				t.Run(fmt.Sprintf("compact=%v/%s/expanded=%v", compact, segment, expanded), func(t *testing.T) {
					h := newTestHarness(t, 80, 24)
					defer h.stop()
					path := filepath.Join(h.dir, "pkg", "inner", "deep", "file.go")
					if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte("package deep"), 0o644); err != nil {
						t.Fatal(err)
					}
					h.app.Explorer.Settings.CompactFolders = compact
					h.exec("sidebar.explorer")
					h.exec("explorer.expandAll")
					target := filepath.Join(h.dir, "pkg")
					if segment != "pkg" {
						target = filepath.Join(target, "inner")
					}
					if segment == "deep" {
						target = filepath.Dir(path)
					}
					label := segment
					if compact {
						label = "pkg/inner/deep"
						h.app.Explorer.Tree.SelectByID(filepath.Dir(path))
					} else {
						h.app.Explorer.Tree.SelectByID(target)
					}
					if !expanded {
						h.exec("explorer.open")
					}
					y := h.rowContaining(label)
					x := displayColumnOf(h.screenRow(y), label)
					if compact {
						x += strings.Index(label, segment)
					}
					h.rightClick(x, y)
					if context := h.app.ExplorerContextNode; context == nil || context.ID != target {
						t.Fatalf("context = %+v, want %s", context, target)
					}
					h.pressKey(tcell.KeyEnter, tcell.ModNone)
					if h.app.Root.HasOverlay() {
						t.Fatal("Open did not close the context menu")
					}
					if selected := h.app.Explorer.Tree.Selected(); selected == nil || selected.ID != target || selected.Expanded == expanded {
						t.Fatalf("Open selected %+v, want %s expanded=%v", selected, target, !expanded)
					}
					if h.app.ExplorerContextNode != nil {
						t.Fatal("Open retained the context target")
					}
				})
			}
		}
	}
}
