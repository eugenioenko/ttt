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
