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

	os.WriteFile(filepath.Join(dir, "state.json"), []byte("{corrupt!"), 0644)
	s := LoadState()
	if s.PanelPosition != "" || s.SidebarWidth != 0 {
		t.Fatalf("corrupt state.json returned non-zero state: %+v", s)
	}
}

func TestLoadStatePartialDecodeReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	OverrideConfigDir = dir
	t.Cleanup(func() { OverrideConfigDir = "" })

	os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"panelPosition":"right","sidebarWidth":"bad"}`), 0644)
	s := LoadState()
	if s.PanelPosition != "" || s.SidebarWidth != 0 {
		t.Fatalf("partial decode returned non-zero state: %+v", s)
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
}
