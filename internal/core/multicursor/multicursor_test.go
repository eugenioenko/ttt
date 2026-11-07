package multicursor

import (
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/selection"
)

func TestMergeSelections(t *testing.T) {
	selected := func(anchorLine, anchorCol, line, col int) CursorState {
		return CursorState{Line: line, Col: col, Sel: selection.Selection{
			Active: true, Anchor: selection.Position{Line: anchorLine, Col: anchorCol},
		}}
	}
	for _, tt := range []struct {
		name        string
		cursors     []CursorState
		primary     int
		want        []CursorState
		wantPrimary int
	}{
		{
			name: "transitive overlap",
			cursors: []CursorState{
				selected(0, 0, 0, 2), selected(0, 1, 0, 3), selected(0, 2, 0, 4), selected(0, 6, 0, 7),
			},
			primary: 2,
			want:    []CursorState{selected(0, 0, 0, 4), selected(0, 6, 0, 7)},
		},
		{
			name:    "nested reversed primary",
			cursors: []CursorState{selected(0, 0, 0, 6), selected(0, 5, 0, 1), selected(0, 8, 0, 9)},
			primary: 1,
			want:    []CursorState{selected(0, 6, 0, 0), selected(0, 8, 0, 9)},
		},
		{
			name: "covered cursors",
			cursors: []CursorState{
				selected(0, 1, 0, 5), {Line: 0, Col: 0}, {Line: 0, Col: 3}, {Line: 0, Col: 5}, {Line: 0, Col: 8},
			},
			primary:     3,
			want:        []CursorState{{Line: 0, Col: 0}, selected(0, 1, 0, 5), {Line: 0, Col: 8}},
			wantPrimary: 1,
		},
		{
			name:        "adjacent selections stay separate",
			cursors:     []CursorState{selected(0, 0, 0, 2), selected(0, 2, 0, 4)},
			primary:     1,
			want:        []CursorState{selected(0, 0, 0, 2), selected(0, 2, 0, 4)},
			wantPrimary: 1,
		},
		{
			name:    "multiline overlap",
			cursors: []CursorState{selected(2, 2, 0, 3), selected(1, 0, 3, 1), {Line: 1, Col: 3}},
			want:    []CursorState{selected(3, 1, 0, 3)},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mc := &MultiCursor{Cursors: tt.cursors, Primary: tt.primary}
			mc.MergeSelections()
			if !reflect.DeepEqual(mc.Cursors, tt.want) || mc.Primary != tt.wantPrimary {
				t.Fatalf("merged = %+v, primary %d; want %+v, primary %d", mc.Cursors, mc.Primary, tt.want, tt.wantPrimary)
			}
			mc.MergeSelections()
			if !reflect.DeepEqual(mc.Cursors, tt.want) || mc.Primary != tt.wantPrimary {
				t.Fatalf("normalizing again changed cursors: %+v, primary %d", mc.Cursors, mc.Primary)
			}
		})
	}
}

func TestNewMultiCursor(t *testing.T) {
	mc := New(5, 10)
	if len(mc.Cursors) != 1 {
		t.Fatalf("expected 1 cursor, got %d", len(mc.Cursors))
	}
	if mc.Cursors[0].Line != 5 || mc.Cursors[0].Col != 10 {
		t.Errorf("expected (5,10), got (%d,%d)", mc.Cursors[0].Line, mc.Cursors[0].Col)
	}
	if mc.IsMulti() {
		t.Error("single cursor should not be multi")
	}
}

func TestAddAndSort(t *testing.T) {
	mc := New(5, 10)
	mc.Add(2, 3)
	mc.Add(5, 5)
	if len(mc.Cursors) != 3 {
		t.Fatalf("expected 3 cursors, got %d", len(mc.Cursors))
	}
	if !mc.IsMulti() {
		t.Error("should be multi with 3 cursors")
	}
	if mc.Cursors[0].Line != 2 || mc.Cursors[0].Col != 3 {
		t.Errorf("first cursor should be (2,3), got (%d,%d)", mc.Cursors[0].Line, mc.Cursors[0].Col)
	}
	if mc.Cursors[1].Line != 5 || mc.Cursors[1].Col != 5 {
		t.Errorf("second cursor should be (5,5), got (%d,%d)", mc.Cursors[1].Line, mc.Cursors[1].Col)
	}
	if mc.Cursors[2].Line != 5 || mc.Cursors[2].Col != 10 {
		t.Errorf("third cursor should be (5,10), got (%d,%d)", mc.Cursors[2].Line, mc.Cursors[2].Col)
	}
	p := mc.PrimaryCursor()
	if p.Line != 5 || p.Col != 10 {
		t.Errorf("primary should remain (5,10), got (%d,%d)", p.Line, p.Col)
	}
}

func TestAddDuplicate(t *testing.T) {
	mc := New(1, 1)
	mc.Add(1, 1)
	if len(mc.Cursors) != 1 {
		t.Errorf("duplicate should not be added, got %d cursors", len(mc.Cursors))
	}
}

func TestRemoveLast(t *testing.T) {
	mc := New(1, 0)
	mc.Add(3, 0)
	mc.Add(5, 0)
	removed, ok := mc.RemoveLast()
	if !ok {
		t.Fatal("RemoveLast should succeed")
	}
	if removed.Line != 5 {
		t.Errorf("expected removed line 5, got %d", removed.Line)
	}
	if len(mc.Cursors) != 2 {
		t.Errorf("expected 2 cursors, got %d", len(mc.Cursors))
	}
	removed, ok = mc.RemoveLast()
	if !ok || removed.Line != 3 {
		t.Errorf("expected removed line 3, got line %d ok=%v", removed.Line, ok)
	}
	_, ok = mc.RemoveLast()
	if ok {
		t.Error("should not remove last remaining cursor")
	}
}

func TestCollapseToSingle(t *testing.T) {
	mc := New(1, 0)
	mc.Add(3, 5)
	mc.Add(7, 2)
	mc.CollapseToSingle()
	if len(mc.Cursors) != 1 {
		t.Fatalf("expected 1 cursor, got %d", len(mc.Cursors))
	}
	if mc.IsMulti() {
		t.Error("should not be multi after collapse")
	}
}

func TestDeduplicate(t *testing.T) {
	mc := New(1, 0)
	mc.Cursors = append(mc.Cursors, CursorState{Line: 1, Col: 0})
	mc.Cursors = append(mc.Cursors, CursorState{Line: 2, Col: 0})
	mc.Deduplicate()
	if len(mc.Cursors) != 2 {
		t.Errorf("expected 2 cursors after dedup, got %d", len(mc.Cursors))
	}
}

func TestAddWithSelection(t *testing.T) {
	mc := New(0, 0)
	sel := selection.Selection{Active: true, Anchor: selection.Position{Line: 1, Col: 0}}
	mc.AddWithSelection(1, 5, sel)
	if len(mc.Cursors) != 2 {
		t.Fatalf("expected 2 cursors, got %d", len(mc.Cursors))
	}
	if !mc.Cursors[1].Sel.Active {
		t.Error("second cursor should have active selection")
	}
}
