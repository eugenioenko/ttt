package ui

import "testing"

func TestDiffTextSelectionContainsBoundaries(t *testing.T) {
	t.Run("single line is half open", func(t *testing.T) {
		selection := diffTextSelection{
			Anchor:  diffSelPos{Line: 3, Col: 2},
			Current: diffSelPos{Line: 3, Col: 5},
		}
		assertDiffSelectionContains(t, selection,
			[]diffSelPos{{Line: 3, Col: 2}, {Line: 3, Col: 3}, {Line: 3, Col: 4}},
			[]diffSelPos{{Line: 3, Col: 1}, {Line: 3, Col: 5}},
		)
	})

	t.Run("single character contains only that cell", func(t *testing.T) {
		selection := diffTextSelection{
			Anchor:  diffSelPos{Line: 1, Col: 4},
			Current: diffSelPos{Line: 1, Col: 5},
		}
		assertDiffSelectionContains(t, selection,
			[]diffSelPos{{Line: 1, Col: 4}},
			[]diffSelPos{{Line: 1, Col: 3}, {Line: 1, Col: 5}},
		)
	})

	t.Run("multiple lines use partial edges and full middle rows", func(t *testing.T) {
		selection := diffTextSelection{
			Anchor:  diffSelPos{Line: 2, Col: 3},
			Current: diffSelPos{Line: 4, Col: 2},
		}
		assertDiffSelectionContains(t, selection,
			[]diffSelPos{
				{Line: 2, Col: 3}, {Line: 2, Col: 8},
				{Line: 3, Col: 0}, {Line: 3, Col: 20},
				{Line: 4, Col: 0}, {Line: 4, Col: 1},
			},
			[]diffSelPos{{Line: 2, Col: 2}, {Line: 4, Col: 2}, {Line: 5, Col: 0}},
		)
	})

	t.Run("reversed selection normalizes through range", func(t *testing.T) {
		selection := diffTextSelection{
			Anchor:  diffSelPos{Line: 4, Col: 2},
			Current: diffSelPos{Line: 2, Col: 3},
		}
		start, end := selection.Range()
		if start != (diffSelPos{Line: 2, Col: 3}) || end != (diffSelPos{Line: 4, Col: 2}) {
			t.Fatalf("range = %v..%v, want 2:3..4:2", start, end)
		}
		assertDiffSelectionContains(t, selection,
			[]diffSelPos{{Line: 2, Col: 3}, {Line: 3, Col: 0}, {Line: 4, Col: 1}},
			[]diffSelPos{{Line: 2, Col: 2}, {Line: 4, Col: 2}},
		)
	})
}

func assertDiffSelectionContains(t *testing.T, selection diffTextSelection, included, excluded []diffSelPos) {
	t.Helper()
	for _, point := range included {
		if !selection.Contains(point.Line, point.Col) {
			t.Errorf("selection %v..%v should contain %+v", selection.Anchor, selection.Current, point)
		}
	}
	for _, point := range excluded {
		if selection.Contains(point.Line, point.Col) {
			t.Errorf("selection %v..%v should not contain %+v", selection.Anchor, selection.Current, point)
		}
	}
}

func TestDiffTextSelectionSelectWordBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name  string
		col   int
		start int
		end   int
	}{
		{name: "first character", col: 0, start: 0, end: 5},
		{name: "last character", col: 4, start: 0, end: 5},
		{name: "whitespace", col: 5, start: 5, end: 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var selection diffTextSelection
			if !selection.SelectWord(7, tc.col, "alpha beta") {
				t.Fatal("SelectWord rejected non-empty text")
			}
			start, end := selection.Range()
			if start != (diffSelPos{Line: 7, Col: tc.start}) || end != (diffSelPos{Line: 7, Col: tc.end}) {
				t.Fatalf("word range = %v..%v, want 7:%d..7:%d", start, end, tc.start, tc.end)
			}
		})
	}
}

func TestDiffTextSelectionMechanics(t *testing.T) {
	selection := diffTextSelection{
		Anchor:  diffSelPos{Line: 2, Col: 3},
		Current: diffSelPos{Line: 0, Col: 1},
	}
	start, end := selection.Range()
	if start != (diffSelPos{Line: 0, Col: 1}) || end != (diffSelPos{Line: 2, Col: 3}) {
		t.Fatalf("range = %v..%v, want 0:1..2:3", start, end)
	}
	for _, point := range []diffSelPos{{Line: 0, Col: 1}, {Line: 1, Col: 0}, {Line: 2, Col: 2}} {
		if !selection.Contains(point.Line, point.Col) {
			t.Errorf("selection should contain %+v", point)
		}
	}
	for _, point := range []diffSelPos{{Line: 0, Col: 0}, {Line: 2, Col: 3}, {Line: 3, Col: 0}} {
		if selection.Contains(point.Line, point.Col) {
			t.Errorf("selection should not contain %+v", point)
		}
	}

	lines := []struct {
		text       string
		selectable bool
	}{{"zero", true}, {"hidden", false}, {"twofold", true}}
	if got := selection.Text(len(lines), func(line int) (string, bool) {
		return lines[line].text, lines[line].selectable
	}); got != "ero\ntwo" {
		t.Fatalf("text = %q, want only selected source text", got)
	}

	if !selection.SelectWord(4, 5, "alpha_beta + gamma") {
		t.Fatal("word selection rejected non-empty text")
	}
	start, end = selection.Range()
	if start != (diffSelPos{Line: 4, Col: 0}) || end != (diffSelPos{Line: 4, Col: 10}) {
		t.Fatalf("word range = %v..%v, want 4:0..4:10", start, end)
	}
}
