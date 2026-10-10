package ui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/term"
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

func fullLiveDiff(old, cur []string) *DiffEditorWidget {
	d := NewEditableDiffWidget("f.txt", old, diff.FullDiffLines(old, cur))
	d.SetMode(DiffModeUnified)
	d.SetContextMode(DiffContextFullFile)
	return d
}

func TestEditableDiffOverlay(t *testing.T) {
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
			o := fullLiveDiff(tt.old, tt.new).liveUnified
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
	e.SetDiffOverlay(fullLiveDiff(oldLines, newLines).liveUnified)

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

func TestEditableSplitAlignsRows(t *testing.T) {
	old := []string{"a", "b", "c", "d", "e"}
	cur := []string{"a", "B", "x", "y", "c", "e", "f"}
	d := fullLiveDiff(old, cur)
	d.SetMode(DiffModeSplit)
	e := newRowsEditor(cur, 30, 10)
	e.LineNumbers = true
	d.bind(e)
	d.SetRect(Rect{X: 0, Y: 0, W: 61, H: 10})
	grid := makeGrid(61, 10)
	d.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 61, H: 10}))

	row := func(y int) string {
		var sb strings.Builder
		for x := 0; x < 61; x++ {
			sb.WriteRune(grid[y][x].Ch)
		}
		return sb.String()
	}
	for _, text := range []string{"a", "c", "e"} {
		found := false
		for y := 0; y < 10; y++ {
			r := row(y)
			left, right := r[:strings.IndexRune(r, '│')], r[strings.IndexRune(r, '│'):]
			if strings.Contains(left, " "+text+" ") && strings.Contains(right, " "+text+" ") {
				found = true
			}
		}
		if !found {
			t.Fatalf("context line %q not on one row in both panes:\n%s", text, strings.Join(func() []string {
				var out []string
				for y := 0; y < 10; y++ {
					out = append(out, row(y))
				}
				return out
			}(), "\n"))
		}
	}
}
