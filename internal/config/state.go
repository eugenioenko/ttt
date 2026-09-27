package config

import (
	"encoding/json"
	"os"
)

type State struct {
	PanelPosition       string   `json:"panelPosition,omitempty"`
	SidebarWidth        int      `json:"sidebarWidth,omitempty"`
	SidebarHidden       bool     `json:"sidebarHidden,omitempty"`
	SidebarPanelOrder   []string `json:"sidebarPanelOrder,omitempty"`
	CommitHistoryHeight int      `json:"commitHistoryHeight,omitempty"`
	PanelHeight         int      `json:"panelHeight,omitempty"`
	PanelWidth          int      `json:"panelWidth,omitempty"`
}

func LoadState() State {
	var s State
	paths := configPaths()
	if data, err := readFirst(paths, "state.json"); err == nil {
		if json.Unmarshal(data, &s) != nil {
			return State{}
		}
	}
	return s
}

func SaveState(s State) error {
	path := ConfigFilePath("state.json")
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}
