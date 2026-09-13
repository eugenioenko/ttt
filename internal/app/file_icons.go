package app

import (
	"github.com/eugenioenko/ttt/internal/fileicons"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func setFolderIcon(node *widgets.TreeNode) {
	node.Icon = fileicons.ForFolder(false).Glyph
	node.ExpandedIcon = fileicons.ForFolder(true).Glyph
	node.IconStyle = mutedIconStyle(node, fileIconStyle(fileicons.ForFolder(false).Color))
}

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

func setFolderIcons(nodes []*widgets.TreeNode) {
	for _, node := range nodes {
		if node.Expandable {
			setFolderIcon(node)
			setFolderIcons(node.Children)
		}
	}
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
