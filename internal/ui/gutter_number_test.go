package ui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func referenceGutterNumber(style, numStr string, gutterW int) string {
	pad := func(n int) string { return strings.Repeat(" ", max(n, 0)) }
	switch style {
	case "minimal":
		return pad(gutterW-1-len(numStr)) + numStr + " "
	case "extended":
		return "  " + pad(gutterW-5-len(numStr)) + numStr + "   "
	default:
		return " " + pad(gutterW-3-len(numStr)) + numStr + "  "
	}
}

func gridRow(grid [][]term.Cell, y, n int) string {
	var sb strings.Builder
	for x := 0; x < n && x < len(grid[y]); x++ {
		sb.WriteRune(grid[y][x].Ch)
	}
	return sb.String()
}

func TestDrawGutterNumberMatchesStringLayout(t *testing.T) {
	for _, style := range []string{"minimal", "compact", "extended"} {
		for gutterW := 1; gutterW <= 9; gutterW++ {
			for _, num := range []int{0, 1, 9, 42, 1234, 123456} {
				numStr := ""
				if num > 0 {
					numStr = strconv.Itoa(num)
				}
				want := referenceGutterNumber(style, numStr, gutterW)
				grid := makeGrid(32, 1)
				for x := range grid[0] {
					grid[0][x].Ch = '.'
				}
				e := &EditorPaneWidget{GutterStyle: style}
				e.drawGutterNumber(NewRenderSurface(grid, Rect{W: 32, H: 1}), 0, gutterW, num, term.Cell{Style: term.StyleLineNumber})
				if got := gridRow(grid, 0, len(want)); got != want {
					t.Fatalf("%s w=%d num=%d: got %q want %q", style, gutterW, num, got, want)
				}
				if grid[0][len(want)].Ch != '.' {
					t.Fatalf("%s w=%d num=%d: wrote past %d", style, gutterW, num, len(want))
				}
				for x := range len(want) {
					if grid[0][x].Style != term.StyleLineNumber {
						t.Fatalf("%s w=%d num=%d: cell %d style %v", style, gutterW, num, x, grid[0][x].Style)
					}
				}
			}
		}
	}
}

func TestEditorGutterNumbersAcrossDigitWidths(t *testing.T) {
	for _, style := range []string{"minimal", "compact", "extended"} {
		for _, n := range []int{9, 10, 99, 100, 1000} {
			lines := make([]string, n)
			e := newEditorWithLines(lines...)
			e.SetRect(Rect{W: 40, H: 4})
			e.LineNumbers = true
			e.GutterStyle = style
			e.Viewport.TopLine = n - 2
			e.Cursor.Line = n - 1
			gutterW := e.GutterWidth()
			grid := makeGrid(40, 4)
			e.Render(NewRenderSurface(grid, Rect{W: 40, H: 4}))
			for y, line := range []int{n - 1, n} {
				want := referenceGutterNumber(style, strconv.Itoa(line), gutterW)
				if got := gridRow(grid, y, len(want)); got != want {
					t.Fatalf("%s n=%d row %d: got %q want %q", style, n, y, got, want)
				}
			}
		}
	}
}
