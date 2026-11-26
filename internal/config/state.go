package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	PanelPosition       string   `json:"panelPosition,omitempty"`
	SidebarWidth        int      `json:"sidebarWidth,omitempty"`
	SidebarHidden       bool     `json:"sidebarHidden,omitempty"`
	SidebarPanelOrder   []string `json:"sidebarPanelOrder,omitempty"`
	CommitHistoryHeight int      `json:"commitHistoryHeight,omitempty"`
	PanelHeight         int      `json:"panelHeight,omitempty"`
	PanelWidth          int      `json:"panelWidth,omitempty"`
	RecentFolders       []string `json:"recentFolders,omitempty"`
	RecentCommands      []string `json:"recentCommands,omitempty"`
	LastUpdateCheckAt   string   `json:"lastUpdateCheckAt,omitempty"`
}

func LoadState() State {
	s, _ := ReadState()
	return s
}

func ReadState() (State, error) {
	s, _, err := findState(configPaths())
	return s, err
}

func findState(dirs []string) (State, string, error) {
	for _, dir := range dirs {
		path := filepath.Join(dir, "state.json")
		s, err := readState(path)
		if !errors.Is(err, os.ErrNotExist) {
			return s, path, err
		}
	}
	return State{}, "", nil
}

func readState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}

func SaveState(s State) error {
	return saveState(s, configPaths())
}

func saveState(s State, dirs []string) error {
	_, path, err := findState(dirs)
	if err != nil {
		return err
	}
	if path == "" {
		path = ConfigFilePath("state.json")
		if _, err := readState(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
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
