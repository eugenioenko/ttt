package config

import (
	"fmt"
	"testing"
)

func TestEditorWorkspaceSessionReadsLegacyTabsAndBoundsViews(t *testing.T) {
	withTempConfigDir(t)
	if err := SaveEditorSession("legacy", []EditorTabState{{Path: "/old.go", Active: true}}); err != nil {
		t.Fatal(err)
	}
	legacy, err := LoadEditorWorkspaceSession("legacy")
	if err != nil || legacy.Layout != nil || len(legacy.Tabs) != 1 || legacy.Tabs[0].Path != "/old.go" {
		t.Fatalf("legacy session = %+v, %v", legacy, err)
	}
	left := make([]EditorTabState, MaxSessionTabs)
	for i := range left {
		left[i].Path = fmt.Sprintf("/file-%d.go", i)
	}
	layout := &EditorLayoutState{Direction: "right", Ratio: 0.35,
		First: &EditorLayoutState{Tabs: left}, Second: &EditorLayoutState{Focused: true,
			Tabs: []EditorTabState{{Path: "/active.go", Active: true}}}}
	if err := SaveEditorWorkspaceSession("split", EditorSessionState{Layout: layout}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadEditorWorkspaceSession("split")
	if err != nil || got.Layout == nil || got.Layout.Ratio != 0.35 || len(got.Layout.First.Tabs) != MaxSessionTabs-1 || len(got.Layout.Second.Tabs) != 1 {
		t.Fatalf("bounded split = %+v, %v", got, err)
	}
	if got.Layout.Second.Tabs[0].Path != "/active.go" || !got.Layout.Second.Focused || len(layout.First.Tabs) != MaxSessionTabs {
		t.Fatal("view cap lost the focused document or mutated the snapshot")
	}
	flat, err := LoadEditorSession("split")
	if err != nil || len(flat) != MaxSessionTabs || !flat[len(flat)-1].Active {
		t.Fatal("flat reader lost bounded split tabs")
	}
	if err := SaveEditorWorkspaceSession("split", EditorSessionState{Layout: &EditorLayoutState{}}); err != nil {
		t.Fatal(err)
	}
	if cleared, err := LoadEditorWorkspaceSession("split"); err != nil || cleared.Layout != nil || len(cleared.Tabs) != 0 {
		t.Fatal("a single empty editor did not clear its saved state")
	}
}

func TestEditorWorkspaceSessionRejectsInvalidLayouts(t *testing.T) {
	leaf := func() *EditorLayoutState { return &EditorLayoutState{} }
	branch := func(a, b *EditorLayoutState) *EditorLayoutState {
		return &EditorLayoutState{Direction: "right", First: a, Second: b}
	}
	for name, layout := range map[string]*EditorLayoutState{
		"direction":     {Direction: "invalid", First: leaf(), Second: leaf()},
		"missing child": {Direction: "right", First: leaf()},
		"depth":         branch(leaf(), branch(leaf(), branch(leaf(), branch(leaf(), leaf())))),
		"groups":        branch(branch(leaf(), leaf()), branch(branch(leaf(), leaf()), leaf())),
	} {
		t.Run(name, func(t *testing.T) {
			if boundedEditorLayout(layout) != nil {
				t.Fatal("invalid layout was accepted")
			}
		})
	}
}
