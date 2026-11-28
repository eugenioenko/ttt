package ui

import (
	"github.com/gdamore/tcell/v3"
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

func TestMergeDeletingDuplicateContextKeepsConflict(t *testing.T) {
	g, m, _ := openTestMerge(t)
	m.preview.Buf.SetLine(0, "left")
	m.flushProjection()
	m.preview.Buf.Splice(0, 1)
	m.flushProjection()
	if g.Editor.Buf.Lines[0] != "<<<<<<< HEAD" || m.session.blocks[0].custom || m.preview.Buf.Lines[0] != "left" {
		t.Fatal(g.Editor.Buf.Lines, m.session.blocks[0])
	}
}

func TestMergeEmptyResultTakesTyping(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, true, "compact")
	path := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(path, []byte("<<<<<<< HEAD\n=======\nincoming\n>>>>>>> topic"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := g.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	if !g.OpenMerge(scm.ConflictInputs{Current: scm.FileContent{Exists: true}, Incoming: scm.FileContent{Exists: true, Content: "incoming"}}, config.MergeTwo, nil, nil) {
		t.Fatal("open")
	}
	defer g.EndMerge()
	m := g.MergeEditor()
	g.PasteText("custom")
	m.flushProjection()
	if !slices.Equal(g.Editor.Buf.Lines, []string{"<<<<<<< HEAD", "custom", "=======", "incoming", ">>>>>>> topic"}) {
		t.Fatal(g.Editor.Buf.Lines)
	}
}

func TestMergeReplaceAllKeepsBlockOwnership(t *testing.T) {
	g, m, _ := openTestMerge(t)
	m.Navigate(1)
	m.ApplyAction("incoming")
	before := slices.Clone(g.Editor.Buf.Lines)
	g.ReplaceAll("left|new", "replacement", SearchOptions{UseRegex: true})
	want := []string{"pre", "replacement", "middle", "replacement", "post"}
	if !slices.Equal(m.preview.Buf.Lines, want) {
		t.Fatal(m.preview.Buf.Lines)
	}
	for _, b := range m.session.blocks {
		if !b.custom || g.Editor.Buf.Lines[b.start+1] != "replacement" {
			t.Fatal(b, g.Editor.Buf.Lines)
		}
	}
	g.Undo()
	if !slices.Equal(before, g.Editor.Buf.Lines) {
		t.Fatal(g.Editor.Buf.Lines)
	}
}

func TestMergeKeyboardFocusAndSearchCleanup(t *testing.T) {
	g, m, _ := openTestMerge(t)
	m.HandleEvent(tcell.NewEventKey(tcell.KeyF7, "", 0))
	if m.focus != 0 {
		t.Fatal(m.focus)
	}
	g.SetSearch("right", SearchOptions{}, []FindMatch{{Line: 1, Len: 5}})
	m.HandleEvent(tcell.NewEventKey(tcell.KeyRune, " ", 0))
	if m.focus != 2 || m.session.blocks[0].choice != mergeIncoming || !g.SearchTargetChanged() {
		t.Fatal(m.focus)
	}
	g.SetSearch("left", SearchOptions{}, nil)
	if m.sources[0].SearchMatches != nil || m.sources[0].SearchQuery != "" {
		t.Fatal("stale highlight")
	}
	m.focus = 1
	g.ClearSearch()
	if m.preview.SearchQuery != "" {
		t.Fatal("stale result highlight")
	}
}

func TestMergeCompletionDetectsInsertedAlternateWidthConflict(t *testing.T) {
	g, m, _ := openTestMerge(t)
	m.ApplyAction("incoming")
	m.Navigate(1)
	m.ApplyAction("incoming")
	buf := g.Editor.Buf
	buf.Splice(len(buf.Lines), 0, "<<< HEAD", "example")
	if !m.CanComplete() {
		t.Fatal("isolated literal blocked completion")
	}
	buf.Splice(len(buf.Lines), 0, "===", ">>> branch")
	if m.CanComplete() {
		t.Fatal("unresolved alternate width conflict can be staged")
	}
}
