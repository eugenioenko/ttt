//go:build !windows

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/render"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
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

func TestTerminalPaddingAppliesToSpawnResizeAndSettings(t *testing.T) {
	for name, position := range map[string]ui.SplitPosition{"bottom": ui.SplitBottom, "right": ui.SplitRight} {
		t.Run(name, func(t *testing.T) {
			a := spawnTestApp(t)
			a.Renderer = &render.Renderer{}
			a.Root.SetSize(120, 40)
			a.ContentSplit.Position = position
			a.Root.Render(a.Renderer.NextFrame(120, 40))
			for i := 0; i < 2; i++ {
				id, err := a.SpawnTerminalSpec(TerminalSpec{Shell: "/bin/cat"})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { a.CloseTerminal(id) })
			}
			check := func(width, height, padding int) {
				t.Helper()
				a.Root.SetSize(width, height)
				a.Root.Render(a.Renderer.NextFrame(width, height))
				resizeTerminals(a)
				r := a.TerminalPanel.ActiveWidget().GetRect()
				panel := a.TerminalPanel.GetRect()
				if left, right := r.X-panel.X-ui.VerticalTabBarWidth, panel.X+panel.W-r.X-r.W; left != padding || right != padding {
					t.Fatalf("margins = (%d,%d), want (%d,%d)", left, right, padding, padding)
				}
				for _, tab := range a.Terminals {
					if cols, rows := tab.Term.Size(); cols != r.W || rows != r.H {
						t.Fatalf("terminal %s size = %dx%d, content = %+v", tab.ID, cols, rows, r)
					}
				}
			}
			check(120, 40, 1)
			for _, padding := range []int{0, 2, 1} {
				s := *a.Settings
				s.Terminal.Padding = padding
				a.ApplySettings(s)
				check(100, 35, padding)
				wantCols := a.TerminalPanel.ActiveWidget().GetRect().W
				id, err := a.SpawnTerminalSpec(TerminalSpec{Shell: "/bin/cat"})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { a.CloseTerminal(id) })
				if cols, _ := a.Terminals[len(a.Terminals)-1].Term.Size(); cols != wantCols {
					t.Fatalf("spawn width = %d, want %d", cols, wantCols)
				}
				check(100, 35, padding)
				a.TerminalPanel.SetActive(0)
				check(110, 36, padding)
			}
			s := *a.Settings
			s.Terminal.Padding = int(^uint(0) >> 1)
			a.ApplySettings(s)
			check(110, 36, (a.TerminalPanel.GetRect().W-ui.VerticalTabBarWidth-2)/2)
		})
	}
}

func TestTerminalSpawnUsesConstrainedLayoutBeforeFirstRender(t *testing.T) {
	for name, position := range map[string]ui.SplitPosition{"bottom": ui.SplitBottom, "right": ui.SplitRight} {
		for _, requested := range []int{0, 15, 1000} {
			t.Run(fmt.Sprintf("%s/%d", name, requested), func(t *testing.T) {
				a := spawnTestApp(t)
				a.ContentSplit.SetRect(ui.Rect{X: 2, Y: 3, W: 80, H: 30})
				a.ContentSplit.Position = position
				a.ContentSplit.BottomH = requested
				a.ContentSplit.RightW = requested
				a.ContentSplit.BottomRatio = 0.5
				a.BottomPanel.SetRect(ui.Rect{W: 200, H: 100})
				id, err := a.SpawnTerminalSpec(TerminalSpec{Shell: "/bin/cat"})
				if err != nil {
					t.Fatal(err)
				}
				defer a.CloseTerminal(id)
				cols, rows := a.Terminals[0].Term.Size()
				cells := make([][]term.Cell, 35)
				for y := range cells {
					cells[y] = make([]term.Cell, 85)
				}
				a.ContentSplit.Render(ui.NewRenderSurface(cells, a.ContentSplit.GetRect()))
				r := a.Terminals[0].Widget.GetRect()
				if cols != r.W || rows != r.H {
					t.Fatalf("spawn size = %dx%d, first rendered content = %+v", cols, rows, r)
				}
				resizeTerminals(a)
				if resizedCols, resizedRows := a.Terminals[0].Term.Size(); resizedCols != cols || resizedRows != rows {
					t.Fatalf("first resize changed spawn size from %dx%d to %dx%d", cols, rows, resizedCols, resizedRows)
				}
			})
		}
	}
}
