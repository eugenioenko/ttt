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

func TestRemoveAndToggle(t *testing.T) {
	mc := New(1, 0)
	mc.Add(3, 0)
	mc.Add(5, 0)
	if len(mc.Cursors) != 3 {
		t.Fatalf("expected 3 cursors, got %d", len(mc.Cursors))
	}

	// Remove middle cursor (3, 0)
	if !mc.Remove(3, 0) {
		t.Fatal("expected Remove(3, 0) to succeed")
	}
	if len(mc.Cursors) != 2 {
		t.Fatalf("expected 2 cursors, got %d", len(mc.Cursors))
	}
	if mc.Cursors[0].Line != 1 || mc.Cursors[1].Line != 5 {
		t.Errorf("unexpected cursors after remove: %+v", mc.Cursors)
	}

	// Remove cursor not in set
	if mc.Remove(99, 0) {
		t.Fatal("expected Remove(99, 0) to return false")
	}

	// Toggle existing cursor (5, 0) -> should remove it
	if !mc.Toggle(5, 0) {
		t.Fatal("expected Toggle(5, 0) to return true")
	}
	if len(mc.Cursors) != 1 {
		t.Fatalf("expected 1 cursor, got %d", len(mc.Cursors))
	}
	if mc.Cursors[0].Line != 1 {
		t.Errorf("expected sole remaining cursor at line 1, got %d", mc.Cursors[0].Line)
	}

	// Toggle sole remaining cursor -> should leave it in place
	if mc.Toggle(1, 0) {
		t.Fatal("expected Toggle on sole cursor to return false")
	}
	if len(mc.Cursors) != 1 {
		t.Fatalf("expected 1 cursor to remain, got %d", len(mc.Cursors))
	}

	// Toggle non-existing cursor (2, 4) -> should add it
	if !mc.Toggle(2, 4) {
		t.Fatal("expected Toggle on new position to return true")
	}
	if len(mc.Cursors) != 2 {
		t.Fatalf("expected 2 cursors, got %d", len(mc.Cursors))
	}
	if mc.Cursors[1].Line != 2 || mc.Cursors[1].Col != 4 {
		t.Errorf("expected added cursor at (2, 4), got (%d, %d)", mc.Cursors[1].Line, mc.Cursors[1].Col)
	}
}

func TestRemoveCursorHistoryOrder(t *testing.T) {
	mc := New(0, 0)
	mc.Add(1, 0)
	mc.Add(2, 0)
	mc.Add(3, 0)
	if len(mc.Cursors) != 4 {
		t.Fatalf("expected 4 cursors, got %d", len(mc.Cursors))
	}

	// Remove cursor (2, 0) out of order
	if !mc.Remove(2, 0) {
		t.Fatal("expected Remove(2, 0) to succeed")
	}

	// RemoveLast should pop in reverse creation order, skipping the removed (2, 0)
	last, ok := mc.RemoveLast()
	if !ok || last.Line != 3 || last.Col != 0 {
		t.Fatalf("expected RemoveLast to return (3, 0), got %+v, ok=%v", last, ok)
	}

	last, ok = mc.RemoveLast()
	if !ok || last.Line != 1 || last.Col != 0 {
		t.Fatalf("expected RemoveLast to return (1, 0), got %+v, ok=%v", last, ok)
	}

	// Only the root cursor (0, 0) should remain
	if len(mc.Cursors) != 1 || mc.Cursors[0].Line != 0 || mc.Cursors[0].Col != 0 {
		t.Fatalf("expected sole cursor at (0, 0), got %+v", mc.Cursors)
	}
}

func TestRemovePrimaryUpdatesPrimaryIndex(t *testing.T) {
	mc := New(1, 0)
	mc.Add(3, 0)
	mc.Add(5, 0)
	// Primary is initially index 0 ((1, 0))
	if mc.Primary != 0 {
		t.Fatalf("expected primary 0, got %d", mc.Primary)
	}
	// Remove primary cursor
	if !mc.Remove(1, 0) {
		t.Fatal("expected Remove to succeed")
	}
	if mc.Primary != 0 {
		t.Fatalf("expected primary 0 after removing index 0, got %d", mc.Primary)
	}
	if mc.PrimaryCursor().Line != 3 {
		t.Fatalf("expected new primary at line 3, got %d", mc.PrimaryCursor().Line)
	}

	// Now set primary to last cursor (index 1, line 5)
	mc.Primary = 1
	if !mc.Remove(5, 0) {
		t.Fatal("expected Remove to succeed")
	}
	if mc.Primary != 0 {
		t.Fatalf("expected primary 0 after removing last cursor, got %d", mc.Primary)
	}
	if mc.PrimaryCursor().Line != 3 {
		t.Fatalf("expected primary at line 3, got %d", mc.PrimaryCursor().Line)
	}
}

func TestRemoveLastAfterCursorMovementOrToggle(t *testing.T) {
	mc := New(0, 0)
	mc.Add(2, 0)
	mc.Add(4, 0)

	// Remove middle cursor via Toggle
	if !mc.Toggle(2, 0) {
		t.Fatal("expected Toggle(2, 0) to remove cursor")
	}
	if len(mc.Cursors) != 2 {
		t.Fatalf("expected 2 cursors, got %d", len(mc.Cursors))
	}

	// RemoveLast should remove cursor at (4, 0)
	removed, ok := mc.RemoveLast()
	if !ok || removed.Line != 4 {
		t.Fatalf("expected RemoveLast to remove line 4, got line %d ok=%v", removed.Line, ok)
	}
	if len(mc.Cursors) != 1 {
		t.Fatalf("expected 1 cursor remaining, got %d", len(mc.Cursors))
	}
}
