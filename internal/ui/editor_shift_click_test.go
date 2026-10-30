package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/gdamore/tcell/v3"
)

func TestEditorShiftClickAcrossScroll(t *testing.T) {
	for _, wrap := range []bool{false, true} {
		t.Run(map[bool]string{false: "unwrapped", true: "wrapped"}[wrap], func(t *testing.T) {
			group := NewEditorGroupWidget(nil, 4, false, "extended")
			e := group.Editor
			e.Buf.Lines = strings.Split(strings.Repeat("abcdef\n", 7999)+"last", "\n")
			e.WordWrap = wrap
			root := NewRoot(group)
			root.SetSize(80, 24)
			redraw := func() { root.Render(makeGrid(80, 24)) }
			redraw()
			redraw()
			r := e.GetRect()
			x, y := r.X+e.GutterWidth()+2, r.Y+2
			click := func(x, y int, mod tcell.ModMask) {
				root.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, mod))
				root.HandleEvent(tcell.NewEventMouse(x, y, tcell.ButtonNone, mod))
			}
			click(x, y, 0)
			anchor := selection.Position{Line: e.Cursor.Line, Col: e.Cursor.Col}
			for range 1000 {
				root.HandleEvent(tcell.NewEventMouse(x, y, tcell.WheelDown, 0))
			}
			redraw()
			if e.Viewport.TopLine < 2000 || e.Cursor.Line != anchor.Line {
				t.Fatalf("scroll did not leave cursor offscreen: top=%d cursor=%+v", e.Viewport.TopLine, e.Cursor)
			}
			line, col := e.mouseToPos(e.GetRect(), x+2, y+3)
			click(x+2, y+3, tcell.ModShift)
			if !e.Selection.Active || e.Selection.Anchor != anchor || e.Cursor.Line != line || e.Cursor.Col != col {
				t.Fatalf("Shift+click: selection=%+v cursor=%+v, want anchor=%+v end=%d:%d", e.Selection, e.Cursor, anchor, line, col)
			}
			// A repeated Shift+click must extend, even at double-click speed.
			click(x+2, y+3, tcell.ModShift)
			if e.Selection.Anchor != anchor || e.Cursor.Line != line || e.Cursor.Col != col {
				t.Fatal("repeated Shift+click changed the selection anchor or endpoint")
			}
			root.HandleEvent(tcell.NewEventMouse(x+3, y+4, tcell.Button1, tcell.ModShift))
			root.HandleEvent(tcell.NewEventMouse(x+4, y+5, tcell.Button1, tcell.ModShift))
			root.HandleEvent(tcell.NewEventMouse(x+4, y+5, tcell.ButtonNone, tcell.ModShift))
			if e.Selection.Anchor != anchor {
				t.Fatal("Shift+drag changed the anchor")
			}
			for range 1000 {
				root.HandleEvent(tcell.NewEventMouse(x, y, tcell.WheelUp, 0))
			}
			redraw()
			click(x-1, r.Y+1, tcell.ModShift)
			if !e.Selection.Active || e.Selection.Anchor != anchor || e.Cursor.Line >= anchor.Line {
				t.Fatal("reverse Shift+click did not extend through the original anchor")
			}
			click(x+1, y+2, 0)
			if e.Selection.Active {
				t.Fatal("ordinary click did not clear selection")
			}
			// With no active selection, use the current cursor, not a stale anchor.
			e.Cursor.Line, e.Cursor.Col = 10, 3
			click(x, y, tcell.ModShift)
			if e.Selection.Anchor != (selection.Position{Line: 10, Col: 3}) {
				t.Fatalf("Shift+click used stale anchor: %+v", e.Selection.Anchor)
			}
		})
	}
}
