package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v3"
)

func TestBuildStyleMapSelectionStyleInvert(t *testing.T) {
	theme := config.DefaultTheme()
	theme.Editor.Selection.Bg = "#123456"

	styles := BuildStyleMap(theme, WithEditorSelectionStyle(config.SelectionStyleInvert))
	style := styles[term.StyleSelection]

	if !style.HasReverse() {
		t.Errorf("StyleSelection.HasReverse() = false, want true when editor.selectionStyle is %q", config.SelectionStyleInvert)
	}
	if style.GetBackground() == tcell.GetColor("#123456") {
		t.Errorf("StyleSelection background = %v, want theme background override to be skipped when editor.selectionStyle is %q", style.GetBackground(), config.SelectionStyleInvert)
	}
}

func TestBuildStyleMapSelectionStyleBackground(t *testing.T) {
	theme := config.DefaultTheme()
	theme.Editor.Selection.Bg = "#123456"

	styles := BuildStyleMap(theme, WithEditorSelectionStyle(config.SelectionStyleBackground))
	style := styles[term.StyleSelection]

	if style.HasReverse() {
		t.Errorf("StyleSelection.HasReverse() = true, want false when editor.selectionStyle is %q", config.SelectionStyleBackground)
	}
	if style.GetBackground() != tcell.GetColor("#123456") {
		t.Errorf("StyleSelection background = %v, want %v", style.GetBackground(), tcell.GetColor("#123456"))
	}
}

func TestBuildStyleMapSelectionStyleDefaultUnset(t *testing.T) {
	theme := config.DefaultTheme()
	theme.Editor.Selection.Bg = "#123456"

	// Unset option defaults to background behavior
	styles := BuildStyleMap(theme)
	style := styles[term.StyleSelection]

	if style.HasReverse() {
		t.Errorf("StyleSelection.HasReverse() = true, want false by default")
	}
	if style.GetBackground() != tcell.GetColor("#123456") {
		t.Errorf("StyleSelection background = %v, want %v", style.GetBackground(), tcell.GetColor("#123456"))
	}
}

func TestBuildTerminalPaletteSelectionStyleInvert(t *testing.T) {
	theme := config.DefaultTheme()
	theme.Terminal.Selection = "#654321"

	p := BuildTerminalPalette(theme, WithTerminalSelectionStyle(config.SelectionStyleInvert))
	if p.SelectionBg.Set {
		t.Errorf("TerminalColorPalette.SelectionBg.Set = true, want unset when terminal.selectionStyle is %q", config.SelectionStyleInvert)
	}
	if p.SelectionStyle != config.SelectionStyleInvert {
		t.Errorf("TerminalColorPalette.SelectionStyle = %q, want %q", p.SelectionStyle, config.SelectionStyleInvert)
	}
}

func TestBuildTerminalPaletteSelectionStyleBackground(t *testing.T) {
	theme := config.DefaultTheme()
	theme.Terminal.Selection = "#654321"

	p := BuildTerminalPalette(theme, WithTerminalSelectionStyle(config.SelectionStyleBackground))
	if !p.SelectionBg.Set {
		t.Errorf("TerminalColorPalette.SelectionBg.Set = false, want set when terminal.selectionStyle is %q", config.SelectionStyleBackground)
	}
	if p.SelectionStyle != config.SelectionStyleBackground {
		t.Errorf("TerminalColorPalette.SelectionStyle = %q, want %q", p.SelectionStyle, config.SelectionStyleBackground)
	}
}

func TestBuildTerminalPaletteSelectionStyleDefaultUnset(t *testing.T) {
	theme := config.DefaultTheme()
	theme.Terminal.Selection = "#654321"

	p := BuildTerminalPalette(theme)
	if !p.SelectionBg.Set {
		t.Errorf("TerminalColorPalette.SelectionBg.Set = false, want set by default")
	}
}
