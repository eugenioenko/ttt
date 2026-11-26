package config

import (
	"os"
	"path/filepath"
	"slices"
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

func TestSaveStatePreservesCorruptFile(t *testing.T) {
	OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { OverrideConfigDir = "" })
	path := filepath.Join(OverrideConfigDir, "state.json")
	const corrupt = `{"recentFolders":["project"],"sidebarWidth":`
	requireNoError(t, os.WriteFile(path, []byte(corrupt), 0o644))

	if err := SaveState(State{SidebarWidth: 40}); err == nil {
		t.Error("SaveState succeeded with corrupt state.json on disk")
	}
	got, err := os.ReadFile(path)
	requireNoError(t, err)
	if string(got) != corrupt {
		t.Errorf("saved over corrupt file with %q", got)
	}

	requireNoError(t, os.WriteFile(path, []byte("{}"), 0o644))
	requireNoError(t, SaveState(State{SidebarWidth: 40}))
	if got := LoadState().SidebarWidth; got != 40 {
		t.Errorf("SidebarWidth after repairing state.json = %d, want 40", got)
	}
}

func TestSaveStateDeclinesFallbackAfterLookupError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	const original = `{"sidebarWidth":40}`
	requireNoError(t, os.WriteFile(path, []byte(original), 0o644))

	unreadable := filepath.Join(t.TempDir(), "invalid\x00directory")
	if err := saveState(State{SidebarWidth: 20}, []string{unreadable, dir}); err == nil {
		t.Error("SaveState skipped the first candidate's lookup error")
	}
	got, err := os.ReadFile(path)
	requireNoError(t, err)
	if string(got) != original {
		t.Errorf("the fallback file was overwritten with %q", got)
	}

	absent := filepath.Join(t.TempDir(), "missing")
	requireNoError(t, saveState(State{SidebarWidth: 20}, []string{absent, dir}))
	saved, err := readState(path)
	requireNoError(t, err)
	if saved.SidebarWidth != 20 {
		t.Errorf("a missing candidate blocked the write: %+v", saved)
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
