package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type SavedBreakpoint struct {
	Line              int  `json:"line"`
	NeedsConfirmation bool `json:"needsConfirmation,omitempty"`
}

func (b *SavedBreakpoint) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] != '{' {
		return json.Unmarshal(data, &b.Line)
	}
	type entry SavedBreakpoint
	return json.Unmarshal(data, (*entry)(b))
}

type State struct {
	Breakpoints         map[string]map[string][]SavedBreakpoint `json:"breakpoints,omitempty"`
	PanelPosition       string                                  `json:"panelPosition,omitempty"`
	SidebarWidth        int                                     `json:"sidebarWidth,omitempty"`
	SidebarHidden       bool                                    `json:"sidebarHidden,omitempty"`
	SidebarPanelOrder   []string                                `json:"sidebarPanelOrder,omitempty"`
	CommitHistoryHeight int                                     `json:"commitHistoryHeight,omitempty"`
	PanelHeight         int                                     `json:"panelHeight,omitempty"`
	PanelWidth          int                                     `json:"panelWidth,omitempty"`
	RecentFolders       []string                                `json:"recentFolders,omitempty"`
	RecentCommands      []string                                `json:"recentCommands,omitempty"`
	LastUpdateCheckAt   string                                  `json:"lastUpdateCheckAt,omitempty"`
}

func LoadState() State {
	s, _ := ReadState()
	return s
}

func ReadState() (State, error) {
	return readStateFrom(configPaths())
}

func readStateFrom(dirs []string) (State, error) {
	for _, dir := range dirs {
		path := filepath.Join(dir, "state.json")
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return State{}, err
		}
		var s State
		if err := json.Unmarshal(data, &s); err != nil {
			return State{}, fmt.Errorf("%s: %w", path, err)
		}
		return s, nil
	}
	return State{}, nil
}

func SaveState(s State) error {
	path := ConfigFilePath("state.json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return writeFileAtomically(path, data, 0644)
}

func (s State) LastUpdateCheckTime() time.Time {
	if s.LastUpdateCheckAt == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, s.LastUpdateCheckAt)
	if err != nil {
		parsed, err = time.Parse(time.RFC3339, s.LastUpdateCheckAt)
		if err != nil {
			return time.Time{}
		}
	}
	return parsed
}

func (s *State) SetLastUpdateCheckTime(t time.Time) {
	if t.IsZero() {
		s.LastUpdateCheckAt = ""
		return
	}
	s.LastUpdateCheckAt = t.UTC().Format(time.RFC3339Nano)
}
