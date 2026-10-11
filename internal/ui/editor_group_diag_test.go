package ui

import (
	"fmt"
	"github.com/eugenioenko/ttt/internal/highlight"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func openTabWithFile(t *testing.T, content string) (*EditorGroupWidget, string) {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.txt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	g := NewEditorGroupWidget(nil, 4, false, "extended")
	g.OpenFile(path)
	if g.tabs[g.active].FilePath != path {
		t.Fatalf("expected active tab %s, got %s", path, g.tabs[g.active].FilePath)
	}
	return g, path
}

func TestSetDiagnosticsSourceMerges(t *testing.T) {
	g, path := openTabWithFile(t, "hello world")

	g.SetDiagnosticsSource("lsp", path, []Diagnostic{
		{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 5, Severity: DiagError},
	})
	g.SetDiagnosticsSource("plugin:x", path, []Diagnostic{
		{StartLine: 0, StartCol: 6, EndLine: 0, EndCol: 11, Severity: DiagWarning},
	})

	if got := len(g.tabs[g.active].Diagnostics); got != 2 {
		t.Fatalf("expected 2 merged diagnostics, got %d", got)
	}
	if got := len(g.Editor.Diagnostics); got != 2 {
		t.Fatalf("expected active editor to mirror 2 diagnostics, got %d", got)
	}
}

func TestClearDiagnosticsSourceRemovesOnlyThatSource(t *testing.T) {
	g, path := openTabWithFile(t, "hello world")

	g.SetDiagnosticsSource("lsp", path, []Diagnostic{
		{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 5, Severity: DiagError},
	})
	g.SetDiagnosticsSource("plugin:x", path, []Diagnostic{
		{StartLine: 0, StartCol: 6, EndLine: 0, EndCol: 11, Severity: DiagWarning},
	})

	g.ClearDiagnosticsSource("plugin:x")

	diags := g.tabs[g.active].Diagnostics
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic after clearing plugin source, got %d", len(diags))
	}
	if diags[0].Severity != DiagError {
		t.Errorf("expected remaining diagnostic to be the LSP error, got severity %d", diags[0].Severity)
	}
}

func TestSetDiagnosticsWrapsLspSource(t *testing.T) {
	g, path := openTabWithFile(t, "hello world")

	g.SetDiagnostics(path, []Diagnostic{
		{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 5, Severity: DiagError},
	})
	// SetDiagnostics must funnel through the "lsp" source so ClearDiagnosticsSource
	// can later remove it.
	g.ClearDiagnosticsSource("lsp")
	if got := len(g.tabs[g.active].Diagnostics); got != 0 {
		t.Fatalf("expected SetDiagnostics to use the lsp source, got %d after clear", got)
	}
}

func TestCustomStyleFlowsToDiagStyleAt(t *testing.T) {
	g, path := openTabWithFile(t, "hello world")

	custom := term.StyleSyntaxKeyword
	g.SetDiagnosticsSource("plugin:x", path, []Diagnostic{
		{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 5, Severity: DiagWarning, Style: custom},
	})

	if got := g.Editor.diagStyleAt(0, 2); got != custom {
		t.Errorf("expected custom style %v at (0,2), got %v", custom, got)
	}
}

func TestSeverityStyleWhenNoCustomStyle(t *testing.T) {
	g, path := openTabWithFile(t, "hello world")

	g.SetDiagnosticsSource("plugin:x", path, []Diagnostic{
		{StartLine: 0, StartCol: 0, EndLine: 0, EndCol: 5, Severity: DiagWarning},
	})

	if got := g.Editor.diagStyleAt(0, 2); got != term.StyleDiagWarning {
		t.Errorf("expected StyleDiagWarning at (0,2), got %v", got)
	}
}

func TestInlineDiffDropsDeferredHighlightingWhenItEnds(t *testing.T) {
	highlight.SetStepScheduler(func() {})
	t.Cleanup(func() { highlight.SetStepScheduler(nil) })
	var file strings.Builder
	var base []string
	for i := 0; i < 3000; i++ {
		fmt.Fprintf(&file, "var v%d = %d\n", i, i)
		base = append(base, fmt.Sprintf("var v%d = %d", i, i))
	}
	base[2900] = "var changed = 1"
	owned := func(d *DiffEditorWidget) []*highlight.Highlighter {
		hs := []*highlight.Highlighter{d.docSyntax.old.hl, d.docSyntax.new.hl}
		for _, p := range d.panes() {
			if p != d.live {
				hs = append(hs, p.Highlighter)
			}
		}
		return hs
	}
	for _, end := range []string{"close tab", "disable", "re-enable"} {
		path := filepath.Join(t.TempDir(), "big.go")
		if err := os.WriteFile(path, []byte(file.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		g := NewEditorGroupWidget(nil, 4, true, "compact")
		g.SyntaxHighlight = true
		g.OpenFile(path)
		if !g.EnableInlineDiff(path, base) {
			t.Fatalf("%s: diff mode did not turn on", end)
		}
		d := g.tabs[g.active].Diff
		d.SetMode(DiffModeSplit)
		g.Editor.Cursor.Line = 2950
		g.Editor.EnsureCursorVisible()
		g.SetRect(Rect{W: 120, H: 30})
		g.Render(NewRenderSurface(makeGrid(120, 30), Rect{W: 120, H: 30}))
		deferred := false
		for _, h := range owned(d) {
			deferred = deferred || h.HasDeferredLines()
		}
		if !deferred {
			t.Fatalf("%s: fixture: rendering deep in the diff should defer its HEAD side", end)
		}
		switch end {
		case "close tab":
			g.CloseTab()
		case "disable":
			g.DisableInlineDiff(path)
		case "re-enable":
			g.EnableInlineDiff(path, base)
		}
		for _, h := range owned(d) {
			if h.HasDeferredLines() {
				t.Errorf("%s: the old diff still waits for highlighting steps", end)
			}
		}
	}
}

func TestEditableSplitSharesSingleLineHighlighting(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "main.go")
	if err := os.WriteFile(path, []byte("package main\n\nfunc B() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	g := NewEditorGroupWidget(nil, 4, true, "extended")
	g.SyntaxHighlight = true
	g.DiffMode = DiffModeSplit
	g.OpenFile(path)
	if !g.EnableInlineDiff(path, []string{"package main", "", "func b() {}"}) {
		t.Fatal("diff mode did not turn on")
	}
	grid := makeGrid(80, 10)
	g.SetRect(Rect{X: 0, Y: 0, W: 80, H: 10})
	g.Render(NewRenderSurface(grid, Rect{X: 0, Y: 0, W: 80, H: 10}))
	d := g.tabs[g.active].Diff
	live := g.Editor.Highlighter
	if live == nil {
		t.Fatal("the file has no highlighter")
	}
	for _, p := range []*EditorPaneWidget{d.left} {
		if p.Highlighter != nil && !p.Highlighter.SharesSingleLinesWith(live) {
			t.Error("the HEAD pane highlights unchanged lines separately from the file")
		}
	}
	if hl := d.docSyntax.old.hl; hl != nil && !hl.SharesSingleLinesWith(live) {
		t.Error("the HEAD side highlights unchanged lines separately from the file")
	}
	if d.left.Highlighter == nil && d.docSyntax.old.hl == nil {
		t.Fatal("the HEAD side was never highlighted")
	}
}
