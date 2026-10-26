package widgets

import (
	"slices"
	"testing"
	"time"

	"github.com/gdamore/tcell/v3"

	"github.com/eugenioenko/ttt/internal/term"
)

func doubleClick(tbl *TableWidget, x, y int) {
	tbl.clickAt = 0
	clickAndRelease(tbl, x, y)
	clickAndRelease(tbl, x, y)
}

func highlighted(s *testSurface, y int) []int {
	var xs []int
	for x, c := range s.cells[y] {
		if c.BgStyle == term.StyleSelection {
			xs = append(xs, x)
		}
	}
	return xs
}

func newCellTable(cols []TableColumn, rows [][]string) *TableWidget {
	tbl := NewTableWidget(TableConfig{Columns: cols, Rows: rows})
	at := time.Unix(1000, 0)
	tbl.now = func() time.Time { return at }
	return tbl
}

func TestTableDoubleClickHighlightsDrawnText(t *testing.T) {
	tbl := newCellTable([]TableColumn{
		{Label: "Name", Width: 10},
		{Label: "Age", Width: 5, Align: "right"},
	}, [][]string{{"Alice", "30"}, {"Bob", "25"}})
	renderWidget(tbl, 0, 0, 20, 10)

	doubleClick(tbl, 8, 2)
	s := renderWidget(tbl, 0, 0, 20, 10)
	if got := highlighted(s, 2); !slices.Equal(got, []int{0, 1, 2, 3, 4}) {
		t.Fatalf("highlight = %v, want only the drawn text of Alice", got)
	}
	if text, ok := tbl.SelectedCellText(); !ok || text != "Alice" {
		t.Fatalf("selected cell = %q %v", text, ok)
	}

	doubleClick(tbl, 13, 3)
	s = renderWidget(tbl, 0, 0, 20, 10)
	if got := highlighted(s, 3); !slices.Equal(got, []int{15, 16}) {
		t.Fatalf("right-aligned highlight = %v, want [15 16]", got)
	}
	if got := highlighted(s, 2); len(got) != 0 {
		t.Fatalf("the earlier cell stayed highlighted: %v", got)
	}
}

func TestTableTruncatedCellHighlightsVisibleTextAndCopiesFullValue(t *testing.T) {
	tbl := newCellTable([]TableColumn{{Label: "Id", Width: 5}, {Label: "N", Width: 3}},
		[][]string{{"abcdefghij", "x"}, {"日本語テキスト", "y"}})
	tbl.SetFocused(true)
	renderWidget(tbl, 0, 0, 20, 10)

	doubleClick(tbl, 1, 2)
	s := renderWidget(tbl, 0, 0, 20, 10)
	if got := rowText(s, 2); got != "abcd…  x" {
		t.Fatalf("row = %q", got)
	}
	if got := highlighted(s, 2); !slices.Equal(got, []int{0, 1, 2, 3, 4}) {
		t.Fatalf("highlight = %v, want the visible text including the ellipsis", got)
	}
	if s.cells[2][0].Style != term.StyleSidebarSelected {
		t.Fatalf("the selected row should keep its style under the highlight, got %v", s.cells[2][0].Style)
	}
	if text, _ := tbl.SelectedCellText(); text != "abcdefghij" {
		t.Fatalf("copied value = %q, want the full value", text)
	}

	doubleClick(tbl, 0, 3)
	s = renderWidget(tbl, 0, 0, 20, 10)
	if got := highlighted(s, 3); !slices.Equal(got, []int{0, 2, 4}) {
		t.Fatalf("wide-rune highlight = %v, want the two drawn runes and the ellipsis", got)
	}
	if text, _ := tbl.SelectedCellText(); text != "日本語テキスト" {
		t.Fatalf("copied value = %q", text)
	}
}

func TestTableCellSelectionClears(t *testing.T) {
	tbl := newCellTable([]TableColumn{{Label: "Name", Width: 10}, {Label: "Age", Width: 5}},
		[][]string{{"Alice", "30"}, {"Bob", "25"}, {"", "1"}})
	renderWidget(tbl, 0, 0, 20, 10)
	selected := func() bool { _, ok := tbl.SelectedCellText(); return ok }

	doubleClick(tbl, 1, 2)
	tbl.HandleEvent(keyPress(tcell.KeyEscape))
	if selected() {
		t.Fatal("Escape kept the selection")
	}

	doubleClick(tbl, 1, 2)
	at := time.Unix(2000, 0)
	tbl.now = func() time.Time { return at }
	clickAndRelease(tbl, 1, 3)
	if selected() {
		t.Fatal("a click on another row kept the selection")
	}
	if tbl.SelectedIndex() != 1 {
		t.Fatalf("a single click should still select the row, got %d", tbl.SelectedIndex())
	}

	doubleClick(tbl, 1, 3)
	tbl.Config.Rows = [][]string{{"Alice", "30"}, {"Bob", "25"}, {"", "1"}}
	renderWidget(tbl, 0, 0, 20, 10)
	if text, ok := tbl.SelectedCellText(); !ok || text != "Bob" {
		t.Fatalf("a render with the same rows should keep the selection, got %q %v", text, ok)
	}
	tbl.Config.Rows = [][]string{{"Alice", "30"}, {"Bob", "26"}}
	renderWidget(tbl, 0, 0, 20, 10)
	if selected() {
		t.Fatal("a render that replaced the row kept the selection")
	}
	doubleClick(tbl, 1, 3)
	tbl.Config.Rows = [][]string{{"Alice", "30"}}
	renderWidget(tbl, 0, 0, 20, 10)
	if selected() {
		t.Fatal("a render that removed the row kept the selection")
	}

	tbl.Config.Rows = [][]string{{"Alice", "30"}, {"Bob", "25"}, {"", "1"}}
	renderWidget(tbl, 0, 0, 20, 10)
	doubleClick(tbl, 1, 4)
	doubleClick(tbl, 11, 2)
	if selected() {
		t.Fatal("a double-click in the gap between columns selected a cell")
	}
	doubleClick(tbl, 1, 4)
	if selected() {
		t.Fatal("a double-click on an empty cell selected it")
	}
}

func TestTableDoubleClickInActionsColumnRunsActions(t *testing.T) {
	tbl, commands, _ := newActionsTable(TableColumn{Label: "Name"})
	renderWidget(tbl, 0, 0, 30, 6)
	doubleClick(tbl, 19, 2)
	if len(*commands) != 2 {
		t.Fatalf("double-click on Open ran %v, want it twice", *commands)
	}
	if _, ok := tbl.SelectedCellText(); ok {
		t.Fatal("a double-click on an action selected a cell")
	}
	doubleClick(tbl, 1, 2)
	if text, ok := tbl.SelectedCellText(); !ok || text != "a1" {
		t.Fatalf("cell in a table with actions = %q %v", text, ok)
	}
}
