package ui

import "github.com/eugenioenko/ttt/internal/term"

type DiagnosticSeverity int

const (
	DiagError       DiagnosticSeverity = 1
	DiagWarning     DiagnosticSeverity = 2
	DiagInformation DiagnosticSeverity = 3
	DiagHint        DiagnosticSeverity = 4
)

type Diagnostic struct {
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	Severity  DiagnosticSeverity
	Style     term.Style
	Message   string
	Source    string
	Code      any
}

// SetDiagnostics replaces the LSP diagnostics for path. It is a thin wrapper
// over SetDiagnosticsSource so LSP and plugin diagnostics merge on the same tab.
func (g *EditorGroupWidget) SetDiagnostics(path string, diags []Diagnostic) {
	g.SetDiagnosticsSource("lsp", path, diags)
}

// SetDiagnosticsSource stores the diagnostics for a given source/path pair and
// recomputes the merged diagnostics shown for that path.
func (g *EditorGroupWidget) SetDiagnosticsSource(source, path string, diags []Diagnostic) {
	if g.diagSources == nil {
		g.diagSources = make(map[string]map[string][]Diagnostic)
	}
	byPath := g.diagSources[source]
	if byPath == nil {
		byPath = make(map[string][]Diagnostic)
		g.diagSources[source] = byPath
	}
	if len(diags) == 0 {
		delete(byPath, path)
	} else {
		byPath[path] = diags
	}
	g.recomputeDiagnostics(path)
	if g.OnDiagnosticsChanged != nil {
		g.OnDiagnosticsChanged()
	}
}

// DiagnosticsByPath returns every diagnostic across all sources, merged by
// file path — used to populate the Diagnostics panel.
func (g *EditorGroupWidget) DiagnosticsByPath() map[string][]Diagnostic {
	merged := make(map[string][]Diagnostic)
	for _, byPath := range g.diagSources {
		for path, diags := range byPath {
			merged[path] = append(merged[path], diags...)
		}
	}
	return merged
}

func (g *EditorGroupWidget) DiagnosticsForSource(source, path string) []Diagnostic {
	return append([]Diagnostic(nil), g.diagSources[source][path]...)
}

// ClearDiagnosticsSource removes all diagnostics published by source and
// recomputes every path it touched.
func (g *EditorGroupWidget) ClearDiagnosticsSource(source string) {
	byPath, ok := g.diagSources[source]
	if !ok {
		return
	}
	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	delete(g.diagSources, source)
	for _, path := range paths {
		g.recomputeDiagnostics(path)
	}
	if g.OnDiagnosticsChanged != nil {
		g.OnDiagnosticsChanged()
	}
}

// recomputeDiagnostics merges every source's diagnostics for path and applies
// the result to the matching tab (and the active editor if it is that tab).
func (g *EditorGroupWidget) recomputeDiagnostics(path string) {
	var merged []Diagnostic
	for _, byPath := range g.diagSources {
		if diags := byPath[path]; len(diags) > 0 {
			merged = append(merged, diags...)
		}
	}
	for i := range g.tabs {
		if g.tabs[i].FilePath == path {
			g.tabs[i].Diagnostics = merged
			if i == g.active {
				g.Editor.Diagnostics = merged
				g.Editor.buildDiagIndex()
			}
			return
		}
	}
}
