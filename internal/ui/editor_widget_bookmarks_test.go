package ui

import (
	"github.com/eugenioenko/ttt/internal/core/fold"
	"strconv"
	"strings"
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

func TestGutterClickSplitsAtDisplayedNumber(t *testing.T) {
	for _, style := range []string{"minimal", "compact", "extended"} {
		for _, number := range []int{1, 12, 123} {
			for _, handled := range []bool{true, false} {
				e := newEditorWithLines(make([]string, 150)...)
				e.LineNumbers = true
				e.GutterStyle = style
				rect := Rect{X: 8, Y: 3, W: 24, H: 5}
				e.SetRect(rect)
				line := number - 1
				e.Folds = fold.NewState()
				e.Folds.SetRanges([]fold.Range{{StartLine: line, EndLine: line + 1}})
				e.Cursor.Line = line
				e.Viewport.TopLine = line
				grid := makeGrid(rect.W, rect.H)
				e.Render(NewRenderSurface(grid, Rect{W: rect.W, H: rect.H}))
				digits := strconv.Itoa(number)
				first := strings.Index(gridRow(grid, 0, e.GutterWidth()), digits)
				if first < 0 {
					t.Fatalf("missing line %d", number)
				}
				split := first + (len(digits)+1)/2
				clicks := 0
				e.OnGutterClick = func(got int) bool {
					if got != line {
						t.Fatalf("clicked line %d, want %d", got, line)
					}
					clicks++
					return handled
				}
				gutterClick(e, rect.X+split-1, rect.Y)
				if clicks != 1 || e.Folds.IsCollapsed(line) {
					t.Fatalf("%s line %d: left half did not isolate bookmark click", style, number)
				}
				for _, collapsed := range []bool{true, false} {
					gutterClick(e, rect.X+split, rect.Y)
					if clicks != 1 || e.Folds.IsCollapsed(line) != collapsed {
						t.Fatalf("%s line %d: right half did not toggle fold", style, number)
					}
				}
			}
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

func TestBookmarkOverlaySharesColumnAndPreservesUnderlyingBookmark(t *testing.T) {
	for _, style := range []string{"compact", "extended", "minimal"} {
		e := newTestEditor()
		e.LineNumbers = true
		e.GutterStyle = style
		rect := Rect{W: 20, H: 5}
		e.SetRect(rect)
		width := e.GutterWidth()
		e.SetBookmark(1, 'B', term.StyleDefault)
		active := true
		e.BookmarkOverlay = func(line int) (Bookmark, bool) {
			return Bookmark{Icon: '*', Style: term.StyleActiveLine}, line == 1 && active
		}
		e.OnGutterClick = func(line int) bool {
			if line != 1 {
				t.Fatal(line)
			}
			active = false
			return true
		}
		grid := makeGrid(20, 5)
		surface := NewRenderSurface(grid, rect)
		e.Render(surface)
		x := bookmarkColumn
		if e.GutterWidth() != width || grid[1][x].Ch != '*' {
			t.Fatalf("%s: missing overlay or changed width", style)
		}
		if b, ok := e.GetBookmark(1); !ok || b.Icon != 'B' {
			t.Fatal("overlay replaced stored bookmark")
		}
		gutterClick(e, x, 1)
		e.Render(surface)
		if grid[1][x].Ch != 'B' {
			t.Fatalf("%s: underlying bookmark did not reappear", style)
		}
	}
}
