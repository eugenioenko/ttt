package app

import (
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v2"
)

func BuildStyleMap(theme config.ThemeConfig) term.StyleMap {
	m := term.DefaultStyleMap()

	base := tcell.StyleDefault
	if theme.Default.Fg != "" {
		base = base.Foreground(tcell.GetColor(theme.Default.Fg))
	}
	if theme.Default.Bg != "" {
		base = base.Background(tcell.GetColor(theme.Default.Bg))
	}
	for i := range m {
		m[i] = base
	}
	m[term.StyleSelection] = base.Reverse(true)

	applyStyleDef(&m, term.StyleStatusBar, theme.StatusBar)
	applyStyleDef(&m, term.StyleActiveTab, theme.Tabs.Active)
	applyStyleDef(&m, term.StyleInactiveTab, theme.Tabs.Inactive)
	applyStyleDef(&m, term.StyleSidebarSelected, theme.Sidebar.Selected)
	applyStyleDef(&m, term.StylePaletteItem, theme.Dialog.Item)
	applyStyleDef(&m, term.StylePaletteSelected, theme.Dialog.Selected)
	applyStyleDef(&m, term.StyleLineNumber, theme.Editor.LineNumber)
	applyStyleDef(&m, term.StyleMenuBar, theme.Menu.Item)
	applyStyleDef(&m, term.StyleMenuBarActive, theme.Menu.Active)
	applyStyleDef(&m, term.StyleBorder, theme.Border)
	applyStyleDef(&m, term.StyleSelection, theme.Editor.Selection)
	applyStyleDef(&m, term.StyleSearchMatch, theme.Editor.SearchMatch)
	applyStyleDef(&m, term.StyleSearchActive, theme.Editor.SearchActive)
	applyStyleDef(&m, term.StyleBracketMatch, theme.Editor.BracketMatch)
	applyStyleDef(&m, term.StyleDiffAdded, theme.Diff.Added)
	applyStyleDef(&m, term.StyleDiffDeleted, theme.Diff.Deleted)
	applyStyleDef(&m, term.StyleDiffModified, theme.Diff.Modified)
	applyStyleDef(&m, term.StyleGutterAdded, theme.Diff.GutterAdded)
	applyStyleDef(&m, term.StyleGutterDeleted, theme.Diff.GutterDeleted)
	applyStyleDef(&m, term.StyleGutterModified, theme.Diff.GutterModified)
	applyStyleDef(&m, term.StyleGutterComment, theme.Diff.GutterModified) // reuse modified color for comment markers
	applyStyleDef(&m, term.StyleActiveLine, theme.Editor.ActiveLine)
	applyStyleDef(&m, term.StyleScrollbar, config.StyleDef{Fg: theme.Scrollbar.Bg})
	applyStyleDef(&m, term.StyleScrollbarThumb, config.StyleDef{Fg: theme.Scrollbar.Fg})
	applyStyleDef(&m, term.StyleSyntaxComment, theme.Syntax.Comment)
	applyStyleDef(&m, term.StyleSyntaxString, theme.Syntax.String)
	applyStyleDef(&m, term.StyleSyntaxKeyword, theme.Syntax.Keyword)
	applyStyleDef(&m, term.StyleSyntaxNumber, theme.Syntax.Number)
	applyStyleDef(&m, term.StyleSyntaxOperator, theme.Syntax.Operator)
	applyStyleDef(&m, term.StyleSyntaxFunction, theme.Syntax.Function)
	applyStyleDef(&m, term.StyleSyntaxType, theme.Syntax.Type)
	applyStyleDef(&m, term.StyleSyntaxBuiltin, theme.Syntax.Builtin)
	applyStyleDef(&m, term.StyleSyntaxVariable, theme.Syntax.Variable)
	applyStyleDef(&m, term.StyleSyntaxPunctuation, theme.Syntax.Punctuation)
	applyStyleDef(&m, term.StyleSyntaxTag, theme.Syntax.Tag)
	applyStyleDef(&m, term.StyleSyntaxAttribute, theme.Syntax.Attribute)
	applyStyleDef(&m, term.StyleInput, theme.Input.Item)
	applyStyleDef(&m, term.StyleInputPlaceholder, theme.Input.Placeholder)
	applyStyleDef(&m, term.StyleInputAction, theme.Input.Action)
	applyStyleDef(&m, term.StyleHoverBold, theme.Hover.Bold)
	applyStyleDef(&m, term.StyleHoverCode, theme.Hover.Code)
	applyStyleDef(&m, term.StyleMuted, theme.Muted)
	applyStyleDef(&m, term.StyleSuccess, theme.Success)
	applyStyleDef(&m, term.StyleDanger, theme.Danger)
	applyStyleDef(&m, term.StyleWarning, theme.Warning)

	applyDiagStyle(&m, term.StyleDiagError, theme.Editor.Diagnostics.Error)
	applyDiagStyle(&m, term.StyleDiagWarning, theme.Editor.Diagnostics.Warning)
	applyDiagStyle(&m, term.StyleDiagInfo, theme.Editor.Diagnostics.Info)
	applyDiagStyle(&m, term.StyleDiagHint, theme.Editor.Diagnostics.Hint)

	applyBracketColors(&m, theme.Editor.BracketColors, theme.Terminal)

	return m
}

var syntaxStyleNames = map[string]term.Style{
	"comment":     term.StyleSyntaxComment,
	"string":      term.StyleSyntaxString,
	"keyword":     term.StyleSyntaxKeyword,
	"number":      term.StyleSyntaxNumber,
	"operator":    term.StyleSyntaxOperator,
	"function":    term.StyleSyntaxFunction,
	"type":        term.StyleSyntaxType,
	"builtin":     term.StyleSyntaxBuiltin,
	"variable":    term.StyleSyntaxVariable,
	"punctuation": term.StyleSyntaxPunctuation,
	"tag":         term.StyleSyntaxTag,
	"attribute":   term.StyleSyntaxAttribute,
}

func applyBracketColors(m *term.StyleMap, colors []string, tc config.TerminalColors) {
	slots := []term.Style{
		term.StyleBracketColor1, term.StyleBracketColor2, term.StyleBracketColor3,
		term.StyleBracketColor4, term.StyleBracketColor5, term.StyleBracketColor6,
	}
	for i, c := range colors {
		if i >= len(slots) {
			break
		}
		if strings.HasPrefix(c, "#") {
			m[slots[i]] = m[slots[i]].Foreground(tcell.GetColor(c))
		} else if ref, ok := syntaxStyleNames[c]; ok {
			m[slots[i]] = m[ref]
		} else if hex := tc.ColorByName(c); hex != "" {
			m[slots[i]] = m[slots[i]].Foreground(tcell.GetColor(hex))
		}
	}
}

func firstRune(s string, fallback rune) rune {
	for _, r := range s {
		return r
	}
	return fallback
}

func BuildBorderSet(bc config.BorderChars) term.BorderSet {
	d := term.SingleBorderSet()
	return term.BorderSet{
		Horizontal:  firstRune(bc.Horizontal, d.Horizontal),
		Vertical:    firstRune(bc.Vertical, d.Vertical),
		TopLeft:     firstRune(bc.TopLeft, d.TopLeft),
		TopRight:    firstRune(bc.TopRight, d.TopRight),
		BottomLeft:  firstRune(bc.BottomLeft, d.BottomLeft),
		BottomRight: firstRune(bc.BottomRight, d.BottomRight),
		TopTee:      firstRune(bc.TopTee, d.TopTee),
		BottomTee:   firstRune(bc.BottomTee, d.BottomTee),
		LeftTee:     firstRune(bc.LeftTee, d.LeftTee),
		RightTee:    firstRune(bc.RightTee, d.RightTee),
	}
}

func applyStyleDef(m *term.StyleMap, idx term.Style, def config.StyleDef) {
	if def.Fg == "" && def.Bg == "" && !def.Bold {
		return
	}
	s := m[idx]
	if def.Fg != "" {
		s = s.Foreground(tcell.GetColor(def.Fg))
	}
	if def.Bg != "" {
		s = s.Background(tcell.GetColor(def.Bg))
	}
	if def.Bold {
		s = s.Bold(true)
	}
	m[idx] = s
}

func applyDiagStyle(m *term.StyleMap, idx term.Style, def config.StyleDef) {
	color := tcell.ColorRed
	if def.Fg != "" {
		color = tcell.GetColor(def.Fg)
	}
	m[idx] = tcell.StyleDefault.Underline(tcell.UnderlineStyleCurly, color)
}

func BuildTerminalPalettePtr(theme config.ThemeConfig) *ui.TerminalColorPalette {
	p := BuildTerminalPalette(theme)
	return &p
}

func BuildTerminalPalette(theme config.ThemeConfig) ui.TerminalColorPalette {
	tc := theme.Terminal
	fg := tc.Foreground
	if fg == "" {
		fg = theme.Default.Fg
	}
	bg := tc.Background
	if bg == "" {
		bg = theme.Default.Bg
	}
	ansi := tc.ANSIPalette()
	p := ui.TerminalColorPalette{
		Fg:       ui.ParseHexColor(fg),
		Bg:       ui.ParseHexColor(bg),
		Color256: ui.Build256Palette(),
	}
	for i, hex := range ansi {
		p.ANSI[i] = ui.ParseHexColor(hex)
		p.Color256[i] = p.ANSI[i]
	}
	return p
}
