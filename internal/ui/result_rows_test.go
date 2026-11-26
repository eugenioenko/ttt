package ui

import (
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/term"
)

func TestResultRowsFullwidth(t *testing.T) {
	problems := NewProblemsWidget()
	problems.SetItems([]ProblemItem{{File: "測.go", Message: "界a"}})
	references := NewReferencesWidget()
	references.SetItems([]ReferenceItem{{File: "測.go", Text: "界a"}})
	search := NewSearchWidget()
	search.Groups = []SearchFileGroup{{
		Expanded: true,
		Matches:  []SearchMatch{{LineNum: 1, LineText: " \t測a界", ColStart: 5, ColEnd: 6}},
	}}
	search.flatten()

	for _, tc := range []struct {
		name     string
		render   func(Surface)
		row      func() int
		cells    map[int]rune
		clip     int
		edge     int
		matchCol int
	}{
		{"problems", problems.Render, func() int { return 0 }, map[int]rune{3: '測', 5: '.', 11: '界', 13: 'a'}, 12, 11, -1},
		{"references", references.Render, func() int { return 0 }, map[int]rune{1: '測', 3: '.', 11: '界', 13: 'a'}, 13, 11, -1},
		{"search", search.Render, func() int { return search.resultStartY + 1 }, map[int]rune{5: '測', 7: 'a', 8: '界'}, 9, 8, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			grid := makeGrid(24, 10)
			tc.render(NewRenderSurface(grid, Rect{W: 24, H: 10}))
			row := grid[tc.row()]
			for x, want := range tc.cells {
				if got := row[x].Ch; got != want {
					t.Errorf("column %d = %q, want %q", x, got, want)
				}
			}
			if x := tc.matchCol; x >= 0 {
				if row[x-2].Style == term.StyleSearchMatch || row[x].Style != term.StyleSearchMatch || row[x+1].Style == term.StyleSearchMatch {
					t.Errorf("search highlighting is misaligned at columns %d–%d", x-2, x+1)
				}
			}

			tc.render(NewRenderSurface(grid, Rect{W: tc.clip, H: 10}))
			if got := grid[tc.row()][tc.edge].Ch; got != ' ' {
				t.Errorf("clipped column %d = %q, want a space", tc.edge, got)
			}
		})
	}
}

func TestSearchSummariesFullwidth(t *testing.T) {
	catalog, err := i18n.ParseCatalog([]byte(`{
		"search.noResults": "Ā測empty",
		"search.searching": "Ā測searching",
		"search.resultsInFiles": "Ā測found {totalMatches}/{groupsCount}"
	}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	i18n.SetCatalog(catalog)
	t.Cleanup(func() { i18n.SetCatalog(nil) })

	for _, tc := range []struct {
		state string
		want  string
	}{
		{"empty", "Ā測 empty"},
		{"searching", "Ā測 searching"},
		{"found", "Ā測 found 1/1"},
	} {
		t.Run(tc.state, func(t *testing.T) {
			search := NewSearchWidget()
			search.Input.Config.OnChange = nil
			search.Input.SetText("query")
			switch tc.state {
			case "searching":
				search.Searching = true
			case "found":
				search.Groups = []SearchFileGroup{{Matches: []SearchMatch{{LineNum: 1}}}}
				search.flatten()
			}
			grid := makeGrid(24, 10)
			search.Render(NewRenderSurface(grid, Rect{W: 24, H: 10}))
			row := grid[2]
			cells := make([]rune, len(row))
			for x, cell := range row {
				cells[x] = cell.Ch
			}
			if got := strings.TrimRight(string(cells), " "); got != tc.want {
				t.Errorf("summary row = %q, want %q", got, tc.want)
			}
		})
	}
}
