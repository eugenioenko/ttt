package ui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func completionLabels(items []CompletionItem) []string {
	labels := make([]string, len(items))
	for i, it := range items {
		labels[i] = it.Label
	}
	return labels
}

func itemsWithSortText(sortText string, labels ...string) []CompletionItem {
	items := make([]CompletionItem, len(labels))
	for i, l := range labels {
		items[i] = CompletionItem{Label: l, SortText: sortText}
	}
	return items
}

func TestFilterCompletionsOrder(t *testing.T) {
	tests := []struct {
		name   string
		items  []CompletionItem
		prefix string
		want   []string
	}{
		{
			name:   "same case prefix wins a sortText tie",
			items:  itemsWithSortText("15", "StringIterator", "String", "StructuredSerializeOptions", "StreamPipeOptions", "StringConstructor", "string"),
			prefix: "str",
			want:   []string{"string", "String", "StringIterator", "StreamPipeOptions", "StringConstructor", "StructuredSerializeOptions"},
		},
		{
			name:   "lowercase keywords before types",
			items:  itemsWithSortText("15", "NodeIterator", "number", "never", "null", "NonNullable"),
			prefix: "n",
			want:   []string{"null", "never", "number", "NonNullable", "NodeIterator"},
		},
		{
			name: "lower sortText beats a case match",
			items: []CompletionItem{
				{Label: "string", SortText: "15"},
				{Label: "Str", SortText: "10"},
			},
			prefix: "str",
			want:   []string{"Str", "string"},
		},
		{
			name:   "empty sortText uses the same tie-breaks",
			items:  itemsWithSortText("", "StringIterator", "String", "const", "string", "console"),
			prefix: "",
			want:   []string{"console", "const", "String", "string", "StringIterator"},
		},
		{
			name:   "empty sortText with a typed prefix",
			items:  itemsWithSortText("", "StringIterator", "String", "string"),
			prefix: "str",
			want:   []string{"string", "String", "StringIterator"},
		},
		{
			name: "empty prefix sorts everything by sortText then label",
			items: []CompletionItem{
				{Label: "toString", SortText: "11"},
				{Label: "length", SortText: "11"},
				{Label: "at", SortText: "11"},
				{Label: "local", SortText: "10"},
			},
			prefix: "",
			want:   []string{"local", "at", "length", "toString"},
		},
		{
			name: "filterText drives the case check",
			items: []CompletionItem{
				{Label: "aaa", FilterText: "String", SortText: "15"},
				{Label: "bbb", FilterText: "string", SortText: "15"},
			},
			prefix: "str",
			want:   []string{"bbb", "aaa"},
		},
		{
			name:   "non matching items are still dropped",
			items:  itemsWithSortText("15", "string", "number"),
			prefix: "str",
			want:   []string{"string"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := completionLabels(FilterCompletions(tt.items, tt.prefix))
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterCompletionsDoesNotReorderInput(t *testing.T) {
	items := itemsWithSortText("15", "b", "a")
	FilterCompletions(items, "")
	if got := completionLabels(items); !reflect.DeepEqual(got, []string{"b", "a"}) {
		t.Fatalf("input reordered: %v", got)
	}
}

func TestAutocompleteDetailFullwidthLayout(t *testing.T) {
	t.Run("detail ending in fullwidth glyph fits inside right border", func(t *testing.T) {
		w := NewAutocompleteWidget([]CompletionItem{{Label: "print", Detail: "abc界"}}, 0, 0)
		if got := w.menuWidth(); got != 17 {
			t.Fatalf("menuWidth() = %d, want 17", got)
		}

		cells := makeGrid(40, 10)
		w.Render(NewRenderSurface(cells, Rect{W: 40, H: 10}))

		row := cells[2]
		want := map[int]rune{
			11: 'a',
			12: 'b',
			13: 'c',
			14: '界',
			15: ' ',
			16: '│',
		}
		for col, wantCh := range want {
			if row[col].Ch != wantCh {
				t.Errorf("col %d = %q, want %q", col, row[col].Ch, wantCh)
			}
		}
	})

	t.Run("consecutive fullwidth glyphs advance by display width", func(t *testing.T) {
		w := NewAutocompleteWidget([]CompletionItem{{Label: "print", Detail: "函数fn"}}, 0, 0)
		if got := w.menuWidth(); got != 18 {
			t.Fatalf("menuWidth() = %d, want 18", got)
		}

		cells := makeGrid(40, 10)
		w.Render(NewRenderSurface(cells, Rect{W: 40, H: 10}))

		row := cells[2]
		want := map[int]rune{
			11: '函',
			12: ' ',
			13: '数',
			14: ' ',
			15: 'f',
			16: 'n',
			17: '│',
		}
		for col, wantCh := range want {
			if row[col].Ch != wantCh {
				t.Errorf("col %d = %q, want %q", col, row[col].Ch, wantCh)
			}
		}
	})

	t.Run("wide label positions detail after display width", func(t *testing.T) {
		w := NewAutocompleteWidget([]CompletionItem{{Label: "函数名", Detail: "int"}}, 0, 0)
		if got := w.menuWidth(); got != 16 {
			t.Fatalf("menuWidth() = %d, want 16", got)
		}

		cells := makeGrid(40, 10)
		w.Render(NewRenderSurface(cells, Rect{W: 40, H: 10}))

		row := cells[2]
		want := map[int]rune{
			4:  '函',
			6:  '数',
			8:  '名',
			10: ' ',
			11: ' ',
			12: 'i',
			13: 'n',
			14: 't',
			15: '│',
		}
		for col, wantCh := range want {
			if row[col].Ch != wantCh {
				t.Errorf("col %d = %q, want %q", col, row[col].Ch, wantCh)
			}
		}
	})

	t.Run("long wide label prevents detail from overwriting label", func(t *testing.T) {
		// 27 wide runes fill cols 4..57 inside the 60-col popup, leaving no room for detail.
		w := NewAutocompleteWidget([]CompletionItem{{Label: strings.Repeat("界", 27), Detail: "xyz"}}, 0, 0)

		cells := makeGrid(70, 10)
		w.Render(NewRenderSurface(cells, Rect{W: 70, H: 10}))

		row := cells[2]
		if row[56].Ch != '界' {
			t.Errorf("col 56 = %q, want '界'", row[56].Ch)
		}
		if row[58].Ch != ' ' {
			t.Errorf("col 58 = %q, want ' '", row[58].Ch)
		}
		if row[59].Ch != '│' {
			t.Errorf("col 59 = %q, want '│'", row[59].Ch)
		}
	})

	t.Run("fullwidth detail rune at clip edge does not bleed onto right border", func(t *testing.T) {
		// 52-col label leaves cols 57..58 before the border at col 59; '界' would straddle 58..59.
		w := NewAutocompleteWidget([]CompletionItem{{Label: strings.Repeat("a", 52), Detail: "x界"}}, 0, 0)

		cells := makeGrid(80, 10)
		w.Render(NewRenderSurface(cells, Rect{W: 80, H: 10}))

		row := cells[2]
		if row[57].Ch != 'x' {
			t.Errorf("col 57 = %q, want 'x'", row[57].Ch)
		}
		if row[58].Ch != ' ' {
			t.Errorf("col 58 = %q, want ' '", row[58].Ch)
		}
		if row[58].Style != term.StylePaletteSelected {
			t.Errorf("col 58 Style = %v, want StylePaletteSelected", row[58].Style)
		}
		if row[59].Ch != '│' {
			t.Errorf("col 59 = %q, want '│'", row[59].Ch)
		}
	})

	t.Run("fullwidth detail rune at clip edge does not bleed onto scrollbar", func(t *testing.T) {
		items := make([]CompletionItem, defaultMaxVisible+2)
		items[0] = CompletionItem{Label: strings.Repeat("a", 52), Detail: "x界"}
		for i := 1; i < len(items); i++ {
			items[i] = CompletionItem{Label: "item"}
		}
		w := NewAutocompleteWidget(items, 0, 0)

		cells := makeGrid(80, 20)
		w.Render(NewRenderSurface(cells, Rect{W: 80, H: 20}))

		// Scrollbar sits at col 59, right border at col 60.
		row := cells[2]
		if row[57].Ch != 'x' {
			t.Errorf("col 57 = %q, want 'x'", row[57].Ch)
		}
		if row[58].Ch != ' ' {
			t.Errorf("col 58 = %q, want ' '", row[58].Ch)
		}
		if row[60].Ch != '│' {
			t.Errorf("col 60 = %q, want '│'", row[60].Ch)
		}
	})

	t.Run("fullwidth detail rune does not bleed past narrow surface edge", func(t *testing.T) {
		w := NewAutocompleteWidget([]CompletionItem{{Label: "print", Detail: "abc界"}}, 0, 0)

		cells := makeGrid(15, 10)
		w.Render(NewRenderSurface(cells, Rect{W: 15, H: 10}))

		row := cells[2]
		want := map[int]rune{
			11: 'a',
			12: 'b',
			13: 'c',
			14: ' ',
		}
		for col, wantCh := range want {
			if row[col].Ch != wantCh {
				t.Errorf("col %d = %q, want %q", col, row[col].Ch, wantCh)
			}
		}
	})

	t.Run("ascii label and detail layout stays unchanged", func(t *testing.T) {
		w := NewAutocompleteWidget([]CompletionItem{{Label: "printf", Detail: "func(string)"}}, 0, 0)
		if got := w.menuWidth(); got != 25 {
			t.Fatalf("menuWidth() = %d, want 25", got)
		}

		cells := makeGrid(40, 10)
		w.Render(NewRenderSurface(cells, Rect{W: 40, H: 10}))

		row := cells[2]
		for i, ch := range "func(string)" {
			if row[12+i].Ch != ch {
				t.Errorf("col %d = %q, want %q", 12+i, row[12+i].Ch, ch)
			}
		}
		if row[24].Ch != '│' {
			t.Errorf("col 24 = %q, want '│'", row[24].Ch)
		}
	})
}
