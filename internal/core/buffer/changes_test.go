package buffer

import (
	"slices"
	"testing"
)

func changesSince(t *testing.T, b *Buffer, v uint64) []Change {
	t.Helper()
	var got []Change
	if !b.ChangesSince(v, func(c Change) { got = append(got, c) }) {
		t.Fatalf("changes since %d not available at version %d", v, b.Version())
	}
	return got
}

func TestBufferRecordsEveryLineChange(t *testing.T) {
	b := &Buffer{Lines: []string{"a", "b", "c"}}
	v := b.Version()

	b.SetLine(1, "B")
	b.InsertLine(3, "d")
	b.DeleteLine(0)
	b.Splice(1, 1, "x", "y", "z")
	b.InsertRune(0, 0, '!')
	b.DeleteRune(0, 0)

	want := []Change{
		{Start: 1, Removed: 1, Added: 1},
		{Start: 3, Removed: 0, Added: 1},
		{Start: 0, Removed: 1, Added: 0},
		{Start: 1, Removed: 1, Added: 3},
		{Start: 0, Removed: 1, Added: 1},
		{Start: 0, Removed: 1, Added: 1},
	}
	if got := changesSince(t, b, v); !slices.Equal(got, want) {
		t.Fatalf("changes = %v, want %v", got, want)
	}
	if want := []string{"B", "x", "y", "z", "d"}; !slices.Equal(b.Lines, want) {
		t.Fatalf("lines = %q, want %q", b.Lines, want)
	}
}

func TestSpliceShrinksGrowsAndReplacesInPlace(t *testing.T) {
	b := &Buffer{Lines: []string{"0", "1", "2", "3", "4"}}
	b.Splice(1, 3, "m")
	if want := []string{"0", "m", "4"}; !slices.Equal(b.Lines, want) {
		t.Fatalf("shrink: %q, want %q", b.Lines, want)
	}
	b.Splice(1, 1, "p", "q", "r", "s")
	if want := []string{"0", "p", "q", "r", "s", "4"}; !slices.Equal(b.Lines, want) {
		t.Fatalf("grow: %q, want %q", b.Lines, want)
	}
	b.Splice(2, 2, "Q", "R")
	if want := []string{"0", "p", "Q", "R", "s", "4"}; !slices.Equal(b.Lines, want) {
		t.Fatalf("replace: %q, want %q", b.Lines, want)
	}
	b.Splice(6, 0, "end")
	b.Splice(4, 99)
	if want := []string{"0", "p", "Q", "R"}; !slices.Equal(b.Lines, want) {
		t.Fatalf("append then clamp-delete: %q, want %q", b.Lines, want)
	}
}

func TestSetLinesAndLoadRecordAWholeBufferChange(t *testing.T) {
	b := &Buffer{Lines: []string{"a", "b"}}
	v := b.Version()
	b.SetLines([]string{"x"})
	if got := changesSince(t, b, v); !slices.Equal(got, []Change{{Start: 0, Removed: 2, Added: 1}}) {
		t.Fatalf("changes = %v", got)
	}
}

func TestChangesSinceAnOldVersionFails(t *testing.T) {
	b := &Buffer{Lines: []string{"a"}}
	v := b.Version()
	for i := 0; i < changeLogSize; i++ {
		b.SetLine(0, "b")
	}
	if !b.ChangesSince(v, func(Change) {}) {
		t.Fatal("a full log should still replay")
	}
	b.SetLine(0, "c")
	if b.ChangesSince(v, func(Change) {}) {
		t.Fatal("changes older than the log replayed")
	}
	if !b.ChangesSince(b.Version(), func(Change) { t.Fatal("no changes expected") }) {
		t.Fatal("the current version should replay nothing")
	}
}
