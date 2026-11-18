package ui

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/fileicons"
	"github.com/eugenioenko/ttt/internal/icons"
	"github.com/eugenioenko/ttt/internal/search"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/gdamore/tcell/v3"
)

func TestSearchStreamsLongMatchingLines(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep not installed")
	}
	t.Setenv("RIPGREP_CONFIG_PATH", "")
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.txt")
	longLine := "asdf" + strings.Repeat("x", 2*1024*1024)
	if err := os.WriteFile(path, []byte(longLine+"\nafter asdf\n"), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewSearchWidget()
	s.SetWorkDirs([]string{dir})
	seedInputText(s.Input, "asdf")
	s.Searching = true
	var done bool
	s.PostBatch = func(batch *SearchBatch) {
		s.ApplyBatch(batch)
		done = batch.Done
	}
	s.streamSearch(context.Background(), s.searchGen)
	if !done || s.Searching || s.Error != "" {
		t.Fatalf("search completion: done=%t searching=%t error=%q", done, s.Searching, s.Error)
	}
	if len(s.Groups) != 1 || len(s.Groups[0].Matches) != 2 {
		t.Fatalf("expected both matches in one file, got %d groups", len(s.Groups))
	}
	for i, want := range []SearchMatch{
		{FilePath: path, LineNum: 1, ColStart: 0, ColEnd: 4, LineText: longLine},
		{FilePath: path, LineNum: 2, ColStart: 6, ColEnd: 10, LineText: "after asdf"},
	} {
		if s.Groups[0].Matches[i] != want {
			t.Errorf("match %d has incorrect path, line, columns, or text", i)
		}
	}
}

func TestSearchRecordLimitPreservesEarlierMatches(t *testing.T) {
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("ripgrep not installed")
	}
	t.Setenv("RIPGREP_CONFIG_PATH", "")
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.txt")
	content := "first asdf\n" + strings.Repeat("x", search.MaxRecordBytes) + "asdf\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewSearchWidget()
	s.SetWorkDirs([]string{dir})
	seedInputText(s.Input, "asdf")
	s.Searching = true
	s.PostBatch = s.ApplyBatch
	s.streamSearch(context.Background(), s.searchGen)
	if s.Searching || s.Error != "16 MiB record limit reached" {
		t.Fatalf("search completion: searching=%t error=%q", s.Searching, s.Error)
	}
	if len(s.Groups) != 1 || len(s.Groups[0].Matches) != 1 || s.Groups[0].Matches[0].LineText != "first asdf" {
		t.Fatal("expected the first match to survive the record limit")
	}
}

func TestSearchInputChevronFollowsFocus(t *testing.T) {
	s := NewSearchWidget()
	s.SetReplaceMode(true)
	s.showFilters = true
	s.SetRect(Rect{W: 40, H: 12})
	inputs := []*widgets.InputWidget{s.Input, s.ReplaceInput, s.Include, s.Exclude}
	check := func(active *widgets.InputWidget) {
		t.Helper()
		grid := makeGrid(40, 12)
		s.Render(NewRenderSurface(grid, s.GetRect()))
		for _, input := range inputs {
			want := term.StyleBorder
			if input == active {
				want = term.StyleBorderActive
			}
			r := input.GetRect()
			if got := grid[r.Y][r.X+1]; got.Ch != '❯' || got.Style != want {
				t.Fatalf("input %q chevron = %+v, want style %v", input.Config.Placeholder, got, want)
			}
			if input.IsFocused() != (input == active) {
				t.Fatalf("input %q focus = %t", input.Config.Placeholder, input.IsFocused())
			}
		}
	}
	check(nil)
	s.SetFocused(true)
	check(s.Input)
	for _, input := range inputs[1:] {
		s.HandleEvent(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone))
		check(input)
	}
	s.HandleEvent(tcell.NewEventKey(tcell.KeyBacktab, "", tcell.ModNone))
	check(s.Include)
	s.SetFocused(false)
	check(nil)
	s.SetFocused(true)
	check(s.Include)
	r := s.ReplaceInput.GetRect()
	s.HandleEvent(tcell.NewEventMouse(r.X+4, r.Y, tcell.Button1, tcell.ModNone))
	s.HandleEvent(tcell.NewEventMouse(r.X+4, r.Y, tcell.ButtonNone, tcell.ModNone))
	check(s.ReplaceInput)
	s.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone))
	if s.ReplaceInput.Text() != "x" {
		t.Fatal("typing did not reach the focused replacement input")
	}
}

func TestSearchReplaceAllUsesConfiguredIcon(t *testing.T) {
	t.Cleanup(func() { icons.Configure("", nil) })
	s := NewSearchWidget()
	s.Icons = config.IconsNerdFont
	s.SetReplaceMode(true)
	for _, glyph := range []string{"@", "#"} {
		if warnings := icons.Configure(s.Icons, map[string]string{"replace-all": glyph}); len(warnings) != 0 {
			t.Fatal(warnings)
		}
		s.Render(NewRenderSurface(makeGrid(40, 12), Rect{W: 40, H: 12}))
		if got := s.ReplaceInput.Actions[0].Label; got != glyph+"All" {
			t.Fatalf("replace all label = %q", got)
		}
		s.SetReplacementPending(true)
		if got := s.ReplaceInput.Actions[0].Label; got != "Cancel" {
			t.Fatalf("pending label = %q", got)
		}
		s.SetReplacementPending(false)
		if got := s.ReplaceInput.Actions[0].Label; got != glyph+"All" {
			t.Fatalf("restored label = %q", got)
		}
	}
}

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
	seedInputText(s.ReplaceInput, "replacement")
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

func TestSearchPartialResultsRemainVisibleAndClickable(t *testing.T) {
	s := NewSearchWidget()
	s.SetRect(Rect{W: 80, H: 12})
	seedInputText(s.Input, "asdf")
	s.Searching = true
	s.ApplyBatch(&SearchBatch{
		Gen: s.searchGen, Done: true, Error: "search failed: signal: killed",
		Groups: []SearchFileGroup{{
			FilePath: "/repo/fixture.txt", RelPath: "fixture.txt", Expanded: true,
			Matches: []SearchMatch{{
				FilePath: "/repo/fixture.txt", LineNum: 3, ColStart: 6, ColEnd: 10, LineText: "found asdf",
			}},
		}},
	})
	grid := makeGrid(80, 12)
	s.Render(NewRenderSurface(grid, s.GetRect()))
	screen := detailTestText(grid)
	for _, want := range []string{"Results may be incomplete", "search failed: signal: killed", "1 results in 1 files", "fixture.txt", "3: found asdf"} {
		if !strings.Contains(screen, want) {
			t.Errorf("screen missing %q:\n%s", want, screen)
		}
	}
	if s.Searching || strings.Contains(screen, "No results") {
		t.Fatal("failed search with partial results has an incorrect completion state")
	}
	opened := false
	s.OnOpenMatch = func(path string, line, col int) {
		opened = true
		if path != "/repo/fixture.txt" || line != 3 || col != 6 {
			t.Errorf("opened %s:%d:%d, want /repo/fixture.txt:3:6", path, line, col)
		}
	}
	s.HandleEvent(tcell.NewEventMouse(5, s.resultStartY+1, tcell.Button1, tcell.ModNone))
	if !opened {
		t.Fatal("partial result could not be opened")
	}
}

func TestSearchKeepsDiffResultsWhenRipgrepUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	s := NewSearchWidget()
	s.SetWorkDirs([]string{t.TempDir()})
	seedInputText(s.Input, "asdf")
	s.DiffSources = func() []DiffSearchSource {
		return []DiffSearchSource{{TabName: "diff.txt", Lines: []string{"asdf"}}}
	}
	s.PostBatch = s.ApplyBatch
	s.streamSearch(context.Background(), s.searchGen)
	if s.Error != "ripgrep (rg) not found" {
		t.Fatalf("Error = %q, want missing ripgrep error", s.Error)
	}
	if len(s.Groups) != 1 || s.Groups[0].FilePath != "diff.txt" || len(s.Groups[0].Matches) != 1 {
		t.Fatal("diff results were lost when disk search failed")
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
	seedInputText(s.Input, "")

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
			seedInputText(s.ReplaceInput, "next")
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

			wantCollapsed := []rune(icons.Get(mode, icons.ChevronLeft))[0]
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

			wantExpanded := []rune(icons.Get(mode, icons.ChevronDown))[0]
			if got := grid[s.toggleRow()][w-2].Ch; got != wantExpanded {
				t.Fatalf("expanded filter toggle glyph = %q, want %q", got, wantExpanded)
			}
		})
	}

	t.Run("overrides", func(t *testing.T) {
		warnings := icons.Configure(config.IconsNone, map[string]string{
			"chevron-down": "v",
			"chevron-left": "<",
			"arrow-left":   "L",
			"replace":      "R",
		})
		if len(warnings) != 0 {
			t.Fatalf("unexpected configure warnings: %v", warnings)
		}

		s := NewSearchWidget()
		s.showReplace = true
		seedInputText(s.ReplaceInput, "next")
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

		if got := grid[s.toggleRow()][w-2].Ch; got != '<' {
			t.Fatalf("overridden collapsed filter toggle = %q, want '<'", got)
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
		if got := grid[s.toggleRow()][w-2].Ch; got != 'v' {
			t.Fatalf("overridden expanded filter toggle = %q, want 'v'", got)
		}

		seedInputText(s.Input, "old")
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
