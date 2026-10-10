package ui

import (
	"strings"
	"testing"
)

func TestWrapLineSegments(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		width    int
		tabW     int
		expected []int
	}{
		{"empty line", "", 10, 4, []int{0}},
		{"short line fits", "hello", 10, 4, []int{0}},
		{"exact fit", "12345", 5, 4, []int{0}},
		{"one wrap", "1234567890", 5, 4, []int{0, 5}},
		{"two wraps", "123456789012345", 5, 4, []int{0, 5, 10}},
		{"single char width", "abc", 1, 4, []int{0, 1, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runes := []rune(tt.line)
			got := wrapLineSegments(runes, tt.width, tt.tabW)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d segments, got %d: %v", len(tt.expected), len(got), got)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("segment[%d] = %d, expected %d", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestWrapLineVisualRows(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		width    int
		expected int
	}{
		{"empty line", "", 10, 1},
		{"short line", "hello", 10, 1},
		{"exact width", "12345", 5, 1},
		{"one extra char", "123456", 5, 2},
		{"two rows", "1234567890", 5, 2},
		{"three rows", "123456789012345", 5, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapLineVisualRows(tt.line, tt.width, 4)
			if got != tt.expected {
				t.Errorf("expected %d visual rows, got %d", tt.expected, got)
			}
		})
	}
}

func TestTotalVisualLines(t *testing.T) {
	lines := []string{
		"short",
		"this is a longer line that wraps",
		"",
		"another short",
	}
	total := wrapLayout(lines, 10, 4).total()
	expected := 1 + 4 + 1 + 2
	if total != expected {
		t.Errorf("expected %d total visual lines, got %d", expected, total)
	}
}

func TestBufferPosToWrapScreenPos(t *testing.T) {
	lines := []string{
		"1234567890ABCDE",
		"short",
	}
	width := 5

	visRow, screenCol := wrapLayout(lines, width, 4).rowOf(0, 0)
	if visRow != 0 || screenCol != 0 {
		t.Errorf("(0,0): expected visRow=0, screenCol=0, got %d, %d", visRow, screenCol)
	}

	visRow, screenCol = wrapLayout(lines, width, 4).rowOf(0, 3)
	if visRow != 0 || screenCol != 3 {
		t.Errorf("(0,3): expected visRow=0, screenCol=3, got %d, %d", visRow, screenCol)
	}

	visRow, screenCol = wrapLayout(lines, width, 4).rowOf(0, 7)
	if visRow != 1 || screenCol != 2 {
		t.Errorf("(0,7): expected visRow=1, screenCol=2, got %d, %d", visRow, screenCol)
	}

	visRow, screenCol = wrapLayout(lines, width, 4).rowOf(1, 0)
	if visRow != 3 || screenCol != 0 {
		t.Errorf("(1,0): expected visRow=3, screenCol=0, got %d, %d", visRow, screenCol)
	}

	visRow, screenCol = wrapLayout(lines, width, 4).rowOf(1, 3)
	if visRow != 3 || screenCol != 3 {
		t.Errorf("(1,3): expected visRow=3, screenCol=3, got %d, %d", visRow, screenCol)
	}
}

func TestWrapVisualRowToTopLine(t *testing.T) {
	lines := []string{
		"1234567890",
		"short",
	}
	width := 5

	bufLine, offset := wrapLayout(lines, width, 4).rowToTop(0)
	if bufLine != 0 || offset != 0 {
		t.Errorf("visRow=0: expected (0,0), got (%d,%d)", bufLine, offset)
	}

	bufLine, offset = wrapLayout(lines, width, 4).rowToTop(1)
	if bufLine != 0 || offset != 1 {
		t.Errorf("visRow=1: expected (0,1), got (%d,%d)", bufLine, offset)
	}

	bufLine, offset = wrapLayout(lines, width, 4).rowToTop(2)
	if bufLine != 1 || offset != 0 {
		t.Errorf("visRow=2: expected (1,0), got (%d,%d)", bufLine, offset)
	}
}

func TestBuildWrapMap(t *testing.T) {
	lines := []string{
		"1234567890",
		"short",
	}
	width := 5

	wm := wrapLayout(lines, width, 4).rows(0, 0, 4)
	if len(wm) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(wm))
	}

	if wm[0].bufLine != 0 || wm[0].startCol != 0 {
		t.Errorf("entry[0]: expected (0,0), got (%d,%d)", wm[0].bufLine, wm[0].startCol)
	}
	if wm[1].bufLine != 0 || wm[1].startCol != 5 {
		t.Errorf("entry[1]: expected (0,5), got (%d,%d)", wm[1].bufLine, wm[1].startCol)
	}
	if wm[2].bufLine != 1 || wm[2].startCol != 0 {
		t.Errorf("entry[2]: expected (1,0), got (%d,%d)", wm[2].bufLine, wm[2].startCol)
	}

	wm2 := wrapLayout(lines, width, 4).rows(0, 1, 3)
	if wm2[0].bufLine != 0 || wm2[0].startCol != 5 {
		t.Errorf("offset entry[0]: expected (0,5), got (%d,%d)", wm2[0].bufLine, wm2[0].startCol)
	}
	if wm2[1].bufLine != 1 || wm2[1].startCol != 0 {
		t.Errorf("offset entry[1]: expected (1,0), got (%d,%d)", wm2[1].bufLine, wm2[1].startCol)
	}
}

func TestBuildWrapMapLongLine(t *testing.T) {
	longLine := strings.Repeat("x", 100)
	lines := []string{longLine}
	width := 10

	wm := wrapLayout(lines, width, 4).rows(0, 0, 5)
	for i, entry := range wm {
		if entry.bufLine != 0 {
			t.Errorf("entry[%d]: expected bufLine=0, got %d", i, entry.bufLine)
		}
		expectedStart := i * 10
		if entry.startCol != expectedStart {
			t.Errorf("entry[%d]: expected startCol=%d, got %d", i, expectedStart, entry.startCol)
		}
	}
}

func wrapLayout(lines []string, width, tabW int) *rowLayout {
	return &rowLayout{lines: lines, wrap: true, width: width, tabW: tabW}
}
