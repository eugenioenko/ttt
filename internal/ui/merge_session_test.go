package ui

import (
	"slices"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/scm"
)

func TestMergeSourcePanesUseGitVersions(t *testing.T) {
	buf := &buffer.Buffer{Lines: []string{"already merged", "<<<<<<< HEAD", "edited", "=======", "theirs", ">>>>>>> topic", "after"}}
	inputs := scm.ConflictInputs{Current: scm.FileContent{Content: "before\nours\nafter", Exists: true}, Incoming: scm.FileContent{Content: "already merged\ntheirs\nafter", Exists: true}}
	m, panes := newMergeSession(buf, inputs)
	if !slices.Equal(panes[1], []string{"before", "ours", "after"}) || !m.blocks[0].custom {
		t.Fatalf("current pane %q, block %+v", panes[1], m.blocks[0])
	}
	cmd := m.edit(0, mergeCurrent)
	cmd.Apply(buf)
	if !slices.Equal(buf.Lines, []string{"already merged", "ours", "after"}) {
		t.Fatal(buf.Lines)
	}
	cmd.Undo(buf)
	if m.remaining() != 1 || buf.Lines[2] != "edited" {
		t.Fatal("lost pending custom result")
	}
}

func TestGitConflictMarkersRespectWidthAndColumn(t *testing.T) {
	lines := []string{"<<<<<<<<< HEAD", "  =======", "======= title", "||||||||| base", "old", "=========", "right", ">>>>>>>>> other"}
	c, ok := gitConflictAt(lines, 0, 9)
	if !ok || c.base != 3 || c.separator != 5 || c.end != 7 {
		t.Fatal(c, ok)
	}
	for _, bad := range [][]string{
		{"<<<<<<< HEAD", "<<<<<<< nested", "=======", ">>>>>>> other"},
		{"<<<<<<< HEAD", "======= extra", ">>>>>>> other"},
		{"<<<<<<< HEAD", "=======", ">>>>>>>> other"},
	} {
		if _, ok := gitConflictAt(bad, 0, 7); ok {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestMergeEmptyCustomSideRestoresIndexedText(t *testing.T) {
	lines := []string{"before", "<<< HEAD", "===", "incoming", ">>> branch", "after"}
	buf := &buffer.Buffer{Lines: lines}
	inputs := scm.ConflictInputs{MarkerSize: 3, Current: scm.FileContent{Content: "before\noriginal\nafter", Exists: true}, Incoming: scm.FileContent{Content: "before\nincoming\nafter", Exists: true}}
	m, _ := newMergeSession(buf, inputs)
	if m == nil || len(m.blocks) != 1 || !m.blocks[0].custom || !slices.Equal(m.blocks[0].sides[1], []string{"original"}) {
		t.Fatalf("%+v", m)
	}
	if _, ok := gitConflictAt(lines, 1, 7); ok {
		t.Fatal("short literal parsed with a different marker size")
	}
}

func TestMergeTextEditsAtEmptyBoundaryAndUndo(t *testing.T) {
	buf := &buffer.Buffer{Lines: []string{"<<<<<<< HEAD", "a", "=======", "b", ">>>>>>> branch", "<<<<<<< HEAD", "c", "=======", "d", ">>>>>>> branch", "tail"}}
	m, _ := newMergeSession(buf, scm.ConflictInputs{Current: scm.FileContent{Content: "a\nc\ntail", Exists: true}, Incoming: scm.FileContent{Content: "b\nd\ntail", Exists: true}})
	m.edit(0, mergeNeither).Apply(buf)
	m.edit(1, mergeCurrent).Apply(buf)
	buf.Splice(0, 0, "insert")
	if m.remaining() != 1 || m.blocks[0].count != 0 || m.blocks[1].count != 2 {
		t.Fatal(m.blocks)
	}
	buf.Splice(0, 1)
	if m.remaining() != 0 {
		t.Fatal(m.blocks)
	}
	m.edit(1, mergeNeither).Apply(buf)
	buf.SetLine(0, "new tail")
	if m.remaining() != 0 || !m.valid {
		t.Fatal(m.blocks)
	}
}

func TestMergeChangedMarkerSizeFallsBackForCompleteBlocks(t *testing.T) {
	buf := &buffer.Buffer{Lines: []string{"<<<<<<< HEAD", "a", "=======", "b", ">>>>>>> branch"}}
	inputs := scm.ConflictInputs{MarkerSize: 3, Current: scm.FileContent{Exists: true, Content: "a"}, Incoming: scm.FileContent{Exists: true, Content: "b"}}
	if m, _ := newMergeSession(buf, inputs); m != nil {
		t.Fatal("stale marker width accepted")
	}
	buf.SetLines([]string{"<<<<<<< example", "ordinary text"})
	if m, _ := newMergeSession(buf, inputs); m == nil {
		t.Fatal("isolated literal rejected")
	}
}
