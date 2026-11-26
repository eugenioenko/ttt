//go:build !windows

package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/ui"
)

func TestEditorTerminalsResolveToTheirOwnGroup(t *testing.T) {
	a := spawnTestApp(t)
	a.Renderer = &render.Renderer{}
	a.Root.SetSize(120, 40)
	frame := func() {
		a.Root.Render(a.Renderer.NextFrame(120, 40))
		resizeTerminals(a)
	}
	frame()

	left := a.EditorGroup
	a.SplitEditor()
	right := a.EditorGroup
	a.activateEditorGroup(left)
	a.SplitEditorDown()
	short := a.EditorGroup
	frame()

	spawn := func() string {
		t.Helper()
		id, err := a.SpawnTerminalSpec(TerminalSpec{Shell: "/bin/cat", InEditor: true})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { a.CloseTerminal(id) })
		return id
	}
	shortID := spawn()
	a.activateEditorGroup(right)
	rightID := spawn()
	frame()

	if s, r := short.ContentRect(), right.ContentRect(); s.H == r.H {
		t.Fatalf("groups must differ in height for this test: %+v vs %+v", s, r)
	}
	assertSizes := func(focused string) {
		t.Helper()
		for id, g := range map[string]*ui.EditorGroupWidget{shortID: short, rightID: right} {
			tt := a.terminalByID(id)
			if got, want := tt.Tab.GetRect(), g.ContentRect(); got != want {
				t.Fatalf("with %s focused, %s tab=%+v, its group's area=%+v", focused, id, got, want)
			}
			content := tt.Tab.ContentRect()
			if cols, rows := tt.Term.Size(); cols != content.W || rows != content.H {
				t.Fatalf("with %s focused, %s PTY=%dx%d, its pane=%dx%d", focused, id, cols, rows, content.W, content.H)
			}
			if !g.HasPluginTab(editorTerminalKey(id)) {
				t.Fatalf("%s is not in its group", id)
			}
		}
	}
	assertSizes("right")

	if err := a.FocusTerminal(shortID); err != nil {
		t.Fatal(err)
	}
	frame()
	if a.EditorGroup != short || a.Root.Focused != short || short.ActiveTerminalTab() != a.terminalByID(shortID).Tab {
		t.Fatal("FocusTerminal must activate and focus the terminal's own group")
	}
	assertSizes("short")

	if err := a.FocusTerminal(rightID); err != nil {
		t.Fatal(err)
	}
	frame()
	assertSizes("right")

	a.CloseTerminal(shortID)
	frame()
	if a.terminalByID(shortID) != nil {
		t.Fatal("CloseTerminal must release the terminal")
	}
	for _, g := range a.editorGroups() {
		if g.HasPluginTab(editorTerminalKey(shortID)) {
			t.Fatal("CloseTerminal must close the tab in a group that is not active")
		}
	}
}
