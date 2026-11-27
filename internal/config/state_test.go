package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLoadStateMissingFileReturnsDefaults(t *testing.T) {
	OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { OverrideConfigDir = "" })

	s := LoadState()
	if s.PanelPosition != "" || s.SidebarWidth != 0 || len(s.SidebarPanelOrder) != 0 || s.CommitHistoryHeight != 0 {
		t.Fatalf("missing state.json returned non-zero state: %+v", s)
	}
}

func TestLoadStateCorruptFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	OverrideConfigDir = dir
	t.Cleanup(func() { OverrideConfigDir = "" })

	requireNoError(t, os.WriteFile(filepath.Join(dir, "state.json"), []byte("{corrupt!"), 0644))
	s := LoadState()
	if s.PanelPosition != "" || s.SidebarWidth != 0 {
		t.Fatalf("corrupt state.json returned non-zero state: %+v", s)
	}
}

func TestLoadStatePartialDecodeReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	OverrideConfigDir = dir
	t.Cleanup(func() { OverrideConfigDir = "" })

	requireNoError(t, os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"panelPosition":"right","sidebarWidth":"bad"}`), 0644))
	s := LoadState()
	if s.PanelPosition != "" || s.SidebarWidth != 0 {
		t.Fatalf("partial decode returned non-zero state: %+v", s)
	}
}

func TestReadStateErrorsAndOverwrite(t *testing.T) {
	dir := t.TempDir()
	OverrideConfigDir = dir
	t.Cleanup(func() { OverrideConfigDir = "" })

	s, err := ReadState()
	if err != nil || s.SidebarWidth != 0 {
		t.Fatalf("missing state.json = (%+v, %v), want zero state and nil error", s, err)
	}

	path := filepath.Join(dir, "state.json")
	for _, invalid := range []string{
		`{"recentFolders":["project"],"sidebarWidth":`,
		`{"panelPosition":"right","sidebarWidth":"bad"}`,
	} {
		requireNoError(t, os.WriteFile(path, []byte(invalid), 0o644))
		s, err = ReadState()
		if err == nil || !strings.Contains(err.Error(), path) {
			t.Fatalf("ReadState(%s) err = %v, want error mentioning %q", invalid, err, path)
		}
		if s.PanelPosition != "" || s.SidebarWidth != 0 || len(s.RecentFolders) != 0 {
			t.Fatalf("ReadState(%s) returned partial state: %+v", invalid, s)
		}
	}

	requireNoError(t, SaveState(State{SidebarWidth: 40}))
	s, err = ReadState()
	if err != nil {
		t.Fatalf("ReadState after SaveState overwrite: %v", err)
	}
	if s.SidebarWidth != 40 {
		t.Errorf("SidebarWidth = %d, want 40", s.SidebarWidth)
	}
}

func TestReadStateDoesNotFallBackAfterError(t *testing.T) {
	first := t.TempDir()
	second := t.TempDir()
	requireNoError(t, os.WriteFile(filepath.Join(first, "state.json"), []byte(`{"sidebarWidth":`), 0o644))
	requireNoError(t, os.WriteFile(filepath.Join(second, "state.json"), []byte(`{"sidebarWidth":40}`), 0o644))

	if s, err := readStateFrom([]string{first, second}); err == nil || s.SidebarWidth != 0 {
		t.Fatalf("readStateFrom fell back past corrupt state: (%+v, %v)", s, err)
	}

	missing := filepath.Join(t.TempDir(), "missing")
	s, err := readStateFrom([]string{missing, second})
	if err != nil || s.SidebarWidth != 40 {
		t.Fatalf("readStateFrom with missing first dir = (%+v, %v), want SidebarWidth 40", s, err)
	}
}

func TestSaveAndLoadStateRoundTrip(t *testing.T) {
	OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { OverrideConfigDir = "" })

	want := State{
		PanelPosition:       "right",
		SidebarWidth:        30,
		SidebarPanelOrder:   []string{"changes", "explorer", "search"},
		CommitHistoryHeight: 15,
		RecentCommands:      []string{"sidebar.toggle", "file.save"},
	}
	if err := SaveState(want); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	got := LoadState()
	if got.PanelPosition != want.PanelPosition {
		t.Errorf("PanelPosition = %q, want %q", got.PanelPosition, want.PanelPosition)
	}
	if got.SidebarWidth != want.SidebarWidth {
		t.Errorf("SidebarWidth = %d, want %d", got.SidebarWidth, want.SidebarWidth)
	}
	if !slices.Equal(got.SidebarPanelOrder, want.SidebarPanelOrder) {
		t.Errorf("SidebarPanelOrder = %v, want %v", got.SidebarPanelOrder, want.SidebarPanelOrder)
	}
	if got.CommitHistoryHeight != want.CommitHistoryHeight {
		t.Errorf("CommitHistoryHeight = %d, want %d", got.CommitHistoryHeight, want.CommitHistoryHeight)
	}
	if !slices.Equal(got.RecentCommands, want.RecentCommands) {
		t.Errorf("RecentCommands = %v, want %v", got.RecentCommands, want.RecentCommands)
	}
}

func TestSavedBreakpointConfirmationRoundTrip(t *testing.T) {
	dir := t.TempDir()
	OverrideConfigDir = dir
	t.Cleanup(func() { OverrideConfigDir = "" })
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"breakpoints":{"project":{"main.c":[2,{"line":7,"needsConfirmation":true}]}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := ReadState()
	if err != nil {
		t.Fatal(err)
	}
	if err = SaveState(s); err != nil {
		t.Fatal(err)
	}
	points := LoadState().Breakpoints["project"]["main.c"]
	if len(points) != 2 || points[0] != (SavedBreakpoint{Line: 2}) || points[1] != (SavedBreakpoint{Line: 7, NeedsConfirmation: true}) {
		t.Fatalf("restored %+v", points)
	}
}
