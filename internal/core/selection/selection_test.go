package selection

import "testing"

func TestRangeNormalization(t *testing.T) {
	s := &Selection{Active: true, Anchor: Position{Line: 5, Col: 10}}
	start, end := s.Range(2, 3)
	if start.Line != 2 || start.Col != 3 {
		t.Errorf("expected start (2,3), got (%d,%d)", start.Line, start.Col)
	}
	if end.Line != 5 || end.Col != 10 {
		t.Errorf("expected end (5,10), got (%d,%d)", end.Line, end.Col)
	}

	start, end = s.Range(8, 0)
	if start.Line != 5 || end.Line != 8 {
		t.Errorf("expected start line 5, end line 8, got %d, %d", start.Line, end.Line)
	}
}

func TestContains(t *testing.T) {
	s := &Selection{Active: true, Anchor: Position{Line: 1, Col: 2}}

	if !s.Contains(1, 3, 2, 5) {
		t.Error("expected (1,3) to be in selection (1,2)-(2,5)")
	}
	if s.Contains(1, 1, 2, 5) {
		t.Error("expected (1,1) to NOT be in selection (1,2)-(2,5)")
	}
	if s.Contains(2, 5, 2, 5) {
		t.Error("expected end position to NOT be in selection (exclusive)")
	}
	if !s.Contains(2, 0, 2, 5) {
		t.Error("expected (2,0) to be in selection")
	}
}

func TestContainsNegativeCol(t *testing.T) {
	s := &Selection{Active: true, Anchor: Position{Line: 1, Col: 2}}

	// col=-1 on the last line of selection should NOT be contained
	if s.Contains(2, -1, 2, 5) {
		t.Error("expected col=-1 on last line to NOT be in selection")
	}
	// col=-1 on a middle line should be contained
	if !s.Contains(2, -1, 3, 5) {
		t.Error("expected col=-1 on middle line to be in selection")
	}
	// col=-1 on the first line (multiline) should be contained
	if !s.Contains(1, -1, 3, 5) {
		t.Error("expected col=-1 on first line to be in selection")
	}
}

func TestContainsInactive(t *testing.T) {
	s := &Selection{Active: false, Anchor: Position{Line: 0, Col: 0}}
	if s.Contains(0, 0, 1, 0) {
		t.Error("inactive selection should not contain anything")
	}
}

func TestTextSingleLine(t *testing.T) {
	lines := []string{"hello world"}
	s := &Selection{Active: true, Anchor: Position{Line: 0, Col: 0}}
	text := s.Text(lines, 0, 5)
	if text != "hello" {
		t.Errorf("expected 'hello', got %q", text)
	}
}

func TestTextMultiLine(t *testing.T) {
	lines := []string{"first line", "second line", "third line"}
	s := &Selection{Active: true, Anchor: Position{Line: 0, Col: 6}}
	text := s.Text(lines, 2, 5)
	if text != "line\nsecond line\nthird" {
		t.Errorf("expected 'line\\nsecond line\\nthird', got %q", text)
	}
}

func TestTextInactive(t *testing.T) {
	s := &Selection{Active: false}
	text := s.Text([]string{"hello"}, 0, 5)
	if text != "" {
		t.Errorf("expected empty string for inactive selection, got %q", text)
	}
}
