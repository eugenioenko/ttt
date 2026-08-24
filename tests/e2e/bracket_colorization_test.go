package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/app"
	"github.com/eugenioenko/ttt/internal/term"
)

func TestBracketPairColorization(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	// Write a file with nested brackets
	fp := filepath.Join(h.dir, "brackets.go")
	content := `package main

func main() {
	x := foo(bar(baz()))
	y := []int{1, 2, 3}
}
`
	os.WriteFile(fp, []byte(content), 0644)
	h.app.Settings.Editor.BracketPairColorization = true
	h.app.EditorGroup.BracketPairColorization = true
	h.app.EditorGroup.Editor.BracketPairColorization = true
	h.app.EditorGroup.Editor.BracketColorStyles = app.ResolveBracketColorStyles(nil)
	h.app.EditorGroup.OpenFile(fp)
	h.redraw()

	h.assertContains("func main()")

	cells, w, _ := h.screen.GetContents()

	fooRow := -1
	fooCol := -1
	for y := 0; y < 24; y++ {
		for x := 0; x < w-3; x++ {
			idx := y*w + x
			if runeAt(cells, idx) == 'f' &&
				runeAt(cells, idx+1) == 'o' &&
				runeAt(cells, idx+2) == 'o' &&
				runeAt(cells, idx+3) == '(' {
				fooRow = y
				fooCol = x + 3
				break
			}
		}
		if fooRow >= 0 {
			break
		}
	}

	if fooRow < 0 {
		t.Fatal("could not find 'foo(' on screen")
	}

	outerOpenCell := cells[fooRow*w+fooCol]
	middleOpenCol := fooCol + 4
	middleOpenCell := cells[fooRow*w+middleOpenCol]
	innerOpenCol := middleOpenCol + 4
	innerOpenCell := cells[fooRow*w+innerOpenCol]

	outerFg := outerOpenCell.Style.GetForeground()
	middleFg := middleOpenCell.Style.GetForeground()
	innerFg := innerOpenCell.Style.GetForeground()

	// All three should have different foreground colors (different nesting depths)
	if outerFg == middleFg {
		t.Errorf("outer and middle bracket should have different colors, both have %v", outerFg)
	}
	if middleFg == innerFg {
		t.Errorf("middle and inner bracket should have different colors, both have %v", middleFg)
	}
	if outerFg == innerFg {
		t.Errorf("outer and inner bracket should have different colors, both have %v", outerFg)
	}

	innerCloseCol := innerOpenCol + 1
	innerCloseCell := cells[fooRow*w+innerCloseCol]
	middleCloseCol := innerCloseCol + 1
	middleCloseCell := cells[fooRow*w+middleCloseCol]
	outerCloseCol := middleCloseCol + 1
	outerCloseCell := cells[fooRow*w+outerCloseCol]

	innerCloseFg := innerCloseCell.Style.GetForeground()
	middleCloseFg := middleCloseCell.Style.GetForeground()
	outerCloseFg := outerCloseCell.Style.GetForeground()

	if innerFg != innerCloseFg {
		t.Errorf("inner open (%v) and close (%v) bracket colors should match", innerFg, innerCloseFg)
	}
	if middleFg != middleCloseFg {
		t.Errorf("middle open (%v) and close (%v) bracket colors should match", middleFg, middleCloseFg)
	}
	if outerFg != outerCloseFg {
		t.Errorf("outer open (%v) and close (%v) bracket colors should match", outerFg, outerCloseFg)
	}
}

func TestBracketPairColorizationToggle(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	fp := filepath.Join(h.dir, "toggle.go")
	content := `package main

func main() {
	x := foo(1)
}
`
	os.WriteFile(fp, []byte(content), 0644)
	h.app.Settings.Editor.BracketPairColorization = true
	h.app.EditorGroup.BracketPairColorization = true
	h.app.EditorGroup.Editor.BracketPairColorization = true
	h.app.EditorGroup.Editor.BracketColorStyles = app.ResolveBracketColorStyles(nil)
	h.app.EditorGroup.OpenFile(fp)
	h.redraw()

	cells, w, _ := h.screen.GetContents()
	parenRow, parenCol := -1, -1
	for y := 0; y < 24; y++ {
		for x := 0; x < w-3; x++ {
			idx := y*w + x
			if runeAt(cells, idx) == 'f' &&
				runeAt(cells, idx+1) == 'o' &&
				runeAt(cells, idx+2) == 'o' &&
				runeAt(cells, idx+3) == '(' {
				parenRow = y
				parenCol = x + 3
				break
			}
		}
		if parenRow >= 0 {
			break
		}
	}

	if parenRow < 0 {
		t.Fatal("could not find 'foo(' on screen")
	}

	colorizedFg := cells[parenRow*w+parenCol].Style.GetForeground()

	h.exec("options.toggleBracketColors")
	_, _, _ = h.screen.GetContents()

	if h.app.Settings.Editor.BracketPairColorization {
		t.Error("expected BracketPairColorization to be false after toggle")
	}

	h.exec("options.toggleBracketColors")
	if !h.app.Settings.Editor.BracketPairColorization {
		t.Error("expected BracketPairColorization to be true after second toggle")
	}

	cells, _, _ = h.screen.GetContents()
	reenabled := cells[parenRow*w+parenCol].Style.GetForeground()

	if colorizedFg != reenabled {
		t.Errorf("re-enabled bracket color %v should match original %v", reenabled, colorizedFg)
	}
}

func TestBracketColorizationSkipsStrings(t *testing.T) {
	h := newTestHarness(t, 80, 24)
	defer h.stop()

	fp := filepath.Join(h.dir, "stringbrackets.go")
	content := `package main

var s = "hello(world)"
var x = (1 + 2)
`
	os.WriteFile(fp, []byte(content), 0644)
	h.app.Settings.Editor.BracketPairColorization = true
	h.app.EditorGroup.BracketPairColorization = true
	h.app.EditorGroup.Editor.BracketPairColorization = true
	h.app.EditorGroup.Editor.BracketColorStyles = app.ResolveBracketColorStyles(nil)
	h.app.EditorGroup.OpenFile(fp)
	h.redraw()

	cells, w, _ := h.screen.GetContents()
	stringParenRow, stringParenCol := -1, -1
	freeParenRow, freeParenCol := -1, -1
	for y := 0; y < 24; y++ {
		for x := 0; x < w-5; x++ {
			idx := y*w + x
			r0 := runeAt(cells, idx)
			r1 := runeAt(cells, idx+1)
			r2 := runeAt(cells, idx+2)
			r3 := runeAt(cells, idx+3)
			r4 := runeAt(cells, idx+4)
			r5 := runeAt(cells, idx+5)
			if r0 == 'h' && r1 == 'e' && r2 == 'l' && r3 == 'l' && r4 == 'o' && r5 == '(' {
				stringParenRow = y
				stringParenCol = x + 5
			}
		}
		for x := 0; x < w-1; x++ {
			idx := y*w + x
			r0 := runeAt(cells, idx)
			r1 := runeAt(cells, idx+1)
			if r0 == '(' && r1 == '1' {
				freeParenRow = y
				freeParenCol = x
			}
		}
	}

	if stringParenRow < 0 {
		t.Fatal("could not find 'hello(' on screen")
	}
	if freeParenRow < 0 {
		t.Fatal("could not find '(1' on screen")
	}

	stringBracketFg := cells[stringParenRow*w+stringParenCol].Style.GetForeground()
	freeBracketFg := cells[freeParenRow*w+freeParenCol].Style.GetForeground()

	if stringBracketFg == freeBracketFg {
		t.Errorf("bracket in string (%v) should have different color than free bracket (%v)", stringBracketFg, freeBracketFg)
	}
}

func runeAt(cells []term.SimCell, idx int) rune {
	if idx < len(cells) && cells[idx].Str != "" {
		return []rune(cells[idx].Str)[0]
	}
	return 0
}
