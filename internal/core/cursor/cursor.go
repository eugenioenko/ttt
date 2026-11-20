package cursor

// Cursor represents the position in the buffer.
type Cursor struct {
	Line int // current line
	Col  int // rune index
	Goal Goal
}

type Goal struct {
	// x is a visual column; line and col identify the last clamped rune position.
	x, line, col int
	valid        bool
}

func (g *Goal) Column(line, col, visualCol int) int {
	if g.valid && g.line == line && g.col == col {
		return g.x
	}
	return visualCol
}

func (g *Goal) Remember(line, col, visualCol int) {
	*g = Goal{x: visualCol, line: line, col: col, valid: true}
}

func (g *Goal) Reset() { *g = Goal{} }
