package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestBuildStyleMapIncludesCollapsedDiffStyle(t *testing.T) {
	theme := config.DefaultTheme()
	theme.Diff.Collapsed = config.StyleDef{Fg: "#123456", Bg: "#654321", Bold: true}
	styles := BuildStyleMap(theme)
	style := styles[term.StyleDiffCollapsed]
	fg, bg, attrs := style.GetForeground(), style.GetBackground(), style.GetAttributes()
	if fg != tcell.GetColor("#123456") || bg != tcell.GetColor("#654321") || attrs&tcell.AttrBold == 0 {
		t.Fatalf("collapsed diff style = fg %v bg %v attrs %v", fg, bg, attrs)
	}
}

func TestBuildStyleMapUsesResolvedSemanticDiffPairing(t *testing.T) {
	theme := config.DefaultTheme()
	theme.Diff.Added.Bg = "#e8f5e8"
	theme.Diff.Deleted.Bg = "#f5e8e8"
	theme.Diff.GutterAdded.Fg = "#73c991"
	theme.Diff.GutterDeleted.Fg = "#f14c4c"
	theme.ResolveColors()
	styles := BuildStyleMap(theme)

	if got, want := styles[term.StyleGutterAdded].GetForeground(), tcell.GetColor(theme.Diff.GutterAdded.Fg); got != want {
		t.Fatalf("rendered added foreground = %v, want resolved %v", got, want)
	}
	if got, want := styles[term.StyleDiffAdded].GetBackground(), tcell.GetColor(theme.Diff.Added.Bg); got != want {
		t.Fatalf("rendered added background = %v, want resolved %v", got, want)
	}
	if got, want := styles[term.StyleGutterDeleted].GetForeground(), tcell.GetColor(theme.Diff.GutterDeleted.Fg); got != want {
		t.Fatalf("rendered deleted foreground = %v, want resolved %v", got, want)
	}
	if got, want := styles[term.StyleDiffDeleted].GetBackground(), tcell.GetColor(theme.Diff.Deleted.Bg); got != want {
		t.Fatalf("rendered deleted background = %v, want resolved %v", got, want)
	}
}
