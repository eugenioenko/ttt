package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestBookmarkRejectsOutOfRangeLines(t *testing.T) {
	e := newTestEditor()
	events := 0
	e.OnBookmarkChange = func(int, string, Bookmark) { events++ }

	e.SetBookmark(-1, '*', term.StyleDefault)
	e.SetBookmark(3, '*', term.StyleDefault)
	e.RemoveBookmark(-1)
	e.RemoveBookmark(1)

	if len(e.Bookmarks) != 0 {
		t.Fatalf("bookmarks = %v, want none", e.Bookmarks)
	}
	if events != 0 {
		t.Fatalf("change events = %d, want 0", events)
	}
	if _, ok := e.GetBookmark(-1); ok {
		t.Fatal("GetBookmark(-1) reported a bookmark")
	}

	e.SetBookmark(2, '*', term.StyleDefault)
	e.RemoveBookmark(2)
	if events != 2 || len(e.Bookmarks) != 0 {
		t.Fatalf("valid set/remove: events = %d, bookmarks = %v", events, e.Bookmarks)
	}
}

func TestSetAllBookmarksDropsOutOfRangeLines(t *testing.T) {
	e := newTestEditor()
	e.SetAllBookmarks(map[int]Bookmark{-1: {Icon: 'a'}, 0: {Icon: 'b'}, 2: {Icon: 'c'}, 3: {Icon: 'd'}})

	if len(e.Bookmarks) != 2 {
		t.Fatalf("bookmarks = %v, want lines 0 and 2", e.Bookmarks)
	}
	if _, ok := e.GetBookmark(0); !ok {
		t.Fatal("line 0 bookmark missing")
	}
	if _, ok := e.GetBookmark(2); !ok {
		t.Fatal("line 2 bookmark missing")
	}
}

func gutterClick(e *EditorPaneWidget, x, y int) EventResult {
	e.HandleEvent(tcell.NewEventMouse(x, y, tcell.Button1, tcell.ModNone))
	return e.HandleEvent(tcell.NewEventMouse(x, y, tcell.ButtonNone, tcell.ModNone))
}

func TestGutterClickOnBookmarkColumn(t *testing.T) {
	for _, style := range []string{"compact", "extended"} {
		e := newTestEditor()
		e.LineNumbers = true
		e.GutterStyle = style
		e.SetRect(Rect{X: 0, Y: 0, W: 20, H: 10})
		var clicked []int
		e.OnGutterClick = func(line int) bool {
			clicked = append(clicked, line)
			return true
		}

		if got := gutterClick(e, e.bookmarkColumn(), 1); got != EventConsumed {
			t.Fatalf("%s: bookmark column click = %v, want consumed", style, got)
		}
		gutterClick(e, e.bookmarkColumn()+1, 2)
		gutterClick(e, e.bookmarkColumn(), 5)

		if len(clicked) != 1 || clicked[0] != 1 {
			t.Fatalf("%s: clicked lines = %v, want [1]", style, clicked)
		}
	}
}

func TestGutterClickFallsThroughWhenUnhandled(t *testing.T) {
	e := newTestEditor()
	e.LineNumbers = true
	e.SetRect(Rect{X: 0, Y: 0, W: 20, H: 10})
	e.OnGutterClick = func(int) bool { return false }

	gutterClick(e, 0, 2)

	if e.Cursor.Line != 2 {
		t.Fatalf("cursor line = %d, want 2", e.Cursor.Line)
	}
	if e.Selection != nil && e.Selection.Active {
		t.Fatal("unhandled gutter click left a selection")
	}
}
