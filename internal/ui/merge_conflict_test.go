package ui

import (
	"reflect"
	"testing"
)

func TestResolveConflict(t *testing.T) {
	tests := []struct {
		name   string
		choice int
		want   []string
	}{
		{"current", 0, []string{"before", "ours", "after"}},
		{"incoming", 1, []string{"before", "theirs", "after"}},
		{"both", 2, []string{"before", "ours", "theirs", "after"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := []string{"before", "<<<<<<< HEAD", "ours", "=======", "theirs", ">>>>>>> branch", "after"}
			e := newRowsEditor(lines, 40, 8)
			if !e.resolveConflict(1, tt.choice) {
				t.Fatal("resolveConflict() = false")
			}
			if !reflect.DeepEqual(e.Buf.Lines, tt.want) {
				t.Fatalf("lines = %q, want %q", e.Buf.Lines, tt.want)
			}
		})
	}
}

func TestConflictActionAt(t *testing.T) {
	for _, tt := range []struct{ column, want int }{{3, 0}, {14, 1}, {23, 2}, {9, -1}, {30, -1}} {
		if got := conflictActionAt(tt.column); got != tt.want {
			t.Errorf("conflictActionAt(%d) = %d, want %d", tt.column, got, tt.want)
		}
	}
}

func TestClickConflictAction(t *testing.T) {
	e := newRowsEditor([]string{"<<<<<<< HEAD", "ours", "=======", "theirs", ">>>>>>> branch"}, 40, 8)
	renderRows(e)
	clickAt(e, e.GutterWidth()+12, 0)
	if !reflect.DeepEqual(e.Buf.Lines, []string{"theirs"}) {
		t.Fatalf("lines after clicking Incoming = %q", e.Buf.Lines)
	}
}
