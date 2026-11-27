package cursor

import "testing"

func TestGoalSurvivesClamping(t *testing.T) {
	c := Cursor{Col: 12}
	for _, p := range []struct{ line, col, visual int }{{1, 5, 5}, {2, 0, 0}, {3, 2, 4}, {4, 12, 12}} {
		x := c.Goal.Column(c.Line, c.Col, 12)
		c.Line, c.Col = p.line, p.col
		c.Goal.Remember(c.Line, c.Col, x)
		if got := c.Goal.Column(c.Line, c.Col, p.visual); got != 12 {
			t.Fatalf("goal at (%d,%d) = %d, want 12", c.Line, c.Col, got)
		}
	}
}

func TestGoalResetsAfterPositionChange(t *testing.T) {
	for _, p := range []struct{ line, col int }{{1, 4}, {2, 5}} {
		var goal Goal
		goal.Remember(1, 5, 12)
		if got := goal.Column(p.line, p.col, 4); got != 4 {
			t.Fatalf("goal after moving to (%d,%d) = %d, want 4", p.line, p.col, got)
		}
	}
}

func TestGoalResetAtSamePosition(t *testing.T) {
	var goal Goal
	goal.Remember(1, 5, 12)
	goal.Reset()
	if got := goal.Column(1, 5, 5); got != 5 {
		t.Fatalf("goal after reset = %d, want 5", got)
	}
	goal.Remember(2, 0, 0)
	if got := goal.Column(2, 0, 7); got != 0 {
		t.Fatalf("zero goal = %d, want 0", got)
	}
}
