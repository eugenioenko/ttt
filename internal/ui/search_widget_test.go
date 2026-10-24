package ui

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/fileicons"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestSearchFileRowsShowIconsAndOmitIconSpacingWhenDisabled(t *testing.T) {
	for _, tc := range []struct {
		mode     string
		iconX    rune
		fileName rune
	}{
		{mode: config.IconsNerdFont, iconX: []rune(fileicons.ForFile("Dockerfile").Glyph)[0], fileName: 'D'},
		{mode: config.IconsNone, fileName: 'D'},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			s := NewSearchWidget()
			s.Icons = tc.mode
			s.Selected = -1
			s.lastSelected = -1
			s.Groups = []SearchFileGroup{{
				FilePath: "/repo/Dockerfile",
				RelPath:  "Dockerfile",
				Expanded: true,
				Matches:  []SearchMatch{{LineNum: 1}},
			}}
			s.flatten()
			grid := makeGrid(40, 8)
			s.Render(NewRenderSurface(grid, Rect{W: 40, H: 8}))

			row := grid[4]
			if tc.mode == config.IconsNerdFont {
				if row[2].Ch != tc.iconX || row[2].Style != term.StyleFileIconBlue {
					t.Fatalf("file icon cell = %+v, want Dockerfile blue icon", row[2])
				}
				if row[4].Ch != tc.fileName {
					t.Fatalf("filename starts at column 4, got %q", row[4].Ch)
				}
			} else if row[2].Ch != tc.fileName {
				t.Fatalf("filename should start at column 2 with icons disabled, got %q", row[2].Ch)
			}
		})
	}
}

func TestSearchDiffRowsUseUnderlyingFileIconPath(t *testing.T) {
	s := NewSearchWidget()
	s.Icons = config.IconsNerdFont
	s.Selected = -1
	s.lastSelected = -1
	s.Groups = []SearchFileGroup{{
		FilePath: "diff-tab-id",
		IconPath: "/repo/Dockerfile",
		RelPath:  "diff-tab-id",
		Expanded: true,
		Matches:  []SearchMatch{{LineNum: 1, LineText: "Dockerfile"}},
	}}
	s.flatten()

	if got := s.Groups[0].FilePath; got != "diff-tab-id" {
		t.Fatalf("search file identity = %q, want diff tab identity", got)
	}
	grid := makeGrid(40, 8)
	s.Render(NewRenderSurface(grid, Rect{W: 40, H: 8}))
	row := grid[s.resultStartY]
	icon := []rune(fileicons.ForFile("Dockerfile").Glyph)[0]
	if row[2].Ch != icon || row[2].Style != term.StyleFileIconBlue {
		t.Fatalf("diff search icon = %+v, want Dockerfile icon", row[2])
	}
}

func TestSearchFileRowsReserveReplaceActionsAndScrollbarForWidePaths(t *testing.T) {
	s := NewSearchWidget()
	s.Icons = config.IconsNerdFont
	s.showReplace = true
	s.ReplaceInput.Text = "replacement"
	s.Selected = -1
	s.lastSelected = -1
	s.Groups = []SearchFileGroup{{
		FilePath: "/repo/界界界界界界",
		RelPath:  "界界界界界界",
		Expanded: true,
		Matches:  []SearchMatch{{LineNum: 1}},
	}}
	s.flatten()
	grid := makeGrid(14, 8)
	s.Render(NewRenderSurface(grid, Rect{W: 14, H: 8}))

	row := grid[s.resultStartY]
	if row[8].Ch != ' ' {
		t.Fatalf("wide path glyph crossed into preview hitbox at column 9: %q", row[8].Ch)
	}
	if row[10].Ch != '⊙' || row[12].Ch != '⟳' {
		t.Fatalf("replace actions overwritten by path: %q %q\n%s", row[10].Ch, row[12].Ch, detailTestText(grid))
	}
	if row[13].Ch != ' ' {
		t.Fatalf("path crossed into scrollbar column: %q", row[13].Ch)
	}
}

func TestRgFailureIgnoresNoMatches(t *testing.T) {
	// rg exits 1 when it finds nothing, which is an empty result, not a failure.
	err := exec.Command("sh", "-c", "exit 1").Run()
	if err == nil {
		t.Fatal("expected a non-nil error from exit 1")
	}
	if got := rgFailure(err); got != "" {
		t.Errorf("rgFailure(exit 1) = %q, want empty", got)
	}
}

func TestRgFailureReportsRealFailures(t *testing.T) {
	err := exec.Command("sh", "-c", "exit 2").Run()
	if err == nil {
		t.Fatal("expected a non-nil error from exit 2")
	}
	if got := rgFailure(err); got == "" {
		t.Error("rgFailure(exit 2) = empty, want a message")
	}

	if got := rgFailure(errors.New("boom")); got != "search failed: boom" {
		t.Errorf("rgFailure(boom) = %q", got)
	}
	if got := rgFailure(nil); got != "" {
		t.Errorf("rgFailure(nil) = %q, want empty", got)
	}
}

func TestApplyBatchSurfacesError(t *testing.T) {
	s := NewSearchWidget()
	s.searchGen = 7

	s.ApplyBatch(&SearchBatch{Gen: 7, Done: true, Error: "ripgrep (rg) not found"})
	if s.Error != "ripgrep (rg) not found" {
		t.Fatalf("Error = %q, want the rg message", s.Error)
	}
	if s.Searching {
		t.Error("expected Searching to be cleared")
	}
}

func TestApplyBatchIgnoresStaleGeneration(t *testing.T) {
	s := NewSearchWidget()
	s.searchGen = 8

	s.ApplyBatch(&SearchBatch{Gen: 7, Done: true, Error: "stale"})
	if s.Error != "" {
		t.Errorf("Error = %q, want empty for a stale batch", s.Error)
	}
}

// A partial batch must not wipe a message set by the run that follows it.
func TestApplyBatchKeepsErrorUntilDone(t *testing.T) {
	s := NewSearchWidget()
	s.searchGen = 3
	s.Error = "previous"

	s.ApplyBatch(&SearchBatch{Gen: 3})
	if s.Error != "previous" {
		t.Errorf("Error = %q, want it untouched by a partial batch", s.Error)
	}

	s.ApplyBatch(&SearchBatch{Gen: 3, Done: true})
	if s.Error != "" {
		t.Errorf("Error = %q, want cleared by a successful final batch", s.Error)
	}
}

func TestSearchStartClearsError(t *testing.T) {
	s := NewSearchWidget()
	s.Error = "ripgrep (rg) not found"
	s.Input.Text = ""

	s.runSearchSync()
	if s.Error != "" {
		t.Errorf("Error = %q, want cleared when a new search starts", s.Error)
	}
}

func TestSearchFilterToggleAndPerFileReplaceFollowIconModeAndOverrides(t *testing.T) {
	t.Cleanup(func() { icons.Configure("", nil) })

	for _, mode := range []string{config.IconsNone, config.IconsNerdFont} {
		t.Run("defaults_"+mode, func(t *testing.T) {
			icons.Configure(mode, nil)

			s := NewSearchWidget()
			s.Icons = mode
			s.showReplace = true
			s.ReplaceInput.Text = "next"
			s.Selected = -1
			s.lastSelected = -1
			s.Groups = []SearchFileGroup{{
				FilePath: "/repo/main.go",
				RelPath:  "main.go",
				Expanded: true,
				Matches:  []SearchMatch{{LineNum: 1, LineText: "old"}},
			}}
			s.flatten()

			const w, h = 30, 10
			grid := makeGrid(w, h)
			s.Render(NewRenderSurface(grid, Rect{W: w, H: h}))

			wantCollapsed := []rune(icons.Get(mode, icons.ChevronDown))[0]
			if got := grid[s.toggleRow()][w-2].Ch; got != wantCollapsed {
				t.Fatalf("collapsed filter toggle glyph = %q, want %q", got, wantCollapsed)
			}
			if grid[s.toggleRow()][w-3].Ch != ' ' || grid[s.toggleRow()][w-1].Ch != ' ' {
				t.Fatalf("collapsed filter toggle padding = (%q, %q), want spaces", grid[s.toggleRow()][w-3].Ch, grid[s.toggleRow()][w-1].Ch)
			}

			wantReplace := []rune(icons.Get(mode, icons.Replace))[0]
			if got := grid[s.resultStartY][w-2].Ch; got != wantReplace {
				t.Fatalf("per-file replace glyph = %q, want %q", got, wantReplace)
			}

			s.showFilters = true
			grid = makeGrid(w, h)
			s.Render(NewRenderSurface(grid, Rect{W: w, H: h}))

			wantExpanded := []rune(icons.Get(mode, icons.ArrowLeft))[0]
			if got := grid[s.toggleRow()][w-2].Ch; got != wantExpanded {
				t.Fatalf("expanded filter toggle glyph = %q, want %q", got, wantExpanded)
			}
		})
	}

	t.Run("overrides", func(t *testing.T) {
		warnings := icons.Configure(config.IconsNone, map[string]string{
			"chevron-down": "v",
			"arrow-left":   "<",
			"replace":      "R",
		})
		if len(warnings) != 0 {
			t.Fatalf("unexpected configure warnings: %v", warnings)
		}

		s := NewSearchWidget()
		s.showReplace = true
		s.ReplaceInput.Text = "next"
		s.Selected = -1
		s.lastSelected = -1
		s.Groups = []SearchFileGroup{{
			FilePath: "/repo/main.go",
			RelPath:  "main.go",
			Expanded: true,
			Matches:  []SearchMatch{{LineNum: 1, LineText: "old"}},
		}}
		s.flatten()

		const w, h = 30, 12
		s.SetRect(Rect{X: 5, Y: 2, W: w, H: h})
		grid := makeGrid(w, h)
		s.Render(NewRenderSurface(grid, Rect{W: w, H: h}))

		if got := grid[s.toggleRow()][w-2].Ch; got != 'v' {
			t.Fatalf("overridden collapsed filter toggle = %q, want 'v'", got)
		}
		if got := grid[s.resultStartY][w-2].Ch; got != 'R' {
			t.Fatalf("overridden per-file replace glyph = %q, want 'R'", got)
		}

		// Clicking border outside filter toggle hit region must not toggle filters.
		s.HandleEvent(tcell.NewEventMouse(5+w-4, 2+s.toggleRow(), tcell.Button1, tcell.ModNone))
		if s.showFilters {
			t.Fatal("click outside filter toggle hit region unexpectedly toggled showFilters")
		}

		// Clicking inside the rendered filter toggle hit region toggles filters on.
		s.HandleEvent(tcell.NewEventMouse(5+w-2, 2+s.toggleRow(), tcell.Button1, tcell.ModNone))
		if !s.showFilters {
			t.Fatal("click on filter toggle hit region did not enable showFilters")
		}

		grid = makeGrid(w, h)
		s.Render(NewRenderSurface(grid, Rect{W: w, H: h}))
		if got := grid[s.toggleRow()][w-2].Ch; got != '<' {
			t.Fatalf("overridden expanded filter toggle = %q, want '<'", got)
		}

		s.Input.Text = "old"
		replacedFile := ""
		s.OnReplaceRequest = func(req SearchReplaceRequest) {
			replacedFile = req.Paths[0] + ":" + req.Replacement
		}
		s.HandleEvent(tcell.NewEventMouse(5+w-2, 2+s.resultStartY, tcell.Button1, tcell.ModNone))
		if replacedFile != "/repo/main.go:next" {
			t.Fatalf("per-file replace callback = %q, want %q", replacedFile, "/repo/main.go:next")
		}
	})
}
