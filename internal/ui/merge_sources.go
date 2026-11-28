package ui

import (
	"slices"
	"strings"

	"github.com/eugenioenko/ttt/internal/core/diff"
)

// Match materialized ranges against each immutable index input. This keeps
// unrelated clean merges and previous resolutions out of the source panes.
func (s *mergeSession) sourceLines(side int, projection []string, content string) []string {
	src := strings.Split(strings.TrimPrefix(strings.ReplaceAll(content, "\r\n", "\n"), "\ufeff"), "\n")
	bounds := make([]int, len(projection)+1)
	old, newPos := 0, 0
	for _, row := range diff.FullDiffLines(projection, src) {
		if row.Left.Num != 0 {
			old++
		}
		if row.Right.Num != 0 {
			newPos++
		}
		bounds[old] = newPos
	}
	var out []string
	at := 0
	for i := range s.blocks {
		b := &s.blocks[i]
		lo := bounds[b.anchors[side]]
		hi := bounds[b.anchors[side]+max(1, len(b.sides[side]))]
		// Equal excerpts are the exact conflict range. Diff alignment can
		// include a neighboring, one-sided edit on the trailing boundary.
		if n := len(b.sides[side]); n > 0 && lo+n <= len(src) && slices.Equal(src[lo:lo+n], b.sides[side]) {
			hi = lo + n
		}
		if len(b.sides[side]) == 0 {
			hi = lo
		}
		lo = max(at, lo)
		hi = max(lo, hi)
		out = append(out, src[at:lo]...)
		b.anchors[side] = len(out)
		if !slices.Equal(b.sides[side], src[lo:hi]) {
			b.custom = true
		}
		b.sides[side] = slices.Clone(src[lo:hi])
		out = append(out, b.sides[side]...)
		if lo == hi {
			out = append(out, "")
		}
		at = hi
	}
	return append(out, src[at:]...)
}
