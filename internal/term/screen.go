package term

type Style int

const (
	StyleDefault Style = iota
	StyleStatusBar
	StyleActiveTab
	StyleInactiveTab
	StyleSidebarSelected
	StylePaletteItem
	StylePaletteSelected
	StyleLineNumber
	StyleMenuBar
	StyleMenuBarActive
	StyleBorder
	StyleSelection
	StyleSearchMatch
	StyleSearchActive
	StyleDiffAdded
	StyleDiffDeleted
	StyleDiffModified
	StyleScrollbar
	StyleScrollbarThumb
	StyleActiveLine
	StyleSyntaxComment
	StyleSyntaxString
	StyleSyntaxKeyword
	StyleSyntaxNumber
	StyleSyntaxOperator
	StyleSyntaxFunction
	StyleSyntaxType
	StyleSyntaxBuiltin
	StyleSyntaxVariable
	StyleSyntaxPunctuation
	StyleSyntaxTag
	StyleSyntaxAttribute
	StyleMuted
	StyleBracketMatch
	StyleSuccess
	StyleDanger
	StyleWarning
	StyleDiagError
	StyleDiagWarning
	StyleDiagInfo
	StyleDiagHint
	StyleInput
	StyleInputPlaceholder
	StyleInputAction
	StyleHoverBold
	StyleHoverCode
	StyleBracketColor1
	StyleBracketColor2
	StyleBracketColor3
	StyleBracketColor4
	StyleBracketColor5
	StyleBracketColor6
	StyleGutterAdded
	StyleGutterModified
	StyleGutterDeleted
	StyleCommentBg
	StyleCommentAuthor
)

// DirectColor holds an RGBA color for terminal emulator output.
// Zero value means "use default".
type DirectColor struct {
	R, G, B byte
	Set     bool
}

// CellAttr holds text attribute flags for direct-style cells.
type CellAttr byte

const (
	CellAttrBold CellAttr = 1 << iota
	CellAttrUnderline
	CellAttrItalic
	CellAttrReverse
	CellAttrBlink
)

// Cell represents a single character cell on the screen.
type Cell struct {
	Ch        rune
	Style     Style
	BgStyle   Style // when non-zero, background comes from this style instead of Style
	UlStyle   Style // when non-zero, underline style+color comes from this style
	Underline bool  // when true, applies underline to styled (non-Direct) cells

	// Direct-style fields for terminal emulator cells.
	// When Direct is true, Fg/Bg/Attrs are used instead of Style.
	Direct bool
	Fg     DirectColor
	Bg     DirectColor
	Attrs  CellAttr
}

// CursorStyle represents the shape of the text cursor.
type CursorStyle int

const (
	CursorStyleBlinkingBar CursorStyle = iota // default
	CursorStyleSteadyBar
	CursorStyleBlinkingBlock
	CursorStyleSteadyBlock
	CursorStyleBlinkingUnderline
	CursorStyleSteadyUnderline
)

func ParseCursorStyle(s string) CursorStyle {
	switch s {
	case "bar", "blinkingBar", "":
		return CursorStyleBlinkingBar
	case "steadyBar":
		return CursorStyleSteadyBar
	case "block", "blinkingBlock":
		return CursorStyleBlinkingBlock
	case "steadyBlock":
		return CursorStyleSteadyBlock
	case "underline", "blinkingUnderline":
		return CursorStyleBlinkingUnderline
	case "steadyUnderline":
		return CursorStyleSteadyUnderline
	default:
		return CursorStyleBlinkingBar
	}
}

// Screen abstracts the terminal screen.
type Screen interface {
	Size() (w, h int)
	SetCell(x, y int, c Cell)
	Show()
	Clear()
	ShowCursor(x, y int)
	HideCursor()
	SetCursorStyle(style CursorStyle)
}

// MockScreen is a test/mock implementation of Screen.
type MockScreen struct {
	Width, Height int
	Cells         map[[2]int]Cell
}

func NewMockScreen(w, h int) *MockScreen {
	return &MockScreen{
		Width:  w,
		Height: h,
		Cells:  make(map[[2]int]Cell),
	}
}

func (m *MockScreen) Size() (int, int) {
	return m.Width, m.Height
}

func (m *MockScreen) SetCell(x, y int, c Cell) {
	m.Cells[[2]int{x, y}] = c
}

func (m *MockScreen) Show()                      {}
func (m *MockScreen) Clear()                     { m.Cells = make(map[[2]int]Cell) }
func (m *MockScreen) ShowCursor(x, y int)        {}
func (m *MockScreen) HideCursor()                {}
func (m *MockScreen) SetCursorStyle(CursorStyle) {}
