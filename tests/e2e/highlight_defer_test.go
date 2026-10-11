package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/highlight"

	"github.com/gdamore/tcell/v3"
)

func TestJumpToEndOfALargeFileDefersHighlighting(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("package main\n\n/*\n")
	for i := 0; i < 3000; i++ {
		fmt.Fprintf(&sb, "x%d := %d\n", i, i)
	}
	sb.WriteString("*/\n")
	content := sb.String()
	open := func(h *testHarness) {
		path := filepath.Join(h.dir, "big.go")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		h.app.EditorGroup.OpenFile(path)
		h.redraw()
		gotoLine(h, 1<<30)
	}
	style := func(h *testHarness) tcell.Style {
		row := findRow(h, "x2999 := 2999")
		if row < 0 {
			t.Fatalf("the last line is not on screen:\n%s", h.screenText())
		}
		s, ok := styleOfRune(h, row, 'x')
		if !ok {
			t.Fatal("could not sample the last line's style")
		}
		return s
	}

	reference := newTestHarness(t, 100, 30)
	defer reference.stop()
	open(reference)
	reference.settle()
	settled := style(reference)

	h := newTestHarness(t, 100, 30)
	defer h.stop()
	open(h)
	if highlight.Pending() == 0 {
		t.Fatal("jumping to the end of a large file tokenized every line above it in the frame")
	}
	if style(h) == settled {
		t.Fatal("fixture: the deferred line already shows the block comment's colors")
	}

	h.settle()
	if highlight.Pending() != 0 {
		t.Fatalf("pending after settling = %d", highlight.Pending())
	}
	if got := style(h); got != settled {
		t.Fatalf("settled style %v, want %v as when highlighted in full", got, settled)
	}
}

func TestDeepDiffLinesDeferAndSettleOnBothSides(t *testing.T) {
	old := []string{"package main", "", "/*"}
	for i := 0; i < 3000; i++ {
		old = append(old, fmt.Sprintf("x%d := %d", i, i))
	}
	old = append(old, "*/")
	cur := append([]string(nil), old...)
	cur[5] = "x2 := 22"
	open := func(h *testHarness) {
		setDiffView(h, "split", "full", false)
		h.app.EditorGroup.OpenDiff("big.go", diff.Parse(diff.Generate(old, cur, "big.go")), old, cur, true)
		h.redraw()
		h.pressKey(tcell.KeyEnd, tcell.ModCtrl)
	}
	styles := func(h *testHarness) (left, right tcell.Style) {
		row := findRow(h, "x2999 := 2999")
		if row < 0 {
			t.Fatalf("the last line is not on screen:\n%s", h.screenText())
		}
		cells, w, _ := h.screen.GetContents()
		var found []tcell.Style
		for x := 0; x+5 <= w; x++ {
			match := true
			for i, r := range "x2999" {
				if runeAt(cells, row*w+x+i) != r {
					match = false
					break
				}
			}
			if match {
				found = append(found, cells[row*w+x].Style)
			}
		}
		if len(found) != 2 {
			t.Fatalf("want the last line on both sides, found it %d times:\n%s", len(found), h.screenRow(row))
		}
		return found[0], found[1]
	}

	reference := newTestHarness(t, 140, 30)
	defer reference.stop()
	open(reference)
	reference.settle()
	wantLeft, wantRight := styles(reference)

	h := newTestHarness(t, 140, 30)
	defer h.stop()
	open(h)
	if highlight.Pending() == 0 {
		t.Fatal("jumping to the end of a large diff tokenized both sides in the frame")
	}
	if l, r := styles(h); l == wantLeft && r == wantRight {
		t.Fatal("fixture: the deferred lines already show the block comment's colors")
	}

	h.settle()
	if l, r := styles(h); l != wantLeft || r != wantRight {
		t.Fatalf("settled styles (%v, %v), want (%v, %v)", l, r, wantLeft, wantRight)
	}
}
