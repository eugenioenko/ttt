package app

import (
	"github.com/eugenioenko/ttt/internal/fileicons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func setFileIcon(node *widgets.TreeNode) {
	icon := fileicons.ForFile(node.Label)
	node.Icon = icon.Glyph
	node.IconStyle = mutedIconStyle(node, fileIconStyle(icon.Color))
}

// setLabelIcon leaves Icon free for rows that already use it, such as the Git
// status letter.
func setLabelIcon(node *widgets.TreeNode, name string) {
	icon := fileicons.ForFile(name)
	node.LabelIcon = icon.Glyph
	node.LabelIconStyle = mutedIconStyle(node, fileIconStyle(icon.Color))
}

func mutedIconStyle(node *widgets.TreeNode, style term.Style) term.Style {
	if node.Muted {
		return term.StyleMuted
	}
	return style
}

func fileIconStyle(color fileicons.Color) term.Style {
	switch color {
	case fileicons.ColorRed:
		return term.StyleFileIconRed
	case fileicons.ColorYellow:
		return term.StyleFileIconYellow
	case fileicons.ColorGreen:
		return term.StyleFileIconGreen
	case fileicons.ColorCyan:
		return term.StyleFileIconCyan
	case fileicons.ColorBlue:
		return term.StyleFileIconBlue
	case fileicons.ColorMagenta:
		return term.StyleFileIconMagenta
	default:
		return term.StyleDefault
	}
}
