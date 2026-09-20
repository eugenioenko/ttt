package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/term"
)

func TestMarkdownSymbolsNesting(t *testing.T) {
	lines := []string{
		"# Title",
		"",
		"## Section One",
		"### Sub",
		"## Section Two",
		"# Second Top",
	}
	syms := markdownSymbols(lines)
	if len(syms) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(syms))
	}
	if syms[0].Name != "Title" || syms[1].Name != "Second Top" {
		t.Errorf("unexpected roots: %s, %s", syms[0].Name, syms[1].Name)
	}
	if len(syms[0].Children) != 2 {
		t.Fatalf("expected 2 children under Title, got %d", len(syms[0].Children))
	}
	if syms[0].Children[0].Name != "Section One" || syms[0].Children[1].Name != "Section Two" {
		t.Errorf("unexpected children: %s, %s", syms[0].Children[0].Name, syms[0].Children[1].Name)
	}
	if len(syms[0].Children[0].Children) != 1 || syms[0].Children[0].Children[0].Name != "Sub" {
		t.Error("expected Sub nested under Section One")
	}
	if syms[1].SelectionRange.Start.Line != 5 {
		t.Errorf("expected Second Top at line 5, got %d", syms[1].SelectionRange.Start.Line)
	}
}

func TestMarkdownSymbolsSkipsFencesAndNonHeadings(t *testing.T) {
	lines := []string{
		"# Real",
		"```",
		"# comment in fence",
		"```",
		"#nospace",
		"####### seven hashes",
		"#",
		"## Also Real",
	}
	syms := markdownSymbols(lines)
	if len(syms) != 1 {
		t.Fatalf("expected 1 root, got %d", len(syms))
	}
	if len(syms[0].Children) != 1 || syms[0].Children[0].Name != "Also Real" {
		t.Fatalf("expected only 'Also Real' child, got %+v", syms[0].Children)
	}
}

func TestMarkdownSymbolsMixedFenceMarkers(t *testing.T) {
	lines := []string{
		"# Real",
		"```",
		"~~~",
		"# hidden inside backtick fence",
		"```",
		"## After",
	}
	syms := markdownSymbols(lines)
	if len(syms) != 1 {
		t.Fatalf("expected 1 root, got %d", len(syms))
	}
	if len(syms[0].Children) != 1 || syms[0].Children[0].Name != "After" {
		t.Fatalf("~~~ must not close a ``` fence; got %+v", syms[0].Children)
	}
}

func TestMarkdownSymbolsDeepFirstHeading(t *testing.T) {
	syms := markdownSymbols([]string{"### Deep", "# Top"})
	if len(syms) != 2 {
		t.Fatalf("expected 2 roots, got %d", len(syms))
	}
	if syms[0].Name != "Deep" || syms[1].Name != "Top" {
		t.Errorf("unexpected roots: %s, %s", syms[0].Name, syms[1].Name)
	}
}

func TestSymbolNodesIDsAndExpansion(t *testing.T) {
	syms := []lsp.DocumentSymbol{
		{
			Name: "Parent", Kind: lsp.SKStruct,
			SelectionRange: lsp.Range{Start: lsp.Position{Line: 3, Character: 5}},
			Children: []lsp.DocumentSymbol{
				{Name: "child", Kind: lsp.SKField, SelectionRange: lsp.Range{Start: lsp.Position{Line: 4, Character: 1}}},
			},
		},
	}
	nodes := symbolNodes(config.IconsNone, syms, map[string]lsp.SymbolKind{})
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	n := nodes[0]
	if n.ID != "3:5" || !n.Expanded || !n.Expandable {
		t.Errorf("unexpected parent node: %+v", n)
	}
	if len(n.Children) != 1 || n.Children[0].ID != "4:1" {
		t.Errorf("unexpected child node: %+v", n.Children)
	}
	if n.Children[0].Expandable || n.Children[0].Expanded {
		t.Error("leaf node should not be expandable")
	}
}

func TestSelectNearest(t *testing.T) {
	sp := NewSymbolsPanel()
	sp.SetSymbols([]lsp.DocumentSymbol{
		{Name: "a", SelectionRange: lsp.Range{Start: lsp.Position{Line: 0}}},
		{Name: "b", SelectionRange: lsp.Range{Start: lsp.Position{Line: 10}}},
		{Name: "c", SelectionRange: lsp.Range{Start: lsp.Position{Line: 20}}},
	})
	for _, tc := range []struct {
		line int
		want string
	}{{0, "a"}, {9, "a"}, {10, "b"}, {12, "b"}, {100, "c"}} {
		sp.SelectNearest(tc.line)
		sel := sp.Tree.Selected()
		if sel == nil || sel.Label != tc.want {
			t.Errorf("line %d: expected %q selected, got %+v", tc.line, tc.want, sel)
		}
	}
}

func TestSymbolIconsFollowIconMode(t *testing.T) {
	sp := NewSymbolsPanel()
	sp.SetSymbols([]lsp.DocumentSymbol{
		{Name: "Parent", Kind: lsp.SKStruct, SelectionRange: lsp.Range{Start: lsp.Position{Line: 1}},
			Children: []lsp.DocumentSymbol{
				{Name: "run", Kind: lsp.SKMethod, SelectionRange: lsp.Range{Start: lsp.Position{Line: 2}}},
			}},
	})
	parent := sp.Tree.Config.Items[0]
	if parent.Icon != "◆" || parent.Children[0].Icon != "ƒ" {
		t.Fatalf("plain icons = %q, %q", parent.Icon, parent.Children[0].Icon)
	}

	sp.SetIcons(config.IconsNerdFont)
	if parent.Icon != "" || parent.Children[0].Icon != "" {
		t.Errorf("nerd icons = %q, %q", parent.Icon, parent.Children[0].Icon)
	}
	if parent.IconStyle != term.StyleSyntaxType {
		t.Errorf("icon style changed: %v", parent.IconStyle)
	}

	sp.SetIcons(config.IconsNone)
	if parent.Icon != "◆" {
		t.Errorf("icon after switching back = %q", parent.Icon)
	}
}
