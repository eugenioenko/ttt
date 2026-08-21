package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestBuildStyleMapWiresCollapsedDiffStyle(t *testing.T) {
	theme := config.DefaultTheme()
	theme.Diff.Collapsed = config.StyleDef{
		Fg:   "#123456",
		Bg:   "#654321",
		Bold: true,
	}

	styles := BuildStyleMap(theme)
	collapsed := styles[term.StyleDiffCollapsed]
	if collapsed == styles[term.StyleDefault] {
		t.Fatal("collapsed diff style resolved to StyleDefault")
	}
	if got := collapsed.GetForeground(); got != tcell.GetColor("#123456") {
		t.Fatalf("collapsed foreground = %v, want #123456", got)
	}
	if got := collapsed.GetBackground(); got != tcell.GetColor("#654321") {
		t.Fatalf("collapsed background = %v, want #654321", got)
	}
	if collapsed.GetAttributes()&tcell.AttrBold == 0 {
		t.Fatal("collapsed diff style should be bold")
	}
}

func TestBuildStyleMapWiresCommitMessageStyle(t *testing.T) {
	theme := config.DefaultTheme()
	theme.CommitMessage = config.StyleDef{Fg: "#abcdef", Bg: "#123456", Bold: true}

	styles := BuildStyleMap(theme)
	message := styles[term.StyleCommitMessage]
	if got := message.GetForeground(); got != tcell.GetColor("#abcdef") {
		t.Fatalf("commit message foreground = %v, want #abcdef", got)
	}
	if got := message.GetBackground(); got != tcell.GetColor("#123456") {
		t.Fatalf("commit message background = %v, want #123456", got)
	}
	if message.GetAttributes()&tcell.AttrBold == 0 {
		t.Fatal("commit message style should preserve configured bold")
	}
}
