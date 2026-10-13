package fold

import (
	"reflect"
	"testing"
)

func TestSetRanges(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{
		{StartLine: 5, EndLine: 10},
		{StartLine: 1, EndLine: 3},
	})
	// Verify ranges are sorted by checking folds at expected lines
	if r := s.FoldAt(1); r == nil || r.EndLine != 3 {
		t.Error("expected fold at line 1 with EndLine 3")
	}
	if r := s.FoldAt(5); r == nil || r.EndLine != 10 {
		t.Error("expected fold at line 5 with EndLine 10")
	}
}

func TestToggle(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{{StartLine: 2, EndLine: 5}})

	s.Toggle(2)
	if !s.IsCollapsed(2) {
		t.Error("expected collapsed after toggle")
	}

	s.Toggle(2)
	if s.IsCollapsed(2) {
		t.Error("expected expanded after second toggle")
	}
}

func TestToggleNonFoldLine(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{{StartLine: 2, EndLine: 5}})
	s.Toggle(7)
	if s.HasCollapsedFolds() {
		t.Error("toggling non-fold line should be no-op")
	}
}

func TestToggleInsideFold(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{{StartLine: 2, EndLine: 5}})
	s.Toggle(2)

	s.Toggle(4)
	if s.IsCollapsed(2) {
		t.Error("toggling inside collapsed fold should expand it")
	}
}

func TestExpand(t *testing.T) {
	ranges := []Range{{StartLine: 1, EndLine: 8}, {StartLine: 3, EndLine: 5}}
	for _, tc := range []struct {
		name      string
		line      int
		collapsed []int
		want      []int
	}{
		{name: "first line", line: 1, collapsed: []int{1}},
		{name: "body", line: 7, collapsed: []int{1}},
		{name: "last line", line: 8, collapsed: []int{1}},
		{name: "nested first line", line: 3, collapsed: []int{1, 3}, want: []int{1}},
		{name: "nested body", line: 4, collapsed: []int{1, 3}, want: []int{1}},
		{name: "open inner fold in collapsed outer", line: 4, collapsed: []int{1}},
		{name: "already open", line: 4},
		{name: "before folds", line: 0, collapsed: []int{1, 3}, want: []int{1, 3}},
		{name: "after folds", line: 9, collapsed: []int{1, 3}, want: []int{1, 3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewState()
			s.SetRanges(ranges)
			for _, line := range tc.collapsed {
				s.Toggle(line)
			}
			s.VisibleLines(10)
			s.Expand(tc.line)
			want := NewState()
			want.SetRanges(ranges)
			for _, line := range tc.want {
				want.Toggle(line)
			}
			if !reflect.DeepEqual(s.collapsed, want.collapsed) {
				t.Fatalf("collapsed folds = %v, want %v", s.collapsed, want.collapsed)
			}
			if got := s.VisibleLines(10); !reflect.DeepEqual(got, want.VisibleLines(10)) {
				t.Fatalf("visible lines = %v, want %v", got, want.VisibleLines(10))
			}
		})
	}
}

func TestCollapse(t *testing.T) {
	ranges := []Range{{StartLine: 1, EndLine: 8}, {StartLine: 3, EndLine: 5}}
	for _, tc := range []struct {
		name      string
		line      int
		collapsed []int
		want      []int
	}{
		{name: "first line", line: 1, want: []int{1}},
		{name: "body", line: 7, want: []int{1}},
		{name: "last line", line: 8, want: []int{1}},
		{name: "nested first line", line: 3, want: []int{3}},
		{name: "nested body", line: 4, want: []int{3}},
		{name: "nested last line", line: 5, want: []int{3}},
		{name: "already closed", line: 4, collapsed: []int{3}, want: []int{3}},
		{name: "collapsed outer", line: 4, collapsed: []int{1}, want: []int{1, 3}},
		{name: "before folds", line: 0},
		{name: "after folds", line: 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewState()
			s.SetRanges(ranges)
			for _, line := range tc.collapsed {
				s.Toggle(line)
			}
			s.VisibleLines(10)
			s.Collapse(tc.line)
			want := NewState()
			want.SetRanges(ranges)
			for _, line := range tc.want {
				want.Toggle(line)
			}
			if !reflect.DeepEqual(s.collapsed, want.collapsed) {
				t.Fatalf("collapsed folds = %v, want %v", s.collapsed, want.collapsed)
			}
			if got := s.VisibleLines(10); !reflect.DeepEqual(got, want.VisibleLines(10)) {
				t.Fatalf("visible lines = %v, want %v", got, want.VisibleLines(10))
			}
			s.Collapse(tc.line)
			if !reflect.DeepEqual(s.collapsed, want.collapsed) {
				t.Fatalf("repeated close changed collapsed folds to %v", s.collapsed)
			}
		})
	}
}

func TestCollapseAll(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{
		{StartLine: 1, EndLine: 3},
		{StartLine: 5, EndLine: 8},
	})
	s.CollapseAll()
	if !s.IsCollapsed(1) || !s.IsCollapsed(5) {
		t.Error("expected all ranges collapsed")
	}
}

func TestExpandAll(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{
		{StartLine: 1, EndLine: 3},
		{StartLine: 5, EndLine: 8},
	})
	s.CollapseAll()
	s.ExpandAll()
	if s.HasCollapsedFolds() {
		t.Error("expected no collapsed folds after ExpandAll")
	}
}

func TestNestedRanges(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{
		{StartLine: 0, EndLine: 9},
		{StartLine: 2, EndLine: 4},
	})
	s.Toggle(0)

	vis := s.VisibleLines(10)
	if len(vis) != 1 {
		t.Errorf("expected 1 visible line (outer fold collapsed), got %d: %v", len(vis), vis)
	}
	if vis[0] != 0 {
		t.Errorf("expected line 0 visible, got %v", vis)
	}
}

func TestVisibleLines_NoFolds(t *testing.T) {
	s := NewState()
	vis := s.VisibleLines(5)
	expected := []int{0, 1, 2, 3, 4}
	if !reflect.DeepEqual(vis, expected) {
		t.Errorf("expected %v, got %v", expected, vis)
	}
}

func TestVisibleLines_OneFold(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{{StartLine: 2, EndLine: 4}})
	s.Toggle(2)

	vis := s.VisibleLines(7)
	expected := []int{0, 1, 2, 5, 6}
	if !reflect.DeepEqual(vis, expected) {
		t.Errorf("expected %v, got %v", expected, vis)
	}
}

func TestVisibleLines_FoldStartVisible(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{{StartLine: 2, EndLine: 4}})
	s.Toggle(2)

	vis := s.VisibleLines(7)
	found := false
	for _, v := range vis {
		if v == 2 {
			found = true
		}
	}
	if !found {
		t.Error("fold start line should always be visible")
	}
}

func TestVisibleLines_MultipleFolds(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{
		{StartLine: 1, EndLine: 2},
		{StartLine: 5, EndLine: 7},
	})
	s.CollapseAll()

	vis := s.VisibleLines(10)
	expected := []int{0, 1, 3, 4, 5, 8, 9}
	if !reflect.DeepEqual(vis, expected) {
		t.Errorf("expected %v, got %v", expected, vis)
	}
}

func TestVisibleLines_NestedFolds(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{
		{StartLine: 1, EndLine: 8},
		{StartLine: 3, EndLine: 5},
	})
	s.Toggle(1)

	vis := s.VisibleLines(10)
	expected := []int{0, 1, 9}
	if !reflect.DeepEqual(vis, expected) {
		t.Errorf("expected %v, got %v", expected, vis)
	}
}

func TestBufferToVisible(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{{StartLine: 2, EndLine: 4}})
	s.Toggle(2)
	s.VisibleLines(7)

	if v := s.BufferToVisible(0); v != 0 {
		t.Errorf("expected 0, got %d", v)
	}
	if v := s.BufferToVisible(2); v != 2 {
		t.Errorf("expected 2, got %d", v)
	}
	if v := s.BufferToVisible(3); v != -1 {
		t.Errorf("expected -1 (hidden), got %d", v)
	}
	if v := s.BufferToVisible(5); v != 3 {
		t.Errorf("expected 3, got %d", v)
	}
}

func TestVisibleToBuffer(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{{StartLine: 2, EndLine: 4}})
	s.Toggle(2)
	s.VisibleLines(7)

	if b := s.VisibleToBuffer(0); b != 0 {
		t.Errorf("expected 0, got %d", b)
	}
	if b := s.VisibleToBuffer(2); b != 2 {
		t.Errorf("expected 2, got %d", b)
	}
	if b := s.VisibleToBuffer(3); b != 5 {
		t.Errorf("expected 5, got %d", b)
	}
}

func TestHiddenCount(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{{StartLine: 2, EndLine: 4}})
	s.Toggle(2)

	total := 7
	hidden := total - s.VisibleLineCount(total)
	if hidden != 2 {
		t.Errorf("expected 2 hidden lines, got %d", hidden)
	}
}

func TestSetRangesPreservesCollapsed(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{{StartLine: 2, EndLine: 5}})
	s.Toggle(2)

	s.SetRanges([]Range{
		{StartLine: 2, EndLine: 5},
		{StartLine: 7, EndLine: 9},
	})
	if !s.IsCollapsed(2) {
		t.Error("expected collapse state preserved for existing range")
	}
	if s.IsCollapsed(7) {
		t.Error("new range should not be collapsed")
	}
}

func TestFoldAt(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{{StartLine: 3, EndLine: 6}})

	if r := s.FoldAt(3); r == nil || r.StartLine != 3 {
		t.Error("expected fold at line 3")
	}
	if r := s.FoldAt(4); r != nil {
		t.Error("expected no fold at line 4")
	}
}

func TestContainingFold(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{{StartLine: 2, EndLine: 5}})
	s.Toggle(2)

	if r := s.ContainingFold(3); r == nil || r.StartLine != 2 {
		t.Error("line 3 should be inside fold starting at 2")
	}
	if r := s.ContainingFold(2); r != nil {
		t.Error("fold start line should not be 'contained'")
	}
	if r := s.ContainingFold(6); r != nil {
		t.Error("line 6 is outside the fold")
	}
}

func TestIsLineHidden(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{{StartLine: 2, EndLine: 5}})
	s.Toggle(2)

	if s.IsLineHidden(2) {
		t.Error("fold start line should not be hidden")
	}
	if !s.IsLineHidden(3) {
		t.Error("line 3 should be hidden")
	}
	if s.IsLineHidden(6) {
		t.Error("line 6 should not be hidden")
	}
}

func TestPendingRangesResolveOnDemand(t *testing.T) {
	s := NewState()
	calls := 0
	s.SetPending(func() []Range {
		calls++
		return []Range{{StartLine: 0, EndLine: 2}}
	})
	if !s.Pending() || s.FoldAt(0) != nil {
		t.Fatal("pending state should have no ranges before Resolve")
	}
	s.Resolve()
	s.Resolve()
	if s.Pending() || s.FoldAt(0) == nil || calls != 1 {
		t.Fatalf("Resolve: pending=%v, fold=%v, calls=%d; want ranges computed once", s.Pending(), s.FoldAt(0), calls)
	}

	s.SetPending(func() []Range { return nil })
	if s.FoldAt(0) != nil {
		t.Error("SetPending kept the previous ranges")
	}

	s.SetPending(func() []Range { t.Fatal("SetRanges should drop the pending computation"); return nil })
	s.SetRanges([]Range{{StartLine: 1, EndLine: 2}})
	s.Resolve()
	if s.FoldAt(1) == nil {
		t.Error("SetRanges ranges lost")
	}
}

func TestMarkStaleKeepsRangesUntilResolved(t *testing.T) {
	s := NewState()
	s.SetRanges([]Range{{StartLine: 0, EndLine: 2}})
	gen := s.Generation()
	s.MarkStale(func() []Range { return []Range{{StartLine: 1, EndLine: 3}} })
	if !s.Pending() || !s.Stale() || s.Generation() == gen {
		t.Fatalf("pending=%v stale=%v generation=%d, want a new stale generation", s.Pending(), s.Stale(), s.Generation())
	}
	if s.FoldAt(0) == nil {
		t.Fatal("marking stale dropped the current ranges")
	}
	s.Resolve()
	if s.Pending() || s.Stale() || s.FoldAt(0) != nil || s.FoldAt(1) == nil {
		t.Fatal("resolve did not replace the stale ranges")
	}

	s.SetPending(func() []Range { return nil })
	if s.Stale() || s.FoldAt(1) != nil {
		t.Fatal("SetPending kept ranges or counted as stale")
	}
}
