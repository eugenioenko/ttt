package ui

import (
	"fmt"
	"maps"
	"math/rand"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/undo"
)

type liveDiffState struct {
	screen         []string
	kinds          []diff.LineKind
	deleted        map[int][]string
	gaps           map[int]int
	labels         map[int]string
	hidden         []diffHiddenRange
	phantoms       map[int]int
	leftPhantoms   map[int]int
	starts         []int
	total          int
	leftTotal      int
	rightFillers   map[int]int
	leftFillers    map[int]int
	liveRows, rowR []int
}

func nonZero(m map[int]int) map[int]int {
	out := map[int]int{}
	for k, v := range m {
		if v != 0 {
			out[k] = v
		}
	}
	return out
}

// captureLiveDiff records the state before drawing, since drawing realigns a
// split diff from scratch when its alignment is out of date.
func captureLiveDiff(f *liveDiffFixture) liveDiffState {
	f.d.bind(f.e)
	if !f.d.IsUnified() && f.d.alignContribs == nil {
		f.d.alignSplit()
	}
	var st liveDiffState
	o := f.e.DiffOverlay
	st.kinds = slices.Clone(o.Kinds)
	st.deleted = map[int][]string{}
	for k, block := range o.Deleted {
		for _, side := range block {
			st.deleted[k] = append(st.deleted[k], side.Text)
		}
	}
	st.gaps, st.labels = maps.Clone(o.Gaps), maps.Clone(o.Labels)
	st.hidden = slices.Clone(o.Hidden)
	st.phantoms = nonZero(f.e.phantoms)
	l := f.e.layout()
	for line := 0; line <= len(f.e.Buf.Lines); line++ {
		st.starts = append(st.starts, l.startRow(line))
	}
	st.total = l.total()
	st.liveRows = slices.Clone(f.d.liveRows)
	if !f.d.IsUnified() {
		st.rightFillers = nonZero(f.d.rightBase.Fillers)
		st.leftFillers = nonZero(f.d.leftBase.Fillers)
		st.leftPhantoms = nonZero(f.d.left.phantoms)
		st.leftTotal = f.d.left.layout().total()
		for _, p := range f.d.pairs {
			st.rowR = append(st.rowR, p.r)
		}
	}
	st.screen = f.render()
	return st
}

func randomLiveDiffFiles(rng *rand.Rand, n int) (old, cur []string) {
	word := func() string {
		return strings.Repeat(string(rune('a'+rng.Intn(26))), 1+rng.Intn(70))
	}
	for i := 0; i < n; i++ {
		line := fmt.Sprintf("%d %s", i, word())
		old = append(old, line)
		switch rng.Intn(12) {
		case 0:
		case 1:
			cur = append(cur, line, word())
		case 2:
			cur = append(cur, word())
		default:
			cur = append(cur, line)
		}
	}
	return old, cur
}

func randomLiveEdit(rng *rand.Rand, e *EditorPaneWidget) {
	n := len(e.Buf.Lines)
	line := rng.Intn(n)
	switch rng.Intn(7) {
	case 0, 1:
		e.exec(&undo.InsertRuneCommand{Line: line, Col: 0, Rune: 'x'})
	case 2:
		if len([]rune(e.Buf.Lines[line])) > 0 {
			e.exec(&undo.DeleteRuneCommand{Line: line, Col: 0})
		}
	case 3:
		e.exec(&undo.SplitLineCommand{Line: line, Col: min(3, len([]rune(e.Buf.Lines[line])))})
	case 4:
		if line+1 < n {
			e.exec(&undo.JoinLineCommand{Line: line + 1})
		}
	case 5:
		e.exec(&undo.InsertLineCommand{Idx: line, Text: "inserted " + strings.Repeat("y", rng.Intn(90))})
	case 6:
		if n > 2 {
			e.exec(&undo.DeleteLineCommand{Idx: line})
		}
	}
}

// TestLiveDiffEditsMatchARebuild edits an editable diff at random and checks,
// after each edit, that the overlay, alignment and row layout moved by the
// change log match the ones rebuilt from the diff and the recorded edits.
func TestLiveDiffEditsMatchARebuild(t *testing.T) {
	for _, mode := range []DiffMode{DiffModeUnified, DiffModeSplit} {
		for _, wrap := range []bool{false, true} {
			for _, compact := range []bool{false, true} {
				name := fmt.Sprintf("mode=%d/wrap=%v/compact=%v", mode, wrap, compact)
				t.Run(name, func(t *testing.T) {
					rng := rand.New(rand.NewSource(int64(mode)*7 + 3))
					old, cur := randomLiveDiffFiles(rng, 120)
					f := newLiveDiffFixture(t, old, cur, mode, 100, 30)
					f.d.SetWrapped(wrap)
					if !compact {
						f.d.SetContextMode(DiffContextFullFile)
					}
					f.render()
					for step := 0; step < 150; step++ {
						if rng.Intn(4) == 0 {
							f.e.Viewport.TopLine = rng.Intn(len(f.e.Buf.Lines))
						}
						f.e.Cursor.Line = rng.Intn(len(f.e.Buf.Lines))
						f.e.Cursor.Col = 0
						f.d.bind(f.e)
						randomLiveEdit(rng, f.e)
						got := captureLiveDiff(f)
						f.d.rebuildLive()
						want := captureLiveDiff(f)
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("step %d: incremental state differs from a rebuild\ngot  %+v\nwant %+v", step, diffFields(got, want), diffFields(want, got))
						}
					}
				})
			}
		}
	}
}

func diffFields(a, b liveDiffState) map[string]string {
	out := map[string]string{}
	av, bv := reflect.ValueOf(a), reflect.ValueOf(b)
	for i := 0; i < av.NumField(); i++ {
		if x, y := fmt.Sprintf("%v", av.Field(i)), fmt.Sprintf("%v", bv.Field(i)); x != y {
			out[av.Type().Field(i).Name] = x
		}
	}
	return out
}

// TestLiveDiffFromAnOlderVersionReplaysLaterEdits hands the widget a diff
// computed before some edits, as a recompute that raced with typing does, and
// checks it lands where the diff tracked through those edits already was.
func TestLiveDiffFromAnOlderVersionReplaysLaterEdits(t *testing.T) {
	for _, mode := range []DiffMode{DiffModeUnified, DiffModeSplit} {
		rng := rand.New(rand.NewSource(11))
		old, cur := randomLiveDiffFiles(rng, 80)
		f := newLiveDiffFixture(t, old, cur, mode, 100, 30)
		f.d.SetContextMode(DiffContextFullFile)
		f.render()
		ver, snap := f.e.Buf.Version(), slices.Clone(f.e.Buf.Lines)
		for i := 0; i < 20; i++ {
			randomLiveEdit(rng, f.e)
			f.render()
		}
		tracked := captureLiveDiff(f)
		f.d.SetLiveDiff(old, diff.FullDiffLines(old, snap), ver, snap)
		got := captureLiveDiff(f)
		if !reflect.DeepEqual(got, tracked) {
			t.Fatalf("mode %d: diff of an older version differs from the tracked one\ngot  %+v\nwant %+v", mode, diffFields(got, tracked), diffFields(tracked, got))
		}
	}
}
