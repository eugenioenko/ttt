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

// A fullwidth key covers two columns, so measuring the key column in runes
// misaligns it and cuts the key off (#1067).
func TestKeyValueListRendersFullwidthKeys(t *testing.T) {
	kv := NewKeyValueListWidget([]KeyValueEntry{
		{Key: "名前", Value: "値"},
		{Key: "Port", Value: "8080"},
	})

	s := renderWidget(kv, 0, 0, 20, 2)

	// The key column is four columns wide for both keys, so both end at
	// column 6 and both values start at column 8.
	if got := rowText(s, 0); !strings.Contains(got, "名前") {
		t.Errorf("row 0 = %q, want it to hold the whole key", got)
	}
	if got := s.cells[0][4].Ch; got != '前' {
		t.Errorf("row 0 cell 4 = %q, want the second fullwidth rune", got)
	}
	if got := s.cells[0][8].Ch; got != '値' {
		t.Errorf("row 0 cell 8 = %q, want the value in the same column as row 1", got)
	}
	if got := s.cells[1][8].Ch; got != '8' {
		t.Errorf("row 1 cell 8 = %q, want the value in the same column as row 0", got)
	}
}
