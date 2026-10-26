package widgets

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func newBasicTable() *TableWidget {
	return NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Name", Width: 10},
			{Label: "Age", Width: 5},
		},
		Rows: [][]string{
			{"Alice", "30"},
			{"Bob", "25"},
			{"Charlie", "35"},
		},
	})
}

func TestTableRenderHeader(t *testing.T) {
	tbl := newBasicTable()
	s := renderWidget(tbl, 0, 0, 20, 10)

	// Row 0 should contain column header labels
	// "Name" starts at x=0
	headerText := extractText(s, 0, 0, 4)
	if headerText != "Name" {
		t.Errorf("expected header 'Name', got %q", headerText)
	}

	// Header style should be StyleHoverBold
	if s.cells[0][0].Style != term.StyleHoverBold {
		t.Errorf("header style should be StyleHoverBold, got %v", s.cells[0][0].Style)
	}
}

func TestTableRenderSeparator(t *testing.T) {
	tbl := newBasicTable()
	s := renderWidget(tbl, 0, 0, 20, 10)

	// Row 1 should be separator line with '─'
	if s.cells[1][0].Ch != '─' {
		t.Errorf("separator should use '─', got '%c'", s.cells[1][0].Ch)
	}
	if s.cells[1][0].Style != term.StyleBorder {
		t.Errorf("separator style should be StyleBorder, got %v", s.cells[1][0].Style)
	}
}

func TestTableRenderDataRows(t *testing.T) {
	tbl := newBasicTable()
	s := renderWidget(tbl, 0, 0, 20, 10)

	// Data rows start at y=2 (after header + separator)
	// Row 0: "Alice" at x=0
	row0Text := extractText(s, 0, 2, 5)
	if row0Text != "Alice" {
		t.Errorf("expected 'Alice' in first data row, got %q", row0Text)
	}

	// Row 1: "Bob" at x=0
	row1Text := extractText(s, 0, 3, 3)
	if row1Text != "Bob" {
		t.Errorf("expected 'Bob' in second data row, got %q", row1Text)
	}
}

func TestTableColumnAlignLeft(t *testing.T) {
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Col", Width: 10, Align: "left"},
		},
		Rows: [][]string{
			{"Hi"},
		},
	})
	s := renderWidget(tbl, 0, 0, 15, 5)

	// "Hi" should start at x=0 in data row (y=2)
	if s.cells[2][0].Ch != 'H' || s.cells[2][1].Ch != 'i' {
		t.Errorf("left-aligned text should start at x=0, got '%c%c'", s.cells[2][0].Ch, s.cells[2][1].Ch)
	}
}

func TestTableColumnAlignRight(t *testing.T) {
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Col", Width: 10, Align: "right"},
		},
		Rows: [][]string{
			{"Hi"},
		},
	})
	s := renderWidget(tbl, 0, 0, 15, 5)

	// "Hi" is 2 chars, column is 10 wide, so padding=8, text starts at x=8
	if s.cells[2][8].Ch != 'H' || s.cells[2][9].Ch != 'i' {
		t.Errorf("right-aligned: expected 'Hi' at x=8..9, got '%c%c'", s.cells[2][8].Ch, s.cells[2][9].Ch)
	}
}

func TestTableColumnAlignCenter(t *testing.T) {
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Col", Width: 10, Align: "center"},
		},
		Rows: [][]string{
			{"Hi"},
		},
	})
	s := renderWidget(tbl, 0, 0, 15, 5)

	// "Hi" is 2 chars, column is 10 wide, padding=8, leftPad=4, text starts at x=4
	if s.cells[2][4].Ch != 'H' || s.cells[2][5].Ch != 'i' {
		t.Errorf("center-aligned: expected 'Hi' at x=4..5, got '%c%c'", s.cells[2][4].Ch, s.cells[2][5].Ch)
	}
}

func TestTableCellTruncation(t *testing.T) {
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Col", Width: 5},
		},
		Rows: [][]string{
			{"LongText123"},
		},
	})
	s := renderWidget(tbl, 0, 0, 10, 5)

	// "LongText123" is 11 chars, column is 5, so we show first 4 chars + '…'
	text := extractText(s, 0, 2, 4)
	if text != "Long" {
		t.Errorf("truncated text prefix should be 'Long', got %q", text)
	}
	if s.cells[2][4].Ch != '…' {
		t.Errorf("truncated text should end with '…', got '%c'", s.cells[2][4].Ch)
	}
}

func TestTableKeyboardUp(t *testing.T) {
	tbl := newBasicTable()
	tbl.SetFocused(true)
	renderWidget(tbl, 0, 0, 20, 10)

	// Initial selection is 0
	if tbl.SelectedIndex() != 0 {
		t.Fatalf("initial selected should be 0, got %d", tbl.SelectedIndex())
	}

	// Move down then up
	tbl.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))
	if tbl.SelectedIndex() != 1 {
		t.Fatalf("after down, selected should be 1, got %d", tbl.SelectedIndex())
	}

	tbl.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", tcell.ModNone))
	if tbl.SelectedIndex() != 0 {
		t.Fatalf("after up, selected should be 0, got %d", tbl.SelectedIndex())
	}
}

func TestTableKeyboardDownClamp(t *testing.T) {
	tbl := newBasicTable()
	tbl.SetFocused(true)
	renderWidget(tbl, 0, 0, 20, 10)

	// Move down past end
	for i := 0; i < 10; i++ {
		tbl.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))
	}
	if tbl.SelectedIndex() != 2 {
		t.Fatalf("down clamp: selected should be 2 (last row), got %d", tbl.SelectedIndex())
	}
}

func TestTableKeyboardUpClamp(t *testing.T) {
	tbl := newBasicTable()
	tbl.SetFocused(true)
	renderWidget(tbl, 0, 0, 20, 10)

	// Already at 0, pressing up should stay at 0
	tbl.HandleEvent(tcell.NewEventKey(tcell.KeyUp, "", tcell.ModNone))
	if tbl.SelectedIndex() != 0 {
		t.Fatalf("up clamp: selected should remain 0, got %d", tbl.SelectedIndex())
	}
}

func TestTableKeyboardEnterFiresOnSelect(t *testing.T) {
	selected := -1
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Col", Width: 10},
		},
		Rows: [][]string{
			{"Row0"},
			{"Row1"},
		},
		OnSelect: func(idx int) { selected = idx },
	})
	tbl.SetFocused(true)
	renderWidget(tbl, 0, 0, 15, 10)

	tbl.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))
	tbl.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))

	if selected != 1 {
		t.Fatalf("enter should fire OnSelect with row 1, got %d", selected)
	}
}

func TestTableKeyCommands(t *testing.T) {
	var cmd string
	var row int
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Col", Width: 10},
		},
		Rows: [][]string{
			{"Row0"},
			{"Row1"},
		},
		KeyCommands: map[rune]string{
			'd': "delete",
			'r': "rename",
		},
		OnCommand: func(c string, idx int) {
			cmd = c
			row = idx
		},
	})
	tbl.SetFocused(true)
	renderWidget(tbl, 0, 0, 15, 10)

	result := tbl.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "d", tcell.ModNone))
	if result != EventConsumed {
		t.Error("key command 'd' should be consumed")
	}
	if cmd != "delete" {
		t.Errorf("expected command 'delete', got %q", cmd)
	}
	if row != 0 {
		t.Errorf("expected row 0, got %d", row)
	}
}

func TestTableKeyCommandUnknownKey(t *testing.T) {
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Col", Width: 10},
		},
		Rows: [][]string{{"Row0"}},
		KeyCommands: map[rune]string{
			'd': "delete",
		},
	})
	tbl.SetFocused(true)
	renderWidget(tbl, 0, 0, 15, 10)

	result := tbl.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone))
	if result == EventConsumed {
		t.Error("unknown key should not be consumed")
	}
}

func TestTableMouseClickSelectsRow(t *testing.T) {
	selected := -1
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Col", Width: 10},
		},
		Rows: [][]string{
			{"Row0"},
			{"Row1"},
			{"Row2"},
		},
		OnSelect: func(idx int) { selected = idx },
	})
	renderWidget(tbl, 0, 0, 15, 10)

	// Data starts at y=2 (header=0, separator=1)
	// Click on row 1 (y=3)
	click := mouseClick(1, 3)
	result := tbl.HandleEvent(click)

	if result != EventConsumed {
		t.Error("click on data row should be consumed")
	}
	if selected != 1 {
		t.Errorf("click on row 1 should select it, got %d", selected)
	}
	if tbl.SelectedIndex() != 1 {
		t.Errorf("selected index should be 1, got %d", tbl.SelectedIndex())
	}
}

func TestTableMouseClickOutsideBounds(t *testing.T) {
	tbl := newBasicTable()
	renderWidget(tbl, 5, 5, 15, 10)

	// Click outside the table rect
	click := mouseClick(0, 0)
	result := tbl.HandleEvent(click)

	if result == EventConsumed {
		t.Error("click outside table rect should be ignored")
	}
}

func TestTableMouseClickOnHeader(t *testing.T) {
	tbl := newBasicTable()
	renderWidget(tbl, 0, 0, 20, 10)

	// Click on header row (y=0)
	click := mouseClick(1, 0)
	result := tbl.HandleEvent(click)

	if result == EventConsumed {
		t.Error("click on header row should be ignored")
	}
}

func TestTableMouseClickPastLastRow(t *testing.T) {
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Col", Width: 10},
		},
		Rows: [][]string{
			{"Row0"},
		},
	})
	renderWidget(tbl, 0, 0, 15, 10)

	// Only 1 data row at y=2, click at y=5 is past it
	click := mouseClick(1, 5)
	result := tbl.HandleEvent(click)

	if result == EventConsumed {
		t.Error("click past last data row should be ignored")
	}
}

func TestTableMouseWheelUp(t *testing.T) {
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Col", Width: 10},
		},
		Rows: makeRows(20),
	})
	renderWidget(tbl, 0, 0, 15, 7) // dataH = 7 - 2 = 5

	// Scroll down first
	tbl.scrollTop = 10

	wheel := tcell.NewEventMouse(1, 3, tcell.WheelUp, tcell.ModNone)
	result := tbl.HandleEvent(wheel)

	if result != EventConsumed {
		t.Error("wheel up should be consumed")
	}
	if tbl.scrollTop != 7 { // 10 - 3 = 7
		t.Errorf("scrollTop should be 7 after wheel up, got %d", tbl.scrollTop)
	}
}

func TestTableMouseWheelDown(t *testing.T) {
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Col", Width: 10},
		},
		Rows: makeRows(20),
	})
	renderWidget(tbl, 0, 0, 15, 7) // dataH = 5

	wheel := tcell.NewEventMouse(1, 3, tcell.WheelDown, tcell.ModNone)
	result := tbl.HandleEvent(wheel)

	if result != EventConsumed {
		t.Error("wheel down should be consumed")
	}
	if tbl.scrollTop != 3 { // 0 + 3 = 3
		t.Errorf("scrollTop should be 3 after wheel down, got %d", tbl.scrollTop)
	}
}

func TestTableMouseWheelUpClamp(t *testing.T) {
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Col", Width: 10},
		},
		Rows: makeRows(20),
	})
	renderWidget(tbl, 0, 0, 15, 7)

	tbl.scrollTop = 1
	wheel := tcell.NewEventMouse(1, 3, tcell.WheelUp, tcell.ModNone)
	tbl.HandleEvent(wheel)

	if tbl.scrollTop != 0 {
		t.Errorf("scrollTop should clamp to 0, got %d", tbl.scrollTop)
	}
}

func TestTableScrollingEnsureVisible(t *testing.T) {
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Col", Width: 10},
		},
		Rows: makeRows(20),
	})
	tbl.SetFocused(true)
	renderWidget(tbl, 0, 0, 15, 7) // dataH = 5

	// Move selection beyond visible area
	for i := 0; i < 8; i++ {
		tbl.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))
	}
	// After moving, re-render to trigger ensureVisible
	renderWidget(tbl, 0, 0, 15, 7)

	if tbl.SelectedIndex() != 8 {
		t.Fatalf("selected should be 8, got %d", tbl.SelectedIndex())
	}
	// scrollTop should adjust so selected is visible: scrollTop <= 8 and scrollTop + 5 > 8
	if tbl.scrollTop > 8 || tbl.scrollTop+5 <= 8 {
		t.Errorf("scrollTop should make row 8 visible, scrollTop=%d, dataH=5", tbl.scrollTop)
	}
}

func TestTableSelectedFocusedStyle(t *testing.T) {
	tbl := newBasicTable()
	tbl.SetFocused(true)
	s := renderWidget(tbl, 0, 0, 20, 10)

	// Selected row (0) when focused should use StyleSidebarSelected
	// Data row 0 is at y=2
	if s.cells[2][0].Style != term.StyleSidebarSelected {
		t.Errorf("selected+focused row should use StyleSidebarSelected, got %v", s.cells[2][0].Style)
	}

	// Non-selected row (1) should use StyleDefault
	if s.cells[3][0].Style != term.StyleDefault {
		t.Errorf("non-selected row should use StyleDefault, got %v", s.cells[3][0].Style)
	}
}

func TestTableSelectedUnfocusedStyle(t *testing.T) {
	tbl := newBasicTable()
	tbl.SetFocused(false)
	s := renderWidget(tbl, 0, 0, 20, 10)

	// Selected row when unfocused should use StyleDefault (not selected style)
	if s.cells[2][0].Style != term.StyleDefault {
		t.Errorf("selected but unfocused row should use StyleDefault, got %v", s.cells[2][0].Style)
	}
}

func TestTableEmptyNoRows(t *testing.T) {
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Col", Width: 10},
		},
		Rows: [][]string{},
	})
	// Should not panic
	renderWidget(tbl, 0, 0, 15, 5)

	if tbl.SelectedIndex() != 0 {
		t.Errorf("empty table selected should be 0, got %d", tbl.SelectedIndex())
	}
}

func TestTableEmptyNoColumns(t *testing.T) {
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{},
		Rows:    [][]string{{"a"}},
	})
	// Should not panic (early return in Render)
	renderWidget(tbl, 0, 0, 15, 5)
}

func TestTableSingleRow(t *testing.T) {
	selected := -1
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Col", Width: 10},
		},
		Rows: [][]string{
			{"Only"},
		},
		OnSelect: func(idx int) { selected = idx },
	})
	tbl.SetFocused(true)
	renderWidget(tbl, 0, 0, 15, 5)

	// Down should not move past single row
	tbl.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))
	if tbl.SelectedIndex() != 0 {
		t.Errorf("single row: down should stay at 0, got %d", tbl.SelectedIndex())
	}

	// Enter should fire OnSelect
	tbl.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	if selected != 0 {
		t.Errorf("enter on single row should fire OnSelect(0), got %d", selected)
	}
}

func TestTableSetSelectedIndex(t *testing.T) {
	tbl := newBasicTable()

	tbl.SetSelectedIndex(2)
	if tbl.SelectedIndex() != 2 {
		t.Errorf("SetSelectedIndex(2) should set to 2, got %d", tbl.SelectedIndex())
	}

	// Clamp beyond range
	tbl.SetSelectedIndex(100)
	if tbl.SelectedIndex() != 2 {
		t.Errorf("SetSelectedIndex(100) should clamp to 2, got %d", tbl.SelectedIndex())
	}

	tbl.SetSelectedIndex(-5)
	if tbl.SelectedIndex() != 0 {
		t.Errorf("SetSelectedIndex(-5) should clamp to 0, got %d", tbl.SelectedIndex())
	}
}

func TestTableFocusable(t *testing.T) {
	tbl := newBasicTable()

	if !tbl.Focusable() {
		t.Error("table should be focusable")
	}

	tbl.SetFocused(true)
	if !tbl.IsFocused() {
		t.Error("table should be focused after SetFocused(true)")
	}

	tbl.SetFocused(false)
	if tbl.IsFocused() {
		t.Error("table should not be focused after SetFocused(false)")
	}
}

func TestTableOnSelectFiredOnNavigation(t *testing.T) {
	// OnSelect should fire when selection changes via keyboard
	selections := []int{}
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Col", Width: 10},
		},
		Rows: [][]string{
			{"Row0"},
			{"Row1"},
			{"Row2"},
		},
		OnSelect: func(idx int) { selections = append(selections, idx) },
	})
	tbl.SetFocused(true)
	renderWidget(tbl, 0, 0, 15, 10)

	tbl.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))
	tbl.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))

	if len(selections) != 2 {
		t.Fatalf("expected 2 OnSelect calls, got %d", len(selections))
	}
	if selections[0] != 1 || selections[1] != 2 {
		t.Errorf("expected selections [1, 2], got %v", selections)
	}
}

func TestTableColumnSeparatorSpacing(t *testing.T) {
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "A", Width: 3},
			{Label: "B", Width: 3},
		},
		Rows: [][]string{
			{"ab", "cd"},
		},
	})
	s := renderWidget(tbl, 0, 0, 20, 5)

	// Column A is 3 wide (x=0..2), then 2 space separators (x=3,4), column B at x=5
	// In data row (y=2), "ab" at x=0..1, spaces, then "cd" at x=5..6
	row0 := extractText(s, 0, 2, 8)
	if len(row0) < 7 {
		t.Logf("row0 text: %q", row0)
	}
	// Verify second column value location
	if s.cells[2][5].Ch != 'c' || s.cells[2][6].Ch != 'd' {
		t.Errorf("second column should start at x=5, got '%c%c' at x=5,6", s.cells[2][5].Ch, s.cells[2][6].Ch)
	}
}

func TestTableHeightAndWidthZero(t *testing.T) {
	tbl := newBasicTable()
	if tbl.Height() != 0 {
		t.Errorf("table Height() should be 0 (fills parent), got %d", tbl.Height())
	}
	if tbl.Width() != 0 {
		t.Errorf("table Width() should be 0 (fills parent), got %d", tbl.Width())
	}
}

func TestTableScrollbarOffWidgetCaptureLifecycle(t *testing.T) {
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{{Label: "Name"}},
		Rows:    makeRows(20),
	})
	tbl.SetRect(Rect{X: 7, Y: 3, W: 6, H: 6})
	tbl.Render(newVirtualSurface(6, 6))
	invalidations := 0
	tbl.SetPointerCaptureInvalidated(func() { invalidations++ })

	if got := tbl.HandleEvent(tcell.NewEventMouse(12, 5, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("scrollbar press result = %v, want captured", got)
	}
	if got := tbl.HandleEvent(tcell.NewEventMouse(80, 80, tcell.Button1, 0)); got != EventCaptured {
		t.Fatalf("off-widget drag result = %v, want captured", got)
	}
	if got := tbl.HandleEvent(tcell.NewEventMouse(80, 80, tcell.ButtonNone, 0)); got != EventConsumed {
		t.Fatalf("off-widget release result = %v, want consumed", got)
	}
	if tbl.OwnsPointerCapture() || invalidations != 0 {
		t.Fatalf("normal release retained or invalidated capture: invalidations=%d", invalidations)
	}

	tbl.HandleEvent(tcell.NewEventMouse(12, 5, tcell.Button1, 0))
	if !tbl.InvalidatePointerInteraction() || tbl.OwnsPointerCapture() || invalidations != 1 {
		t.Fatalf("explicit invalidation failed: owns=%v invalidations=%d", tbl.OwnsPointerCapture(), invalidations)
	}
}

// --- helpers ---

func makeRows(n int) [][]string {
	rows := make([][]string, n)
	for i := range n {
		rows[i] = []string{string(rune('A' + i%26))}
	}
	return rows
}

func extractText(s *testSurface, x, y, length int) string {
	runes := make([]rune, 0, length)
	for i := 0; i < length; i++ {
		ch := s.cells[y][x+i].Ch
		if ch == 0 {
			break
		}
		runes = append(runes, ch)
	}
	return string(runes)
}

func TestTableActivateFiresOnEnterAndClickNotArrows(t *testing.T) {
	tbl := newBasicTable()
	var activated []int
	tbl.Config.OnActivate = func(i int) { activated = append(activated, i) }
	at := time.Unix(1000, 0)
	tbl.now = func() time.Time { return at }
	tbl.SetFocused(true)
	renderWidget(tbl, 0, 0, 20, 10)

	tbl.HandleEvent(tcell.NewEventKey(tcell.KeyDown, "", tcell.ModNone))
	if len(activated) != 0 {
		t.Fatalf("arrow keys should not activate, got %v", activated)
	}
	tbl.HandleEvent(tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
	tbl.HandleEvent(mouseClick(2, 4))
	tbl.HandleEvent(mouseClick(2, 4))
	tbl.HandleEvent(mouseRelease(2, 4))
	if len(activated) != 1 || tbl.SelectedIndex() != 2 {
		t.Fatalf("a single click should select without activating, got %v", activated)
	}
	at = at.Add(100 * time.Millisecond)
	tbl.HandleEvent(mouseClick(2, 4))
	tbl.HandleEvent(mouseRelease(2, 4))
	if len(activated) != 2 || activated[0] != 1 || activated[1] != 2 {
		t.Fatalf("enter then a double click should activate rows 1 and 2, got %v", activated)
	}
}

func TestTableRowStyles(t *testing.T) {
	tbl := newBasicTable()
	tbl.Config.RowStyles = []term.Style{0, term.StyleMuted}
	s := renderWidget(tbl, 0, 0, 20, 10)
	if s.cells[2][0].Style != term.StyleDefault || s.cells[3][0].Style != term.StyleMuted {
		t.Fatalf("row styles = %v, %v", s.cells[2][0].Style, s.cells[3][0].Style)
	}
}

func TestTableLoadingAndEmptyStates(t *testing.T) {
	tbl := NewTableWidget(TableConfig{Columns: []TableColumn{{Label: "Name"}}, EmptyText: "No keys yet"})
	s := renderWidget(tbl, 0, 0, 30, 8)
	if got := strings.TrimSpace(rowText(s, 4)); got != "No keys yet" {
		t.Fatalf("empty state = %q", got)
	}
	tbl.Config.Loading = true
	s = renderWidget(tbl, 0, 0, 30, 8)
	if got := strings.TrimSpace(rowText(s, 4)); got != "Loading…" {
		t.Fatalf("loading state = %q", got)
	}
	tbl.Config.Rows = [][]string{{"a"}}
	s = renderWidget(tbl, 0, 0, 30, 8)
	if got := rowText(s, 0); !strings.HasSuffix(got, "Loading…") || rowText(s, 2) != "a" {
		t.Fatalf("loading with rows keeps them and marks the header: %q / %q", got, rowText(s, 2))
	}
}

func TestInsetKeepsScrollbarAtEdge(t *testing.T) {
	rows := make([][]string, 20)
	for i := range rows {
		rows[i] = []string{strings.Repeat("x", 30)}
	}
	tbl := NewTableWidget(TableConfig{Columns: []TableColumn{{Label: "Name"}}, Rows: rows, Inset: 1})
	tbl.SetFocused(true)
	s := renderWidget(tbl, 0, 0, 12, 8)
	if got := rowText(s, 2); got[:len(got)-len("█")] != " xxxxxxxx… " {
		t.Fatalf("row = %q, want 1 cell inset on both sides", got)
	}
	if s.cells[2][0].Style != term.StyleSidebarSelected || s.cells[2][10].Style != term.StyleSidebarSelected {
		t.Fatalf("selection should span the inset: %v %v", s.cells[2][0].Style, s.cells[2][10].Style)
	}
	if s.cells[2][11].Style == term.StyleSidebarSelected || s.cells[2][11].Ch == ' ' {
		t.Fatalf("scrollbar should stay at the right edge, got %q", s.cells[2][11].Ch)
	}
}

func TestTableDoubleClickNeedsTwoQuickClicksOnTheSameRow(t *testing.T) {
	tbl := newBasicTable()
	activated := 0
	tbl.Config.OnActivate = func(int) { activated++ }
	at := time.Unix(1000, 0)
	tbl.now = func() time.Time { return at }
	renderWidget(tbl, 0, 0, 20, 10)
	click := func(x, y int, after time.Duration) {
		at = at.Add(after)
		tbl.HandleEvent(mouseClick(x, y))
		tbl.HandleEvent(mouseRelease(x, y))
	}

	click(2, 4, 0)
	click(2, 4, 600*time.Millisecond)
	if activated != 0 {
		t.Fatal("two clicks 600ms apart activated the row")
	}
	click(2, 0, 100*time.Millisecond)
	click(2, 4, 100*time.Millisecond)
	if activated != 0 {
		t.Fatal("a header click between the row clicks still counted as a double-click")
	}
	click(2, 4, 100*time.Millisecond)
	if activated != 1 {
		t.Fatalf("a quick second click on the row should activate it, got %d", activated)
	}
}

func TestTableRightClickWithoutMenuIsIgnored(t *testing.T) {
	tbl := newBasicTable()
	renderWidget(tbl, 0, 0, 20, 10)
	if got := tbl.HandleEvent(tcell.NewEventMouse(1, 3, tcell.Button2, tcell.ModNone)); got != EventIgnored {
		t.Errorf("right-click result = %v, want ignored so the caller can show its own menu", got)
	}
}

func newWideTable() *TableWidget {
	return NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "A", Width: 10},
			{Label: "B", Width: 10},
			{Label: "C", Width: 10},
		},
		Rows: [][]string{{"a1", "b1", "c1"}},
	})
}

func keyPress(k tcell.Key) *tcell.EventKey {
	return tcell.NewEventKey(k, "", tcell.ModNone)
}

func TestTableScrollsOverflowingColumnsWithLeftRight(t *testing.T) {
	tbl := newWideTable()
	s := renderWidget(tbl, 0, 0, 20, 6)
	if got := rowText(s, 2); got != "a1          b1" {
		t.Fatalf("row before scrolling = %q", got)
	}
	if s.cells[5][0].Ch != '▄' || s.cells[5][19].Ch != '▄' {
		t.Fatalf("horizontal scrollbar should fill the last row, got %q", rowText(s, 5))
	}

	if got := tbl.HandleEvent(keyPress(tcell.KeyRight)); got != EventConsumed {
		t.Fatalf("Right on an overflowing table = %v, want consumed", got)
	}
	s = renderWidget(tbl, 0, 0, 20, 6)
	if got := rowText(s, 0); got != "B           C" {
		t.Fatalf("header after Right = %q, want it to scroll to column B", got)
	}
	if got := rowText(s, 2); got != "b1          c1" {
		t.Fatalf("row after Right = %q, want it aligned with the header", got)
	}

	tbl.HandleEvent(keyPress(tcell.KeyRight))
	s = renderWidget(tbl, 0, 0, 20, 6)
	if got := rowText(s, 0); got != "          C" {
		t.Fatalf("header at the right end = %q", got)
	}

	tbl.HandleEvent(keyPress(tcell.KeyLeft))
	tbl.HandleEvent(keyPress(tcell.KeyLeft))
	s = renderWidget(tbl, 0, 0, 20, 6)
	if got := rowText(s, 0); got != "A           B" {
		t.Fatalf("header after Left = %q, want the start", got)
	}
}

func TestTableLeftRightIgnoredWhenColumnsFit(t *testing.T) {
	tbl := newBasicTable()
	s := renderWidget(tbl, 0, 0, 20, 6)
	if got := tbl.HandleEvent(keyPress(tcell.KeyRight)); got != EventIgnored {
		t.Fatalf("Right on a table that fits = %v, want ignored", got)
	}
	if s.cells[5][0].Ch == '▄' {
		t.Fatal("a table that fits should not draw a horizontal scrollbar")
	}
}

func TestTableHorizontalWheel(t *testing.T) {
	tbl := newWideTable()
	renderWidget(tbl, 0, 0, 20, 6)
	tbl.HandleEvent(tcell.NewEventMouse(1, 2, tcell.WheelDown, tcell.ModShift))
	if tbl.scrollLeft != 3 || tbl.scrollTop != 0 {
		t.Fatalf("Shift+wheel down: scrollLeft=%d scrollTop=%d, want 3 and 0", tbl.scrollLeft, tbl.scrollTop)
	}
	for range 3 {
		tbl.HandleEvent(tcell.NewEventMouse(1, 2, tcell.WheelRight, tcell.ModNone))
	}
	if tbl.scrollLeft != 12 {
		t.Fatalf("wheel right: scrollLeft=%d, want 12", tbl.scrollLeft)
	}
	tbl.HandleEvent(tcell.NewEventMouse(1, 2, tcell.WheelRight, tcell.ModNone))
	if tbl.scrollLeft != 14 {
		t.Fatalf("wheel right past the end: scrollLeft=%d, want clamped to 14", tbl.scrollLeft)
	}
	tbl.HandleEvent(tcell.NewEventMouse(1, 2, tcell.WheelUp, tcell.ModShift))
	tbl.HandleEvent(tcell.NewEventMouse(1, 2, tcell.WheelLeft, tcell.ModNone))
	if tbl.scrollLeft != 8 {
		t.Fatalf("wheel left: scrollLeft=%d, want 8", tbl.scrollLeft)
	}
}

func TestTableHorizontalScrollbarClickScrollsWithoutSelecting(t *testing.T) {
	selected := -1
	tbl := newWideTable()
	tbl.Config.Rows = makeRows(10)
	tbl.Config.OnSelect = func(i int) { selected = i }
	renderWidget(tbl, 0, 0, 20, 6)
	if got := tbl.HandleEvent(mouseClick(18, 5)); got != EventCaptured {
		t.Fatalf("click on the horizontal track = %v, want captured", got)
	}
	tbl.HandleEvent(mouseRelease(18, 5))
	if tbl.scrollLeft == 0 {
		t.Fatal("click near the right of the track should scroll right")
	}
	if selected != -1 {
		t.Fatalf("click on the scrollbar selected row %d", selected)
	}
}

func TestTableAutoColumnsKeepTheirMinimumWidth(t *testing.T) {
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{
			{Label: "Name"},
			{Label: "Notes", MinWidth: 8},
			{Label: "Fixed", Width: 10},
		},
		Rows: [][]string{{"alice", "longer notes", "x"}},
	})
	renderWidget(tbl, 0, 0, 20, 6)
	if got, want := tbl.widths, []int{4, 8, 10}; !slices.Equal(got, want) {
		t.Fatalf("widths = %v, want %v", got, want)
	}
	renderWidget(tbl, 0, 0, 40, 6)
	if got, want := tbl.widths, []int{13, 13, 10}; !slices.Equal(got, want) {
		t.Fatalf("widths with room = %v, want the space split equally: %v", got, want)
	}
}

func TestTableFullwidthRuneNotDrawnInLastVisibleColumn(t *testing.T) {
	tbl := NewTableWidget(TableConfig{
		Columns: []TableColumn{{Label: "X", Width: 30}},
		Rows:    [][]string{{"a漢漢漢漢漢漢漢"}},
	})
	s := renderWidget(tbl, 0, 0, 10, 5)
	if s.cells[2][9].Ch != ' ' {
		t.Fatalf("last visible column holds %q, want a space instead of a fullwidth rune", s.cells[2][9].Ch)
	}
}
