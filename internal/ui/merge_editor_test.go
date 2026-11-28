package ui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/scm"
)

const mergeFixture = "pre\n<<<<<<< HEAD\nleft\n=======\nright\n>>>>>>> topic\nmiddle\n<<<<<<< HEAD\n=======\nnew\n>>>>>>> topic\npost"

func openTestMerge(t *testing.T) (*EditorGroupWidget, *MergeEditorWidget, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "a.txt")
	if err := os.WriteFile(path, []byte(mergeFixture), 0600); err != nil {
		t.Fatal(err)
	}
	g := NewEditorGroupWidget(nil, 4, true, "compact")
	if err := g.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	inputs := scm.ConflictInputs{Current: scm.FileContent{Content: "pre\nleft\nmiddle\npost", Exists: true}, Incoming: scm.FileContent{Content: "pre\nright\nmiddle\nnew\npost", Exists: true}}
	if !g.OpenMerge(inputs, config.MergeTwo, func(string) {}, func(func()) { t.Error("unexpected overwrite") }) {
		t.Fatal("open merge")
	}
	t.Cleanup(g.EndMerge)
	return g, g.MergeEditor(), path
}

func TestMergeBlockChoicesUndoAndPartialSave(t *testing.T) {
	g, m, path := openTestMerge(t)
	if g.IsDirty() || m.Remaining() != 2 {
		t.Fatal("opening a merge changed the file")
	}
	m.ApplyAction("current")
	m.ApplyAction("incoming")
	if !slices.Equal(m.preview.Buf.Lines, []string{"pre", "right", "left", "middle", "post"}) {
		t.Fatal(m.preview.Buf.Lines)
	}
	m.ApplyAction("swap")
	if got := m.preview.Buf.Lines[1:3]; !slices.Equal(got, []string{"left", "right"}) {
		t.Fatal(got)
	}
	m.Navigate(1)
	m.ApplyAction("current")
	if !m.CanComplete() {
		t.Fatal("empty side still unresolved")
	}
	g.Undo()
	if m.Remaining() != 1 || !g.Save() {
		t.Fatal("undo did not restore second conflict")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "pre\nleft\nright\nmiddle\n<<<<<<< HEAD\n=======\nnew\n>>>>>>> topic\npost" {
		t.Fatalf("partial save: %q, %v", data, err)
	}
	g.Redo()
	if !m.CanComplete() {
		t.Fatal("redo failed")
	}
}

func TestMergeManualEditsShareFileHistory(t *testing.T) {
	g, m, path := openTestMerge(t)
	m.ApplyAction("incoming")
	m.preview.Cursor.Line, m.preview.Cursor.Col = 1, 5
	g.PasteText("!")
	m.flushProjection()
	if m.Remaining() != 2 || !g.Save() {
		t.Fatal("manual edit did not reopen block")
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "<<<<<<< HEAD\nright!\n=======\nright\n>>>>>>> topic") {
		t.Fatalf("saved custom result: %q, %v", data, err)
	}
	warned := false
	m.confirm = func(func()) { warned = true }
	m.ApplyAction("current")
	if !warned || m.preview.Buf.Lines[1] != "right!" {
		t.Fatal("manual edit overwritten")
	}
	m.ApplyAction("manual")
	if m.Remaining() != 1 || g.Editor.Buf.Lines[1] != "right!" {
		t.Fatal("manual resolution lost")
	}
	g.Undo()
	g.Undo()
	m.project()
	if m.Remaining() != 1 || m.preview.Buf.Lines[1] != "right" {
		t.Fatal("undo failed", g.Editor.Buf.Lines)
	}
}

func TestMergeCrossBlockEditCanUndo(t *testing.T) {
	g, m, _ := openTestMerge(t)
	before := slices.Clone(g.Editor.Buf.Lines)
	m.preview.Buf.Splice(0, 4, "replacement")
	m.flushProjection()
	if !slices.Equal(m.preview.Buf.Lines, []string{"replacement"}) {
		t.Fatal("projection", m.preview.Buf.Lines)
	}
	g.Undo()
	m.project()
	if !slices.Equal(g.Editor.Buf.Lines, before) || !m.session.valid {
		t.Fatal("undo", g.Editor.Buf.Lines, m.session.blocks)
	}
}
