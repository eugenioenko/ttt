package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
)

func commentedDiffPair() ([]string, []string) {
	var old []string
	old = append(old, "package main", "", "func main() {}", "/*")
	for i := 0; i < 900; i++ {
		old = append(old, fmt.Sprintf("x%d := %d", i, i))
	}
	old = append(old, "*/")
	updated := append([]string(nil), old...)
	for i := 300; i < 900; i += 40 {
		updated[i] = fmt.Sprintf("y%d := %q", i, "changed")
	}
	return old, updated
}

func diffFrame(h *testHarness) [][]term.Cell {
	cells := h.renderer.NextFrame(h.app.Root.Width, h.app.Root.Height)
	h.app.Root.Render(cells)
	out := make([][]term.Cell, len(cells))
	for i := range cells {
		out[i] = append([]term.Cell(nil), cells[i]...)
	}
	h.renderer.Render(h.app.Screen)
	return out
}

func TestProgressiveDiffHighlightSettlesToExact(t *testing.T) {
	old, updated := commentedDiffPair()
	fd := diff.Parse(diff.Generate(old, updated, "main.go"))
	for _, mode := range []string{config.DiffModeSplit, config.DiffModeUnified} {
		t.Run(mode, func(t *testing.T) {
			open := func(async bool, notified chan struct{}) *testHarness {
				h := newTestHarness(t, 160, 40)
				h.app.EditorGroup.RequestRedraw = nil
				if async {
					h.app.EditorGroup.RequestRedraw = func() {
						select {
						case notified <- struct{}{}:
						default:
						}
					}
				}
				setDiffView(h, mode, config.DiffContextChanges, false)
				h.app.EditorGroup.OpenDiff("main.go", fd, old, updated, false)
				return h
			}
			syncH := open(false, nil)
			defer syncH.stop()
			want := diffFrame(syncH)

			notified := make(chan struct{}, 1)
			h := open(true, notified)
			defer h.stop()
			first := diffFrame(h)
			if equalFrames(editorCells(h, first), editorCells(syncH, want)) {
				t.Fatal("first frame already matched; the deep comment lines were not deferred")
			}
			deadline := time.After(10 * time.Second)
			for {
				select {
				case <-notified:
				case <-deadline:
					t.Fatal("background highlighting never settled")
				}
				if got := diffFrame(h); equalFrames(editorCells(h, got), editorCells(syncH, want)) {
					return
				}
			}
		})
	}
}

func editorCells(h *testHarness, frame [][]term.Cell) [][]term.Cell {
	r := h.app.EditorGroup.GetRect()
	var out [][]term.Cell
	for y := r.Y; y < r.Y+r.H && y < len(frame); y++ {
		out = append(out, frame[y][r.X:min(r.X+r.W, len(frame[y]))])
	}
	return out
}

func equalFrames(a, b [][]term.Cell) bool {
	if len(a) != len(b) {
		return false
	}
	for y := range a {
		if len(a[y]) != len(b[y]) {
			return false
		}
		for x := range a[y] {
			if a[y][x] != b[y][x] {
				return false
			}
		}
	}
	return true
}
