//go:build !windows

package app

import (
	"testing"
)

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
	check("dialog dismissed", "editor", 1)
}
