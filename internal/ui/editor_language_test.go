package ui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/term"
)

func TestLanguageSelectionSurvivesTabAndPathChanges(t *testing.T) {
	for _, selection := range []string{"source.shell", highlight.PlainText} {
		t.Run(selection, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "deploy")
			g := newRenameGroup(t, path, "#!/usr/bin/env python3\n# comment\nprint('hi')\n")
			t.Cleanup(func() { g.CloseAllTabs() })
			g.CommitActiveTab()
			second := filepath.Join(dir, "main.go")
			if err := os.WriteFile(second, []byte("package main\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := g.OpenFile(second); err != nil {
				t.Fatal(err)
			}
			g.SwitchTab(0)
			opened, closed := 0, 0
			g.OnFileOpen = func(string, string, []string) { opened++ }
			g.OnFileClose = func(string, string) { closed++ }
			g.SetActiveLanguage(selection)
			if opened != 0 || closed != 0 || g.ActiveBuffer().Dirty || g.ActiveLanguage() != "Python" {
				t.Fatal("manual selection changed document lifecycle or state")
			}
			check := func() {
				t.Helper()
				if g.ActiveLanguageSelection() != selection {
					t.Fatal("selection was reset")
				}
				h := g.Editor.Highlighter
				if selection == highlight.PlainText {
					if h != nil || g.Editor.autoClosingSyntax() != nil {
						t.Fatal("Plain Text acquired syntax")
					}
				} else if h == nil || h.Language() != "Shell" {
					t.Fatal("Shell selection lost")
				} else {
					spans := h.HighlightLineAt(g.ActiveBuffer().Lines, 1)
					if len(spans) == 0 || spans[0].Style != term.StyleSyntaxComment {
						t.Fatalf("comment spans = %v", spans)
					}
				}
			}
			check()
			g.SwitchTab(1)
			if g.Editor.Highlighter.Language() != "Go" || g.ActiveLanguageSelection() != "" {
				t.Fatal("selection leaked into another tab")
			}
			g.SwitchTab(0)
			check()
			if !g.SaveAs(filepath.Join(dir, "saved.go")) {
				t.Fatal("SaveAs failed")
			}
			check()
			if !g.ReloadFile(g.ActiveFilePath()) {
				t.Fatal("ReloadFile failed")
			}
			check()
			if !g.RenamePath(g.ActiveFilePath(), filepath.Join(dir, "renamed.py")) {
				t.Fatal("RenamePath failed")
			}
			check()
			g.RefreshHighlighters()
			check()
			if docs := g.OpenDocuments(); len(docs) != 2 || docs[0].Language != "Python" {
				t.Fatalf("document language changed with highlighting: %+v", docs)
			}
			g.SetActiveLanguage("")
			if g.Editor.Highlighter == nil || g.Editor.Highlighter.Language() != "Python" {
				t.Fatal("Auto Detect did not restore Python")
			}
		})
	}
}

func TestLanguageSelectionTextTargetsAndMissingGrammar(t *testing.T) {
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.SyntaxHighlight = true
	t.Cleanup(func() { g.CloseAllTabs(); highlight.SetExternalGrammars(nil) })
	g.activeTab().FilePath = ""
	g.SetActiveLanguage("source.foo")
	if g.Editor.Highlighter != nil {
		t.Fatal("unavailable grammar resolved")
	}
	grammar := filepath.Join(t.TempDir(), "foo.json")
	if err := os.WriteFile(grammar, []byte(`{"scopeName":"source.foo","name":"Foo","patterns":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	highlight.SetExternalGrammars([]highlight.ExternalGrammar{{Path: grammar}})
	g.RefreshHighlighters()
	if g.Editor.Highlighter == nil || g.Editor.Highlighter.Language() != "Foo" {
		t.Fatal("untitled selection was not restored when grammar became available")
	}
	g.SyntaxHighlight = false
	g.SetActiveLanguage("source.shell")
	if g.Editor.Highlighter != nil || g.ActiveLanguageSelection() != "source.shell" {
		t.Fatal("disabled highlighting did not retain the selection")
	}
	g.SyntaxHighlight = true
	g.OpenBufferReadOnly("readonly", "", []string{"# comment"})
	g.SetActiveLanguage("source.shell")
	if g.Editor.Highlighter == nil || g.Editor.Highlighter.Language() != "Shell" {
		t.Fatal("read-only selection failed")
	}
	g.OpenDiff("main.go", diff.FileDiff{}, nil, nil, false)
	g.SetActiveLanguage("source.shell")
	if g.ActiveLanguageSelection() != "" || g.ActiveLanguage() != "" {
		t.Fatal("non-text tab accepted a selection")
	}
}
