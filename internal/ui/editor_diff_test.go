package ui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"
)

func deletedTexts(o *DiffOverlay) map[int][]string {
	out := map[int][]string{}
	for anchor, block := range o.Deleted {
		for _, l := range block {
			out[anchor] = append(out[anchor], l.Text)
		}
	}
	return out
}

func TestNewDiffOverlay(t *testing.T) {
	C, A := diff.Context, diff.Added
	tests := []struct {
		name      string
		old, new  []string
		wantKinds []diff.LineKind
		wantDel   map[int][]string
	}{
		{"modify and append", []string{"a", "b", "c", "d"}, []string{"a", "B", "c", "e", "f"},
			[]diff.LineKind{C, A, C, A, A}, map[int][]string{1: {"b"}, 3: {"d"}}},
		{"delete at end", []string{"a", "b"}, []string{"a"},
			[]diff.LineKind{C}, map[int][]string{1: {"b"}}},
		{"delete in middle", []string{"a", "b", "c"}, []string{"a", "c"},
			[]diff.LineKind{C, C}, map[int][]string{1: {"b"}}},
		{"block replace", []string{"a", "x", "y", "z"}, []string{"a", "P", "Q"},
			[]diff.LineKind{C, A, A}, map[int][]string{1: {"x", "y", "z"}}},
		{"no changes", []string{"a"}, []string{"a"},
			[]diff.LineKind{C}, map[int][]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := NewDiffOverlay(diff.FullDiffLines(tt.old, tt.new))
			if !reflect.DeepEqual(o.Kinds, tt.wantKinds) {
				t.Errorf("kinds = %v, want %v", o.Kinds, tt.wantKinds)
			}
			if got := deletedTexts(o); !reflect.DeepEqual(got, tt.wantDel) {
				t.Errorf("deleted = %v, want %v", got, tt.wantDel)
			}
		})
	}
}

func TestEditorRendersDiffOverlay(t *testing.T) {
	oldLines := []string{"a", "b", "c", "d"}
	newLines := []string{"a", "B", "c", "e", "f"}
	e := newRowsEditor(newLines, 20, 8)
	e.LineNumbers = true
	e.SetDiffOverlay(NewDiffOverlay(diff.FullDiffLines(oldLines, newLines)))

	grid := makeGrid(20, 8)
	e.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 20, H: 8}))
	g := e.GutterWidth()

	text := func(y int) string {
		var sb strings.Builder
		for x := g; x < g+e.Viewport.Width; x++ {
			sb.WriteRune(grid[y][x].Ch)
		}
		return strings.TrimRight(sb.String(), " ")
	}
	want := []string{"a", "b", "B", "c", "d", "e", "f", ""}
	for y, w := range want {
		if got := text(y); got != w {
			t.Fatalf("row %d = %q, want %q", y, got, w)
		}
	}
	if grid[1][g].BgStyle != term.StyleDiffDeleted || grid[1][0].Ch != '−' {
		t.Errorf("phantom row not styled as deleted: %+v %+v", grid[1][g], grid[1][0])
	}
	if !strings.Contains(string([]rune{grid[1][1].Ch, grid[1][2].Ch, grid[1][3].Ch}), "2") {
		t.Errorf("phantom row gutter should show old line number 2")
	}
	if grid[2][g].BgStyle != term.StyleDiffAdded || grid[2][g+5].BgStyle != term.StyleDiffAdded {
		t.Errorf("added row not tinted: %+v", grid[2][g])
	}
	if grid[3][g].BgStyle == term.StyleDiffAdded {
		t.Errorf("context row tinted")
	}

	e.Cursor.Line = 1
	clickAt(e, g, 4)
	if e.Cursor.Line != 3 {
		t.Fatalf("click on deleted row = line %d, want 3", e.Cursor.Line)
	}
}

func TestNewSplitDiffOverlaysAlignRows(t *testing.T) {
	old := []string{"a", "b", "c", "d", "e"}
	cur := []string{"a", "B", "x", "y", "c", "e", "f"}
	left, right := NewSplitDiffOverlays(diff.FullDiffLines(old, cur))
	if len(left.Kinds) != len(old) || len(right.Kinds) != len(cur) {
		t.Fatalf("kinds: left %d right %d", len(left.Kinds), len(right.Kinds))
	}
	rows := func(o *DiffOverlay) int {
		n := len(o.Kinds)
		for _, f := range o.Fillers {
			n += f
		}
		return n
	}
	if rows(left) != rows(right) {
		t.Fatalf("row totals differ: left %d right %d", rows(left), rows(right))
	}

	leftPane := NewEditorPaneWidget(&buffer.Buffer{Lines: old}, &cursor.Cursor{}, &view.Viewport{Width: 20})
	rightPane := NewEditorPaneWidget(&buffer.Buffer{Lines: cur}, &cursor.Cursor{}, &view.Viewport{Width: 20})
	leftPane.SetDiffOverlay(left)
	rightPane.SetDiffOverlay(right)
	ll, rl := leftPane.layout(), rightPane.layout()
	for _, pair := range [][2]int{{0, 0}, {2, 4}, {4, 5}} {
		if l, r := ll.startRow(pair[0])+leftPane.phantoms[pair[0]], rl.startRow(pair[1])+rightPane.phantoms[pair[1]]; l != r {
			t.Fatalf("old line %d at row %d, new line %d at row %d", pair[0], l, pair[1], r)
		}
	}
}
