//go:build !windows

package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/term"
)

func spawnTestApp(t *testing.T) *App {
	t.Helper()
	a := buildTestApp(t, config.DefaultSettings())
	sim := term.NewSimScreen()
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sim.Fini)
	a.Screen = term.NewTcellScreenFrom(sim)
	return a
}

// markerSpec runs a child that records it started by touching marker.
func markerSpec(cwd, marker string) TerminalSpec {
	return TerminalSpec{Shell: "/bin/sh", Args: []string{"-c", `touch "$0"; pwd; exec cat`, marker}, Cwd: cwd}
}

func TestSpawnTerminalSpecRejectsInvalidCwd(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for name, cwd := range map[string]string{
		"missing": filepath.Join(tmp, "missing"),
		"file":    file,
	} {
		t.Run(name, func(t *testing.T) {
			a := spawnTestApp(t)
			marker := filepath.Join(tmp, name+".started")
			id, err := a.SpawnTerminalSpec(markerSpec(cwd, marker))
			if err == nil {
				a.CloseTerminal(id)
				t.Fatalf("SpawnTerminalSpec(cwd=%q) succeeded, want an error", cwd)
			}
			if len(a.Terminals) != 0 || a.TerminalPanel.Count() != 0 {
				t.Fatalf("a failed spawn must not add a tab: %d terminals, %d panel tabs", len(a.Terminals), a.TerminalPanel.Count())
			}
			time.Sleep(100 * time.Millisecond)
			if _, err := os.Stat(marker); err == nil {
				t.Fatal("the child ran despite the invalid cwd")
			}
		})
	}
}

func TestSpawnTerminalSpecUsesExplicitCwd(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := spawnTestApp(t)
	id, err := a.SpawnTerminalSpec(markerSpec(dir, filepath.Join(t.TempDir(), "started")))
	if err != nil {
		t.Fatalf("SpawnTerminalSpec: %v", err)
	}
	defer a.CloseTerminal(id)
	waitForTerminalOutput(t, a.Terminals[0].Term, dir)
}

func TestTerminalCtrlVPastesAppliesToExistingAndNewTabs(t *testing.T) {
	a := spawnTestApp(t)
	a.Renderer = &render.Renderer{}
	s := *a.Settings
	s.Terminal.CtrlVPastes = false
	a.ApplySettings(s)
	for i := 0; i < 2; i++ {
		id, err := a.SpawnTerminalSpec(TerminalSpec{Shell: "/bin/cat"})
		if err != nil {
			t.Fatal(err)
		}
		defer a.CloseTerminal(id)
		if a.Terminals[i].Widget.CtrlVPastes {
			t.Fatal("new terminal ignored ctrlVPastes=false")
		}
	}
	for _, enabled := range []bool{true, false, true} {
		s.Terminal.CtrlVPastes = enabled
		a.ApplySettings(s)
		for _, tab := range a.Terminals {
			if tab.Widget.CtrlVPastes != enabled {
				t.Errorf("terminal %s: CtrlVPastes = %v, want %v", tab.ID, tab.Widget.CtrlVPastes, enabled)
			}
		}
	}
	id, err := a.SpawnTerminalSpec(TerminalSpec{Shell: "/bin/cat"})
	if err != nil {
		t.Fatal(err)
	}
	defer a.CloseTerminal(id)
	if !a.Terminals[2].Widget.CtrlVPastes {
		t.Fatal("new terminal ignored ctrlVPastes=true")
	}
}
