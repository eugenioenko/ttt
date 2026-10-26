package widgets

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestKeyValueListHeight(t *testing.T) {
	kv := NewKeyValueListWidget([]KeyValueEntry{
		{Key: "Name", Value: "Alice"},
		{Key: "Age", Value: "30"},
		{Key: "City", Value: "Berlin"},
	})
	if got := kv.Height(); got != 3 {
		t.Errorf("expected Height()=3, got %d", got)
	}
}

func TestKeyValueListWidthReturnsZero(t *testing.T) {
	kv := NewKeyValueListWidget([]KeyValueEntry{
		{Key: "K", Value: "V"},
	})
	if got := kv.Width(); got != 0 {
		t.Errorf("expected Width()=0, got %d", got)
	}
}

func TestKeyValueListRender(t *testing.T) {
	kv := NewKeyValueListWidget([]KeyValueEntry{
		{Key: "Name", Value: "Alice"},
		{Key: "Age", Value: "30"},
	})

	s := renderWidget(kv, 0, 0, 30, 5)

	// keyColWidth = max(len("Name"), len("Age")) + 2 = 4 + 2 = 6
	// Key "Name" is right-aligned within keyColW: starts at 6 - 4 = 2
	// Value starts at keyColW + 2 = 8

	// Check that "Name" appears starting at x=2
	if s.cells[0][2].Ch != 'N' || s.cells[0][3].Ch != 'a' {
		t.Errorf("expected 'Na' at (2,0), got '%c%c'", s.cells[0][2].Ch, s.cells[0][3].Ch)
	}

	// Check that "Alice" appears starting at x=8
	if s.cells[0][8].Ch != 'A' || s.cells[0][9].Ch != 'l' {
		t.Errorf("expected 'Al' at (8,0), got '%c%c'", s.cells[0][8].Ch, s.cells[0][9].Ch)
	}

	// Check key uses StylePaletteItem
	if s.cells[0][2].Style != term.StylePaletteItem {
		t.Errorf("key should use StylePaletteItem, got %v", s.cells[0][2].Style)
	}

	// Check value uses StyleMuted
	if s.cells[0][8].Style != term.StyleMuted {
		t.Errorf("value should use StyleMuted, got %v", s.cells[0][8].Style)
	}
}

func TestKeyValueListEmptyItems(t *testing.T) {
	kv := NewKeyValueListWidget([]KeyValueEntry{})
	if got := kv.Height(); got != 0 {
		t.Errorf("expected Height()=0 for empty entries, got %d", got)
	}

	// Render should not panic on empty entries
	renderWidget(kv, 0, 0, 20, 5)
}

func TestKeyValueListInvertStyles(t *testing.T) {
	kv := NewKeyValueListWidget([]KeyValueEntry{
		{Key: "K", Value: "V"},
	})
	kv.InvertStyles = true

	s := renderWidget(kv, 0, 0, 20, 3)

	// keyColWidth = len("K") + 2 = 3, key starts at 3 - 1 = 2
	// With InvertStyles: key uses StyleMuted, value uses StylePaletteItem
	if s.cells[0][2].Style != term.StyleMuted {
		t.Errorf("inverted key should use StyleMuted, got %v", s.cells[0][2].Style)
	}

	// Value starts at keyColW + 2 = 5
	if s.cells[0][5].Style != term.StylePaletteItem {
		t.Errorf("inverted value should use StylePaletteItem, got %v", s.cells[0][5].Style)
	}
}

func TestKeyValueListScrollSize(t *testing.T) {
	kv := NewKeyValueListWidget([]KeyValueEntry{
		{Key: "Name", Value: "Alice"},
		{Key: "X", Value: "Y"},
	})
	w, h := kv.ScrollSize()
	if h != 2 {
		t.Errorf("scroll height should be 2, got %d", h)
	}
	// Widest row: len("Name") + 4 + len("Alice") = 4 + 4 + 5 = 13
	if w != 13 {
		t.Errorf("scroll width should be 13, got %d", w)
	}
}

func TestKeyValueListHandleEventIgnored(t *testing.T) {
	kv := NewKeyValueListWidget([]KeyValueEntry{
		{Key: "K", Value: "V"},
	})
	ev := tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone)
	if got := kv.HandleEvent(ev); got != EventIgnored {
		t.Errorf("expected EventIgnored, got %v", got)
	}
}

func TestKeyValueListFullwidthKeys(t *testing.T) {
	kv := NewKeyValueListWidget([]KeyValueEntry{
		{Key: "名前", Value: "値"},
		{Key: "Port", Value: "8080"},
	})

	s := renderWidget(kv, 0, 0, 30, 5)

	// keyColWidth = max(width("名前"), width("Port")) + 2 = 4 + 2 = 6.
	// Both keys are right-aligned to end at column 6, so 名前 occupies
	// columns 2-5 and Port occupies columns 2-5.
	if s.cells[0][2].Ch != '名' || s.cells[0][4].Ch != '前' {
		t.Errorf("名前 not complete/right-aligned: col2=%q col4=%q, want 名 and 前",
			s.cells[0][2].Ch, s.cells[0][4].Ch)
	}
	if s.cells[1][2].Ch != 'P' || s.cells[1][5].Ch != 't' {
		t.Errorf("Port not right-aligned: col2=%q col5=%q, want P and t",
			s.cells[1][2].Ch, s.cells[1][5].Ch)
	}

	// Both values start at the same column (keyColW + 2 = 8).
	if s.cells[0][8].Ch != '値' {
		t.Errorf("value 値 not at col 8, got %q", s.cells[0][8].Ch)
	}
	if s.cells[1][8].Ch != '8' {
		t.Errorf("value 8080 not at col 8, got %q", s.cells[1][8].Ch)
	}

	// rowText reads the row the way the terminal shows it, so a complete
	// 名前 proves neither rune was dropped or clipped.
	if row := rowText(s, 0); !strings.Contains(row, "名前") {
		t.Errorf("row 0 = %q, want it to contain the complete key 名前", row)
	}
}

func TestKeyValueListScrollSizeFullwidth(t *testing.T) {
	kv := NewKeyValueListWidget([]KeyValueEntry{
		{Key: "名前ラベル", Value: "値"},
	})
	// Widest row: width("名前ラベル") + 4 + width("値") = 10 + 4 + 2 = 16.
	if w, _ := kv.ScrollSize(); w != 16 {
		t.Errorf("scroll width = %d, want 16", w)
	}
}

func TestKeyValueListScrollSizeWithWidthsInDifferentRows(t *testing.T) {
	kv := NewKeyValueListWidget([]KeyValueEntry{
		{Key: "名前ラベル", Value: "x"},
		{Key: "Port", Value: "123456789012"},
	})

	w, h := kv.ScrollSize()
	if w != 26 || h != 2 {
		t.Errorf("scroll size = (%d, %d), want (26, 2)", w, h)
	}

	scroll := NewScrollViewWidget(kv)
	scroll.scrollX = 100
	s := renderWidget(scroll, 0, 0, 20, 3)
	if got := rowText(s, 1); !strings.Contains(got, "123456789012") {
		t.Errorf("value at the end of horizontal scrolling = %q, want the complete value", got)
	}
}

func TestKeyValueListWrapsLongValues(t *testing.T) {
	kv := NewWrappingKeyValueListWidget([]KeyValueEntry{
		{Key: "Run binary", Value: "/opt/postgresql/16/bin/psql"},
		{Key: "Editor tab", Value: "yes"},
	})
	// keyColW 12, values start at 14, leaving 10 columns.
	if got := kv.HeightForWidth(24); got != 4 {
		t.Fatalf("HeightForWidth(24) = %d, want 4", got)
	}
	s := renderWidget(kv, 0, 0, 24, 4)
	want := []string{
		"  Run binary  /opt/postg",
		"              resql/16/b",
		"              in/psql",
		"  Editor tab  yes",
	}
	for y, w := range want {
		if got := rowText(s, y); got != w {
			t.Errorf("row %d = %q, want %q", y, got, w)
		}
	}
	if got := kv.Height(); got != 4 {
		t.Errorf("Height after render = %d, want 4", got)
	}
}

func TestKeyValueListWrapFullwidthValue(t *testing.T) {
	kv := NewWrappingKeyValueListWidget([]KeyValueEntry{{Key: "K", Value: "/データ/ベース"}})
	// keyColW 3, values start at 5, leaving 5 columns: a wide rune that would
	// straddle the edge moves to the next row.
	s := renderWidget(kv, 0, 0, 10, 4)
	want := []string{"  K  /デー", "     タ/ベ", "     ース"}
	for y, w := range want {
		if got := rowText(s, y); got != w {
			t.Errorf("row %d = %q, want %q", y, got, w)
		}
	}
}

func TestKeyValueListWithoutWrapClipsLongValues(t *testing.T) {
	kv := NewKeyValueListWidget([]KeyValueEntry{{Key: "Run binary", Value: "/opt/postgresql/16/bin/psql"}})
	s := renderWidget(kv, 0, 0, 24, 3)
	if got := rowText(s, 1); got != "" {
		t.Errorf("row 1 = %q, want the value clipped to one row", got)
	}
	if got := kv.NaturalWidth(); got != 12+2+27 {
		t.Errorf("NaturalWidth = %d, want %d", got, 12+2+27)
	}
}

func TestKeyValueListWrapKeepsTheFullKeyAtNaturalWidth(t *testing.T) {
	kv := NewWrappingKeyValueListWidget([]KeyValueEntry{{Key: "Accept commands from", Value: "hub"}})
	w := kv.NaturalWidth()
	if got := kv.HeightForWidth(w); got != 1 {
		t.Fatalf("HeightForWidth(NaturalWidth) = %d, want 1", got)
	}
	s := renderWidget(kv, 0, 0, w, 2)
	if got, want := rowText(s, 0), "  Accept commands from  hub"; got != want {
		t.Errorf("row 0 = %q, want %q", got, want)
	}
}
