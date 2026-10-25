package plugin

import (
	"github.com/eugenioenko/ttt/internal/term"
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
	WidgetImage
	WidgetTabs
	WidgetPager
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
	case WidgetImage:
		return "image"
	case WidgetTabs:
		return "tabs"
	case WidgetPager:
		return "pager"
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
	// PaddingXSet records an explicit padding_left/padding_right, so widgets with
	// default horizontal padding keep it when the plugin sets neither.
	PaddingXSet bool

	Items         []*widgets.TreeNode
	Indent        int
	SelectOnClick bool
	TruncateLeft  bool
	OnSelect      func(node *widgets.TreeNode)
	OnExpand      func(node *widgets.TreeNode)
	OnCommand     func(command string, node *widgets.TreeNode)
	NodeMenu      []widgets.MenuEntry
	KeyCommands   map[rune]string

	BadgeRight bool

	Label        string
	OnClick      func()
	Checked      bool
	OnChangeBool func(bool)

	InputValue    *string
	Placeholder   string
	Prefix        string
	OnChange      func(text string)
	OnSubmit      func(text string)
	ClearOnSubmit bool
	Wrap          bool
	MaxRows       int
	MinRows       int
	Multiline     bool
	Autofocus     bool
	Password      bool

	ImageData   string
	ImageFit    string
	ImageAlt    string
	ImageHeight int

	Children     []WidgetDesc
	Border       bool
	BorderTop    bool
	BorderBottom bool
	BorderLeft   bool
	BorderRight  bool
	FixedHeight  int
	FixedWidth   int
	Gap          int
	BoxTitle     string

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

	TabItems    []widgets.TabItem
	ActiveTab   string
	OnTabChange func(id string)

	PagerOffset int
	PagerLimit  int
	PagerTotal  int
	OnPage      func(offset int)

	OnActivateIndex func(int)
	RowStyles       []term.Style
	Selected        int
	Loading         bool
	EmptyText       string
	Inset           int

	FullWidthDividers bool
	MarkdownContent   string
	Follow            bool
	WrapWidth         int
}
