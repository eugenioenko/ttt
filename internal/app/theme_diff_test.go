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
