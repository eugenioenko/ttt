package config

import (
	"bytes"
	"encoding/json"
)

const MaxEditorGroups = 16

type EditorLayoutState struct {
	Direction string             `json:"direction,omitempty"`
	Ratio     float64            `json:"ratio,omitempty"`
	First     *EditorLayoutState `json:"first,omitempty"`
	Second    *EditorLayoutState `json:"second,omitempty"`
	Tabs      []EditorTabState   `json:"tabs,omitempty"`
	Focused   bool               `json:"focused,omitempty"`
}

type EditorSessionState struct {
	Version int                `json:"version"`
	Tabs    []EditorTabState   `json:"tabs,omitempty"`
	Layout  *EditorLayoutState `json:"layout,omitempty"`
}

func LoadEditorWorkspaceSession(key string) (EditorSessionState, error) {
	data, err := readEditorSession(key)
	if err != nil || len(data) == 0 {
		return EditorSessionState{}, err
	}
	var session EditorSessionState
	if trimmed := bytes.TrimSpace(data); len(trimmed) > 0 && trimmed[0] == '[' {
		err = json.Unmarshal(data, &session.Tabs)
	} else {
		err = json.Unmarshal(data, &session)
		if err == nil && session.Version != 1 {
			return EditorSessionState{}, nil
		}
	}
	if err != nil {
		return EditorSessionState{}, err
	}
	session.Tabs = boundedSessionTabs(session.Tabs)
	session.Layout = boundedEditorLayout(session.Layout)
	return session, nil
}

func LoadEditorSession(key string) ([]EditorTabState, error) {
	session, err := LoadEditorWorkspaceSession(key)
	if err != nil || session.Layout == nil {
		return session.Tabs, err
	}
	var tabs []EditorTabState
	seen := map[string]int{}
	var collect func(*EditorLayoutState)
	collect = func(node *EditorLayoutState) {
		if node.First != nil {
			collect(node.First)
			collect(node.Second)
			return
		}
		for _, tab := range node.Tabs {
			tab.Active = tab.Active && node.Focused
			if i, exists := seen[tab.Path]; exists {
				pinned := tabs[i].Pinned || tab.Pinned
				if tab.Active {
					tabs[i] = tab
				}
				tabs[i].Pinned = pinned
			} else {
				seen[tab.Path] = len(tabs)
				tabs = append(tabs, tab)
			}
		}
	}
	collect(session.Layout)
	return tabs, nil
}

func SaveEditorWorkspaceSession(key string, session EditorSessionState) error {
	session.Layout = boundedEditorLayout(session.Layout)
	if session.Layout == nil {
		return SaveEditorSession(key, session.Tabs)
	}
	if session.Layout.First == nil && len(session.Layout.Tabs) == 0 {
		return SaveEditorSession(key, nil)
	}
	session.Version, session.Tabs = 1, nil
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return writeEditorSession(key, data)
}

func boundedEditorLayout(layout *EditorLayoutState) *EditorLayoutState {
	groups := 0
	var leaves []*EditorLayoutState
	var copyNode func(*EditorLayoutState, int) *EditorLayoutState
	copyNode = func(node *EditorLayoutState, depth int) *EditorLayoutState {
		if node == nil || depth >= MaxEditorGroups {
			return nil
		}
		if node.First == nil && node.Second == nil && node.Direction == "" {
			groups++
			if groups > MaxEditorGroups {
				return nil
			}
			leaf := &EditorLayoutState{Tabs: node.Tabs, Focused: node.Focused}
			leaves = append(leaves, leaf)
			return leaf
		}
		if node.Direction != "right" && node.Direction != "down" {
			return nil
		}
		first, second := copyNode(node.First, depth+1), copyNode(node.Second, depth+1)
		if first == nil || second == nil {
			return nil
		}
		return &EditorLayoutState{Direction: node.Direction, Ratio: node.Ratio, First: first, Second: second}
	}
	result := copyNode(layout, 0)
	if result == nil {
		return nil
	}
	active, total := -1, 0
	for _, leaf := range leaves {
		for _, tab := range leaf.Tabs {
			if leaf.Focused && tab.Active {
				active = total
			}
			total++
		}
	}
	limit, index := MaxSessionTabs, 0
	if active >= limit {
		limit--
	}
	for _, leaf := range leaves {
		tabs := make([]EditorTabState, 0, min(len(leaf.Tabs), MaxSessionTabs))
		for _, tab := range leaf.Tabs {
			if index < limit || index == active {
				tabs = append(tabs, tab)
			}
			index++
		}
		leaf.Tabs = tabs
	}
	return result
}
