package plugin

import (
	"github.com/eugenioenko/ttt/internal/widgets"
)

type WidgetKind int

const (
	WidgetLabel WidgetKind = iota
	WidgetTree
	WidgetList
	WidgetButton
	WidgetInput
	WidgetVStack
	WidgetBox
	WidgetDropdown
	WidgetTitle
	WidgetKeyValue
	WidgetScrollView
	WidgetHStack
	WidgetDivider
	WidgetProgress
	WidgetTable
	WidgetMarkdown
	WidgetCheckbox
)

func (k WidgetKind) String() string {
	switch k {
	case WidgetLabel:
		return "label"
	case WidgetTree:
		return "tree"
	case WidgetList:
		return "list"
	case WidgetButton:
		return "button"
	case WidgetInput:
		return "input"
	case WidgetVStack:
		return "vstack"
	case WidgetBox:
		return "box"
	case WidgetDropdown:
		return "dropdown"
	case WidgetTitle:
		return "title"
	case WidgetKeyValue:
		return "keyvalue"
	case WidgetScrollView:
		return "scrollview"
	case WidgetHStack:
		return "hstack"
	case WidgetDivider:
		return "divider"
	case WidgetProgress:
		return "progress"
	case WidgetTable:
		return "table"
	case WidgetMarkdown:
		return "markdown"
	case WidgetCheckbox:
		return "checkbox"
	}
	return "unknown"
}

type WidgetDesc struct {
	Kind WidgetKind
	Key  string

	Text      string
	TextStyle string
	Badge     string
	Icon      string
	Padded    bool

	MarginTop     int
	MarginBottom  int
	MarginLeft    int
	MarginRight   int
	PaddingTop    int
	PaddingBottom int
	PaddingLeft   int
	PaddingRight  int

	Items         []*widgets.TreeNode
	Indent        int
	SelectOnClick bool
	TruncateLeft  bool
	OnSelect      func(node *widgets.TreeNode)
	OnExpand      func(node *widgets.TreeNode)
	OnCommand     func(command string, node *widgets.TreeNode)
	NodeMenu      []widgets.MenuEntry
	KeyCommands   map[rune]string

	Label        string
	OnClick      func()
	Checked      bool
	OnChangeBool func(bool)

	Placeholder   string
	Prefix        string
	OnChange      func(text string)
	OnSubmit      func(text string)
	ClearOnSubmit bool

	Children     []WidgetDesc
	Border       bool
	BorderTop    bool
	BorderBottom bool
	BorderLeft   bool
	BorderRight  bool
	FixedHeight  int
	FixedWidth   int
	Gap          int

	Entries         []widgets.MenuEntry
	OnMenu          func(command string)
	KeyValueEntries []widgets.KeyValueEntry

	Value     float64
	Char      rune
	StyleName string

	Columns       []widgets.TableColumn
	Rows          [][]string
	OnSelectIndex func(int)
	OnCommandStr  func(command string, rowIndex int)

	MarkdownContent string
}
