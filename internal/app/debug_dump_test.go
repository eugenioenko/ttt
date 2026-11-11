//go:build !windows

package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/terminal"
	"github.com/eugenioenko/ttt/internal/ui"
)

func TestDebugStateSelectsEditorAndPanelTerminals(t *testing.T) {
	a := spawnTestApp(t)
	check := func(id string, index int) {
		t.Helper()
		s := a.BuildDebugState()
		want := DebugActiveTerminal{ID: id, Index: index}
		if s.ActiveTerminal == nil || *s.ActiveTerminal != want {
			t.Fatalf("active_terminal = %+v, want %+v", s.ActiveTerminal, want)
		}
		if s.Terminals[index].ID != id {
			t.Fatalf("terminals[%d].ID = %q, want %q", index, s.Terminals[index].ID, id)
		}
	}

	if s := a.BuildDebugState(); s.ActiveTerminal != nil {
		t.Fatalf("active_terminal = %+v without terminals", s.ActiveTerminal)
	}
	for _, id := range []string{"editor-0", "editor-1"} {
		tw := ui.NewTerminalWidget(&terminal.Terminal{}, nil)
		entry := TerminalTab{ID: id, Term: tw.Term, Widget: tw, Tab: ui.NewTerminalTabWidget(tw)}
		a.openEditorTerminal(entry, TerminalSpec{InEditor: true})
	}
	check("editor-1", 1)
	a.Root.SetFocus(a.Sidebar)
	check("editor-1", 1)

	for _, id := range []string{"panel-0", "panel-1"} {
		tw := ui.NewTerminalWidget(&terminal.Terminal{}, nil)
		a.Terminals = append(a.Terminals, TerminalTab{ID: id, Term: tw.Term, Widget: tw})
		a.TerminalPanel.AddTerminal(tw)
	}
	a.Root.SetFocus(a.TerminalPanel)
	check("panel-1", 1)
	a.TerminalPanel.SetActive(0)
	check("panel-0", 0)

	a.focusEditorTerminal("editor-0")
	check("editor-0", 2)
	a.focusEditorTerminal("editor-1")
	check("editor-1", 3)
	a.Root.SetFocus(a.Sidebar)
	check("editor-1", 3)

	a.EditorGroup.SwitchTab(0)
	a.Root.SetFocus(a.EditorGroup)
	check("panel-0", 0)
}

func TestDebugStateReportsTerminalFocus(t *testing.T) {
	a := spawnTestApp(t)
	cat := TerminalSpec{Shell: "/bin/sh", Args: []string{"-c", "exec cat"}, Cwd: t.TempDir()}
	var ids []string
	for range 2 {
		id, err := a.SpawnTerminalSpec(cat)
		if err != nil {
			t.Fatalf("SpawnTerminalSpec: %v", err)
		}
		defer a.CloseTerminal(id)
		ids = append(ids, id)
	}

	check := func(step, focus string, index int) {
		t.Helper()
		s := a.BuildDebugState()
		if s.Focus != focus {
			t.Errorf("%s: focus = %q, want %q", step, s.Focus, focus)
		}
		want := DebugActiveTerminal{ID: ids[index], Index: index}
		if s.ActiveTerminal == nil || *s.ActiveTerminal != want {
			t.Errorf("%s: active_terminal = %+v, want %+v", step, s.ActiveTerminal, want)
		}
	}

	check("after spawn", "terminal", 1)

	if err := a.FocusTerminal(ids[0]); err != nil {
		t.Fatal(err)
	}
	check("after switching tabs", "terminal", 0)

	a.FocusEditor()
	check("editor focused", "editor", 0)

	if err := a.FocusTerminal(ids[1]); err != nil {
		t.Fatal(err)
	}
	a.ShowInputDialog("Name", "", "", func(string) {})
	check("dialog over terminal", "overlay", 1)

	a.DismissDialog()
	check("dialog dismissed", "terminal", 1)
}
