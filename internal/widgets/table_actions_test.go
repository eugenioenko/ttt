package widgets

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/gdamore/tcell/v3"
)

type tableMenuCall struct {
	entries []MenuEntry
	row     int
	x, y    int
}

func newActionsTable(cols ...TableColumn) (*TableWidget, *[]string, *[]tableMenuCall) {
	var commands []string
	var menus []tableMenuCall
	cols = append(cols, TableColumn{Actions: []TableAction{
		{Label: "Open", Command: "open"},
		{Label: "Copy", Command: "copy", Style: term.StyleMuted},
		{Menu: true},
	}})
	tbl := NewTableWidget(TableConfig{
		Columns:  cols,
		Rows:     [][]string{{"a1", "b1", "c1"}, {"a2", "b2", "c2"}},
		NodeMenu: []MenuEntry{{Label: "Delete", Command: "delete"}},
		OnCommand: func(cmd string, row int) {
			commands = append(commands, fmt.Sprintf("%s:%d", cmd, row))
		},
		OnMenu: func(entries []MenuEntry, row, x, y int) {
			menus = append(menus, tableMenuCall{entries, row, x, y})
		},
	})
	return tbl, &commands, &menus
}

func clickAndRelease(w Widget, x, y int) EventResult {
	r := w.HandleEvent(mouseClick(x, y))
	w.HandleEvent(mouseRelease(x, y))
	return r
}

func TestTableActionsColumnSizedFromLabels(t *testing.T) {
	tbl, _, _ := newActionsTable(TableColumn{Label: "Name"})
	s := renderWidget(tbl, 0, 0, 30, 6)
	if got, want := tbl.widths[1], textwidth.String("Open Copy ⋮"); got != want {
		t.Fatalf("actions column width = %d, want %d", got, want)
	}
	if got := rowText(s, 2); got != "a1                 Open Copy ⋮" {
		t.Fatalf("row = %q, want the actions at the right edge in place of cell text", got)
	}
	open, copyCell, more := s.cells[2][19], s.cells[2][24], s.cells[2][29]
	if !open.Underline || open.Style != term.StyleDefault {
		t.Errorf("Open cell = %+v, want an underlined link in the row style", open)
	}
	if copyCell.Underline || copyCell.Style != term.StyleMuted {
		t.Errorf("muted action cell = %+v, want the muted style", copyCell)
	}
	if more.Ch != '⋮' || more.Underline {
		t.Errorf("menu cell = %+v, want a plain ⋮", more)
	}
}

func TestTableActionsUseRowStyleOnSelectedRow(t *testing.T) {
	tbl, _, _ := newActionsTable(TableColumn{Label: "Name"})
	tbl.SetFocused(true)
	s := renderWidget(tbl, 0, 0, 30, 6)
	for _, x := range []int{19, 24, 29} {
		if c := s.cells[2][x]; c.Style != term.StyleSidebarSelected {
			t.Errorf("selected row cell %d style = %v, want the selection style so it stays readable", x, c.Style)
		}
	}
}

func TestTableActionsColumnPinnedUnderHorizontalScroll(t *testing.T) {
	tbl, commands, _ := newActionsTable(
		TableColumn{Label: "A", Width: 10},
		TableColumn{Label: "B", Width: 10},
		TableColumn{Label: "C", Width: 10},
	)
	s := renderWidget(tbl, 0, 0, 30, 7)
	if !tbl.hRange().visible() {
		t.Fatal("table should overflow and scroll horizontally")
	}
	if got := rowText(s, 2); got != "a1          b1     Open Copy ⋮" {
		t.Fatalf("row before scrolling = %q", got)
	}
	tbl.HandleEvent(keyPress(tcell.KeyRight))
	s = renderWidget(tbl, 0, 0, 30, 7)
	if got := rowText(s, 2); got != "b1          c1     Open Copy ⋮" {
		t.Fatalf("row after scrolling = %q, want the data scrolled and the actions pinned", got)
	}
	if got := rowText(s, 0); !strings.HasPrefix(got, "B           C") {
		t.Fatalf("header after scrolling = %q", got)
	}
	clickAndRelease(tbl, 19, 3)
	if !slices.Equal(*commands, []string{"open:1"}) {
		t.Fatalf("click on pinned Open after scrolling = %v", *commands)
	}
}

func TestTableActionClicks(t *testing.T) {
	tbl, commands, menus := newActionsTable(TableColumn{Label: "Name"})
	activated := 0
	tbl.Config.OnActivate = func(int) { activated++ }
	renderWidget(tbl, 2, 1, 30, 6)

	if got := clickAndRelease(tbl, 2+19, 4); got != EventConsumed {
		t.Fatalf("click on Open = %v", got)
	}
	clickAndRelease(tbl, 2+22, 4)
	clickAndRelease(tbl, 2+25, 3)
	if !slices.Equal(*commands, []string{"open:1", "open:1", "copy:0"}) {
		t.Fatalf("commands = %v", *commands)
	}
	if tbl.SelectedIndex() != 0 {
		t.Fatalf("click on an action should select its row, got %d", tbl.SelectedIndex())
	}
	if activated != 0 {
		t.Fatal("two quick clicks on an action activated the row")
	}

	clickAndRelease(tbl, 2+29, 4)
	if len(*menus) != 1 {
		t.Fatalf("click on ⋮ opened %d menus", len(*menus))
	}
	m := (*menus)[0]
	if m.row != 1 || m.x != 2+29 || m.y != 5 || m.entries[0].Command != "delete" {
		t.Fatalf("menu = %+v, want node_menu for row 1 anchored below the ⋮ at (31, 5)", m)
	}

	tbl.Config.Columns[1].Actions[2].Entries = []MenuEntry{{Label: "Own", Command: "own"}}
	renderWidget(tbl, 2, 1, 30, 6)
	clickAndRelease(tbl, 2+29, 3)
	if got := (*menus)[1].entries[0].Command; got != "own" {
		t.Fatalf("menu with its own entries opened %q", got)
	}

	clickAndRelease(tbl, 2+23, 3)
	clickAndRelease(tbl, 2+18, 3)
	if len(*commands) != 3 {
		t.Fatalf("clicks beside the actions ran %v", (*commands)[3:])
	}
}

func TestTableKeysUnchangedWithActionsColumn(t *testing.T) {
	tbl, commands, _ := newActionsTable(TableColumn{Label: "Name"})
	activated := -1
	tbl.Config.OnActivate = func(i int) { activated = i }
	tbl.Config.KeyCommands = map[rune]string{'d': "delete"}
	renderWidget(tbl, 0, 0, 30, 6)
	tbl.HandleEvent(keyPress(tcell.KeyDown))
	tbl.HandleEvent(keyPress(tcell.KeyEnter))
	tbl.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "d", tcell.ModNone))
	if activated != 1 || !slices.Equal(*commands, []string{"delete:1"}) {
		t.Fatalf("activated=%d commands=%v", activated, *commands)
	}
}
