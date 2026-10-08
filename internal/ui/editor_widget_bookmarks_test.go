package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
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
