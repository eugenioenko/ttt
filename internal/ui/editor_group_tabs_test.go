package ui

import (
	"testing"
)

func TestOpenPluginTabReplacementClosesPreviousContent(t *testing.T) {
	group := NewEditorGroupWidget(nil, 4, true, "relative")
	closed := false
	notified := ""
	group.OnContentTabClose = func(id string) { notified = id }
	first := NewCommitDetailWidget("/repo", "ref", "abcdef0", false)
	first.OnClose = func() { closed = true }
	group.OpenPluginTab("commit", "Commit", first)
	group.OpenPluginTab("commit", "Replacement", NewCommitDetailWidget("/repo", "other", "1234567", false))
	if !closed {
		t.Fatal("replacing a content tab did not close its previous incarnation")
	}
	if notified != "commit" {
		t.Fatalf("replacement close notification = %q, want commit", notified)
	}
}

func TestPluginTabKeyIsSeparateFromFilePath(t *testing.T) {
	group := NewEditorGroupWidget(nil, 4, true, "relative")
	group.OpenBufferReadOnly("Lookalike", "plugin:p:a", []string{"text"})
	before := len(group.tabs)

	group.OpenPluginTab("plugin:p:a", "Page", NewCommitDetailWidget("/repo", "ref", "abcdef0", false))
	if len(group.tabs) != before+1 {
		t.Fatalf("OpenPluginTab reused the buffer at its key: tabs = %d, want %d", len(group.tabs), before+1)
	}
	if !group.ClosePluginTab("plugin:p:a") || len(group.tabs) != before {
		t.Fatalf("ClosePluginTab did not close the plugin tab: tabs = %d", len(group.tabs))
	}
	if group.ClosePluginTab("plugin:p:a") || group.ClosePluginTab("") {
		t.Fatal("ClosePluginTab closed a tab that has no plugin key")
	}
	if len(group.tabs) != before || group.tabs[len(group.tabs)-1].FilePath != "plugin:p:a" {
		t.Fatal("the read-only buffer was closed")
	}
}

type focusProbeWidget struct {
	mockWidget
	focused bool
}

func (f *focusProbeWidget) SetFocused(focused bool) { f.focused = focused }

func closeAllButPinnedA(g *EditorGroupWidget) {
	for i, tab := range g.tabs {
		if tab.PluginKey == "a" {
			g.SwitchTab(i)
		}
	}
	g.TogglePinTab()
	g.SwitchTab(len(g.tabs) - 1)
	g.CloseAllTabs()
}

func TestClosingATabFocusesTheContentTabThatBecomesActive(t *testing.T) {
	closers := map[string]func(g *EditorGroupWidget){
		"close tab":        func(g *EditorGroupWidget) { g.CloseTab() },
		"close plugin tab": func(g *EditorGroupWidget) { g.ClosePluginTab("b") },
		"close all tabs":   closeAllButPinnedA,
	}
	for name, closeTabs := range closers {
		t.Run(name, func(t *testing.T) {
			g := NewEditorGroupWidget(nil, 4, false, "extended")
			g.SetFocused(true)
			a, b := &focusProbeWidget{}, &focusProbeWidget{}
			g.OpenPluginTab("a", "A", a)
			g.OpenPluginTab("b", "B", b)
			if a.focused || !b.focused {
				t.Fatalf("before close: a.focused=%v b.focused=%v, want false true", a.focused, b.focused)
			}
			closeTabs(g)
			if !a.focused {
				t.Fatal("the tab that became active was not focused")
			}
		})
	}
}

func TestCloseAllUnfocusesAPinnedTabThatIsNoLongerActive(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.SetFocused(true)
	a, b := &focusProbeWidget{}, &focusProbeWidget{}
	g.OpenPluginTab("a", "A", a)
	g.TogglePinTab()
	g.OpenPluginTab("b", "B", b)
	g.TogglePinTab()
	g.OpenPluginTab("c", "C", &focusProbeWidget{})
	for i, tab := range g.tabs {
		if tab.PluginKey == "b" {
			g.SwitchTab(i)
		}
	}
	g.CloseAllTabs()
	if g.PluginTabKey(g.active) != "a" || !a.focused || b.focused {
		t.Fatalf("active=%q a.focused=%v b.focused=%v, want a true false", g.PluginTabKey(g.active), a.focused, b.focused)
	}
}

func TestClosingATabKeepsContentUnfocusedWhileTheGroupIsUnfocused(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	a := &focusProbeWidget{}
	g.OpenPluginTab("a", "A", a)
	g.OpenPluginTab("b", "B", &focusProbeWidget{})
	g.CloseTab()
	if a.focused {
		t.Fatal("content was focused although the editor group is not")
	}
}

func TestClosePluginTabLeftOfActiveKeepsActiveTab(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	for _, key := range []string{"a", "b", "c"} {
		g.OpenPluginTab(key, key, NewCommitDetailWidget("/repo", key, "abcdef0", false))
	}
	for i, tab := range g.tabs {
		if tab.PluginKey == "b" {
			g.SwitchTab(i)
		}
	}
	g.ClosePluginTab("a")
	if got := g.PluginTabKey(g.active); got != "b" {
		t.Fatalf("active tab after closing a tab to its left = %q, want b", got)
	}
}
