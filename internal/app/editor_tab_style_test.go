package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
)

func TestEditorTabBarStyleAppliesToExistingAndNewSplits(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(a.CloseTasks)
	a.SplitEditor()
	s := *a.Settings
	s.Editor.TabBarStyle = "hidden"
	a.ApplySettings(s)
	a.SplitEditorDown()
	for _, group := range a.editorGroups() {
		if group.TabBar.Height() != 1 {
			t.Fatal("style did not apply to an existing or new split")
		}
		group.NewFile()
		group.PrevTab()
		if group.ActiveTabIndex() != 0 || group.TabCount() != 2 {
			t.Fatal("hiding the strip disabled tab commands")
		}
	}
	s.Editor.TabBarStyle = "extended"
	a.ApplySettings(s)
	for _, group := range a.editorGroups() {
		if group.TabBar.Height() != 3 {
			t.Fatal("restoring the strip did not apply to all splits")
		}
	}
	found := false
	for _, category := range settingsCategories() {
		for _, field := range category.Fields {
			if field.Label == i18n.Text("settings.tabBarStyle") {
				found = category.Title == i18n.Text("settings.appearance") && len(field.Options(a.Settings)) == 3
			}
		}
	}
	if !found {
		t.Fatal("Appearance settings does not expose all tab bar styles")
	}
}
