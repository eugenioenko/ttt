package ui

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
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
				e := &EditorPaneWidget{GutterStyle: style}
				for _, limit := range []int{-1, gutterW} {
					grid := makeGrid(32, 1)
					for x := range grid[0] {
						grid[0][x].Ch = '.'
					}
					e.drawGutterNumber(NewRenderSurface(grid, Rect{W: 32, H: 1}), 0, gutterW, num, limit, term.Cell{})
					expect := want
					if limit >= 0 && len(expect) > limit {
						expect = expect[:limit]
					}
					if got := gridRow(grid, 0, len(expect)); got != expect {
						t.Fatalf("%s w=%d num=%d limit=%d: got %q want %q", style, gutterW, num, limit, got, expect)
					}
					if grid[0][len(expect)].Ch != '.' {
						t.Fatalf("%s w=%d num=%d limit=%d: wrote past %d", style, gutterW, num, limit, len(expect))
					}
				}
			}
		}
	}
}

func TestDiffGutterMatchesFormattedLayout(t *testing.T) {
	for width := 1; width <= 8; width++ {
		for _, num := range []int{0, 7, 42, 12345} {
			for _, kind := range []diff.LineKind{diff.Context, diff.Added, diff.Deleted, diff.Collapsed} {
				number := ""
				if num > 0 {
					number = fmt.Sprintf("%d", num)
				}
				marker := map[diff.LineKind]rune{diff.Context: ' ', diff.Added: '+', diff.Deleted: '−', diff.Collapsed: '▶'}[kind]
				want := []rune(fmt.Sprintf("%*s %c", width-2, number, marker))
				if len(want) > width {
					want = want[:width]
				}
				grid := makeGrid(32, 1)
				for x := range grid[0] {
					grid[0][x].Ch = '.'
				}
				renderDiffGutterWithSigns(NewRenderSurface(grid, Rect{W: 32, H: 1}), 0, 0, width, diff.SideLine{Num: num, Kind: kind}, term.StyleDefault, true, true)
				if got := gridRow(grid, 0, len(want)); got != string(want) || grid[0][len(want)].Ch != '.' {
					t.Fatalf("w=%d num=%d kind=%v: got %q want %q", width, num, kind, gridRow(grid, 0, len(want)+1), string(want))
				}
			}
		}
	}
}
