package widgets

import "testing"

func TestTreeEmptyTextUsesDisplayWidth(t *testing.T) {
	for _, tc := range []struct {
		name  string
		text  string
		width int
		want  map[int]rune
	}{
		{"wide glyph advances two columns", "界a", 8, map[int]rune{1: '界', 3: 'a'}},
		{"ascii layout is unchanged", "None", 8, map[int]rune{1: 'N', 2: 'o', 3: 'n', 4: 'e'}},
		{"wide glyph straddling the right edge becomes a space", "界a", 2, map[int]rune{1: ' '}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := NewTreeWidget(TreeConfig{EmptyText: tc.text})
			surface := renderWidget(tree, 0, 0, tc.width, 1)
			for x, want := range tc.want {
				if got := surface.cells[0][x].Ch; got != want {
					t.Errorf("column %d = %q, want %q", x, got, want)
				}
			}
			for x, cell := range surface.cells[0] {
				if x >= tc.width && cell.Ch != 0 && cell.Ch != ' ' {
					t.Errorf("column %d painted %q beyond the widget width", x, cell.Ch)
				}
			}
		})
	}
}
