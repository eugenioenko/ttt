package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

// Each fullwidth rune must land two columns apart, leaving the column it covers
// untouched. Writing into that column is what made the terminal swallow the
// following character (issue #434).
func TestDrawTextFullwidthAdvancesTwoColumns(t *testing.T) {
	grid := makeGrid(20, 3)
	s := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 20, H: 3})

	endX := s.DrawText(0, 0, "가a나", 0, term.StyleDefault)
	if endX != 5 {
		t.Fatalf("endX = %d, want 5", endX)
	}
	want := map[int]rune{0: '가', 2: 'a', 3: '나'}
	for x, ch := range want {
		if grid[0][x].Ch != ch {
			t.Errorf("grid[0][%d] = %q, want %q", x, grid[0][x].Ch, ch)
		}
	}
}

// A fullwidth rune is painted across two columns by the terminal no matter what
// the clip says, so one that would straddle the limit must not be drawn at all.
func TestDrawTextFullwidthStraddlingLimit(t *testing.T) {
	grid := makeGrid(20, 3)
	s := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 20, H: 3})

	s.DrawText(0, 0, "a가", 2, term.StyleDefault)
	if grid[0][0].Ch != 'a' {
		t.Fatalf("grid[0][0] = %q, want 'a'", grid[0][0].Ch)
	}
	if grid[0][1].Ch == '가' {
		t.Error("fullwidth rune drawn at the clip edge would bleed past it")
	}
}

func TestDrawTextFullwidthStopsAtSurfaceEdge(t *testing.T) {
	grid := makeGrid(20, 3)
	s := NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 3, H: 3})

	s.DrawText(0, 0, "a가", 0, term.StyleDefault)
	if grid[0][2].Ch == '가' {
		t.Error("fullwidth rune drawn in the last column would bleed outside the surface")
	}
	if grid[0][3].Ch != '.' {
		t.Error("drawing escaped the surface")
	}
}

// 가나다 occupies 6 terminal columns: 가=[0,1] 나=[2,3] 다=[4,5].
func TestBufColToVisualColFullwidth(t *testing.T) {
	cases := []struct {
		name   string
		line   string
		bufCol int
		want   int
	}{
		{"start of line", "가나다", 0, 0},
		{"after one fullwidth rune", "가나다", 1, 2},
		{"after two", "가나다", 2, 4},
		{"end of line", "가나다", 3, 6},
		{"ascii is unaffected", "abc", 2, 2},
		{"mixed ascii and fullwidth", "가a나b다", 3, 5},
		{"cyrillic stays narrow", "привет", 6, 6},
		{"tab then fullwidth", "\t가", 2, 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := bufColToVisualCol(tc.line, tc.bufCol, 4); got != tc.want {
				t.Errorf("bufColToVisualCol(%q, %d) = %d, want %d", tc.line, tc.bufCol, got, tc.want)
			}
		})
	}
}

func TestVisualColToBufColFullwidth(t *testing.T) {
	cases := []struct {
		name    string
		line    string
		visCol  int
		want    int
		comment string
	}{
		{name: "column 0", line: "가나다", visCol: 0, want: 0},
		{name: "second column of first rune", line: "가나다", visCol: 1, want: 0,
			comment: "clicking either half of a fullwidth rune selects that rune"},
		{name: "first column of second rune", line: "가나다", visCol: 2, want: 1},
		{name: "second column of second rune", line: "가나다", visCol: 3, want: 1},
		{name: "first column of third rune", line: "가나다", visCol: 4, want: 2},
		{name: "past the end clamps", line: "가나다", visCol: 99, want: 3},
		{name: "ascii is unaffected", line: "abc", visCol: 2, want: 2},
		{name: "ascii after fullwidth", line: "가a나", visCol: 2, want: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := visualColToBufCol(tc.line, tc.visCol, 4); got != tc.want {
				t.Errorf("visualColToBufCol(%q, %d) = %d, want %d", tc.line, tc.visCol, got, tc.want)
			}
		})
	}
}

// Round-tripping a rune index through its visual column must return the same
// index — otherwise the cursor drifts every time it is converted for drawing.
func TestVisualColRoundTrip(t *testing.T) {
	for _, line := range []string{"가나다라마바사", "가a나b다", "hello가", "\t가나", "привет"} {
		for i := range []rune(line) {
			vis := bufColToVisualCol(line, i, 4)
			if got := visualColToBufCol(line, vis, 4); got != i {
				t.Errorf("%q: rune %d -> col %d -> rune %d", line, i, vis, got)
			}
		}
	}
}

func TestWrapLineSegmentsFullwidth(t *testing.T) {
	// Width 6 fits exactly three fullwidth runes per visual row.
	segments := wrapLineSegments([]rune("가나다라마바"), 6, 4)
	want := []int{0, 3}
	if len(segments) != len(want) {
		t.Fatalf("segments = %v, want %v", segments, want)
	}
	for i := range want {
		if segments[i] != want[i] {
			t.Fatalf("segments = %v, want %v", segments, want)
		}
	}
}

// A fullwidth rune must never be split across two visual rows: with an odd
// width the last column of the row stays empty instead.
func TestWrapLineSegmentsNeverSplitsFullwidthRune(t *testing.T) {
	segments := wrapLineSegments([]rune("가나다라"), 5, 4)
	if len(segments) != 2 || segments[1] != 2 {
		t.Fatalf("segments = %v, want [0 2] (two runes fit in 5 columns)", segments)
	}
}

func TestBufferPosToWrapScreenPosFullwidth(t *testing.T) {
	lines := []string{"가나다라마바"}
	_, screenCol := bufferPosToWrapScreenPos(lines, 0, 2, 6, 4)
	if screenCol != 4 {
		t.Errorf("screenCol = %d, want 4", screenCol)
	}
	// The fourth rune starts the second visual row.
	row, screenCol := bufferPosToWrapScreenPos(lines, 0, 3, 6, 4)
	if row != 1 || screenCol != 0 {
		t.Errorf("row, col = %d, %d; want 1, 0", row, screenCol)
	}
}
