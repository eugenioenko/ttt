package config

type StyleDef struct {
	Fg   string `json:"fg,omitempty"`
	Bg   string `json:"bg,omitempty"`
	Bold bool   `json:"bold,omitempty"`
}

type BorderChars struct {
	Horizontal  string `json:"horizontal"`
	Vertical    string `json:"vertical"`
	TopLeft     string `json:"topLeft"`
	TopRight    string `json:"topRight"`
	BottomLeft  string `json:"bottomLeft"`
	BottomRight string `json:"bottomRight"`
	TopTee      string `json:"topTee"`
	BottomTee   string `json:"bottomTee"`
	LeftTee     string `json:"leftTee"`
	RightTee    string `json:"rightTee"`
}

type TabStyles struct {
	Active   StyleDef `json:"active"`
	Inactive StyleDef `json:"inactive"`
}

type SidebarStyles struct {
	Header   StyleDef `json:"header"`
	Item     StyleDef `json:"item"`
	Selected StyleDef `json:"selected"`
}

type DialogStyles struct {
	Input    StyleDef `json:"input"`
	Item     StyleDef `json:"item"`
	Selected StyleDef `json:"selected"`
	Muted    StyleDef `json:"muted"`
}

type InputStyles struct {
	Item        StyleDef `json:"item"`
	Placeholder StyleDef `json:"placeholder"`
	Action      StyleDef `json:"action"`
}

type MenuStyles struct {
	Item   StyleDef `json:"item"`
	Active StyleDef `json:"active"`
}

type DiagnosticStyles struct {
	Error   StyleDef `json:"error"`
	Warning StyleDef `json:"warning"`
	Info    StyleDef `json:"info"`
	Hint    StyleDef `json:"hint"`
}

type EditorStyles struct {
	LineNumber   StyleDef         `json:"lineNumber"`
	ActiveLine   StyleDef         `json:"activeLine"`
	Selection    StyleDef         `json:"selection"`
	SearchMatch  StyleDef         `json:"searchMatch"`
	SearchActive StyleDef         `json:"searchActive"`
	BracketMatch StyleDef         `json:"bracketMatch"`
	Bookmark     StyleDef         `json:"bookmark"`
	Diagnostics  DiagnosticStyles `json:"diagnostics"`
}

type DiffStyles struct {
	Added    StyleDef `json:"added"`
	Deleted  StyleDef `json:"deleted"`
	Modified StyleDef `json:"modified"`
}

type SyntaxStyles struct {
	Comment     StyleDef `json:"comment"`
	String      StyleDef `json:"string"`
	Keyword     StyleDef `json:"keyword"`
	Number      StyleDef `json:"number"`
	Operator    StyleDef `json:"operator"`
	Function    StyleDef `json:"function"`
	Type        StyleDef `json:"type"`
	Builtin     StyleDef `json:"builtin"`
	Variable    StyleDef `json:"variable"`
	Punctuation StyleDef `json:"punctuation"`
	Tag         StyleDef `json:"tag"`
	Attribute   StyleDef `json:"attribute"`
}

type TerminalColors struct {
	Foreground    string `json:"foreground,omitempty"`
	Background    string `json:"background,omitempty"`
	Black         string `json:"black,omitempty"`
	Red           string `json:"red,omitempty"`
	Green         string `json:"green,omitempty"`
	Yellow        string `json:"yellow,omitempty"`
	Blue          string `json:"blue,omitempty"`
	Magenta       string `json:"magenta,omitempty"`
	Cyan          string `json:"cyan,omitempty"`
	White         string `json:"white,omitempty"`
	BrightBlack   string `json:"brightBlack,omitempty"`
	BrightRed     string `json:"brightRed,omitempty"`
	BrightGreen   string `json:"brightGreen,omitempty"`
	BrightYellow  string `json:"brightYellow,omitempty"`
	BrightBlue    string `json:"brightBlue,omitempty"`
	BrightMagenta string `json:"brightMagenta,omitempty"`
	BrightCyan    string `json:"brightCyan,omitempty"`
	BrightWhite   string `json:"brightWhite,omitempty"`
}

func DefaultTerminalColors() TerminalColors {
	return TerminalColors{
		Black:         "#1e1e1e",
		Red:           "#f44747",
		Green:         "#6a9955",
		Yellow:        "#d7ba7d",
		Blue:          "#569cd6",
		Magenta:       "#c586c0",
		Cyan:          "#4ec9b0",
		White:         "#d4d4d4",
		BrightBlack:   "#808080",
		BrightRed:     "#f14c4c",
		BrightGreen:   "#73c991",
		BrightYellow:  "#e2c08d",
		BrightBlue:    "#6cb6ff",
		BrightMagenta: "#d670d6",
		BrightCyan:    "#58d1c9",
		BrightWhite:   "#e5e5e5",
	}
}

// ANSIPalette returns the 16 ANSI colors as an ordered array [0..15].
func (tc TerminalColors) ANSIPalette() [16]string {
	return [16]string{
		tc.Black, tc.Red, tc.Green, tc.Yellow,
		tc.Blue, tc.Magenta, tc.Cyan, tc.White,
		tc.BrightBlack, tc.BrightRed, tc.BrightGreen, tc.BrightYellow,
		tc.BrightBlue, tc.BrightMagenta, tc.BrightCyan, tc.BrightWhite,
	}
}

type HoverStyles struct {
	Bold StyleDef `json:"bold"`
	Code StyleDef `json:"code"`
}

type ThemeConfig struct {
	Default   StyleDef       `json:"default"`
	Muted     StyleDef       `json:"muted"`
	Success   StyleDef       `json:"success"`
	Danger    StyleDef       `json:"danger"`
	Warning   StyleDef       `json:"warning"`
	StatusBar StyleDef       `json:"statusBar"`
	Tabs      TabStyles      `json:"tabs"`
	Sidebar   SidebarStyles  `json:"sidebar"`
	Dialog    DialogStyles   `json:"dialog"`
	Editor    EditorStyles   `json:"editor"`
	Menu      MenuStyles     `json:"menu"`
	Input     InputStyles    `json:"input"`
	Hover     HoverStyles    `json:"hover"`
	Border    StyleDef       `json:"border"`
	Diff      DiffStyles     `json:"diff"`
	Scrollbar StyleDef       `json:"scrollbar"`
	Syntax    SyntaxStyles   `json:"syntax"`
	Borders   BorderChars    `json:"borders"`
	Terminal  TerminalColors `json:"terminal,omitempty"`
}

func DefaultTheme() ThemeConfig {
	t := ThemeConfig{
		Terminal: DefaultTerminalColors(),
		Default:  StyleDef{Fg: "#fafafa", Bg: "#1f1f1f"},
		Muted:    StyleDef{Fg: "#888888"},

		Menu: MenuStyles{
			Active: StyleDef{Fg: "#ffffff", Bg: "#505050", Bold: true},
		},
		StatusBar: StyleDef{},

		Tabs: TabStyles{
			Active:   StyleDef{Fg: "#ffffff", Bold: true},
			Inactive: StyleDef{Fg: "#999999"},
		},

		Sidebar: SidebarStyles{
			Header:   StyleDef{Fg: "#ffffff", Bold: true},
			Selected: StyleDef{Fg: "#ffffff", Bg: "#37373d"},
		},

		Dialog: DialogStyles{
			Selected: StyleDef{Fg: "#ffffff", Bg: "#37373d"},
		},

		Border: StyleDef{Fg: "#555555"},

		Editor: EditorStyles{
			ActiveLine:   StyleDef{Bg: "#282828"},
			Selection:    StyleDef{Bg: "#282828"},
			LineNumber:   StyleDef{Fg: "#999999"},
			SearchMatch:  StyleDef{Bg: "#623800"},
			SearchActive: StyleDef{Bg: "#9e6a03"},
			BracketMatch: StyleDef{Bg: "#3a3a3a"},
		},
		Scrollbar: StyleDef{Fg: "#999999", Bg: "#555555"},

		Syntax: SyntaxStyles{
			Comment:     StyleDef{Fg: "#6a9955"},
			String:      StyleDef{Fg: "#ce9178"},
			Keyword:     StyleDef{Fg: "#569cd6"},
			Number:      StyleDef{Fg: "#b5cea8"},
			Operator:    StyleDef{Fg: "#d4d4d4"},
			Function:    StyleDef{Fg: "#dcdcaa"},
			Type:        StyleDef{Fg: "#4ec9b0"},
			Builtin:     StyleDef{Fg: "#4ec9b0"},
			Variable:    StyleDef{Fg: "#9cdcfe"},
			Punctuation: StyleDef{Fg: "#d4d4d4"},
			Tag:         StyleDef{Fg: "#569cd6"},
			Attribute:   StyleDef{Fg: "#9cdcfe"},
		},

		Borders: BorderChars{
			Horizontal:  "─",
			Vertical:    "│",
			TopLeft:     "┌",
			TopRight:    "┐",
			BottomLeft:  "└",
			BottomRight: "┘",
			TopTee:      "┬",
			BottomTee:   "┴",
			LeftTee:     "├",
			RightTee:    "┤",
		},
	}
	return t
}

func (t *ThemeConfig) ResolveColors() {
	fillFg(&t.Dialog.Muted, t.Muted.Fg)
	fillBg(&t.Input.Item, t.Default.Bg)
	fillFg(&t.Input.Item, t.Default.Fg)
	fillFg(&t.Input.Placeholder, t.Muted.Fg)
	fillFg(&t.Input.Action, t.Muted.Fg)
	fillBg(&t.Diff.Added, "#1e2e1e")
	fillBg(&t.Diff.Deleted, "#2e1e1e")
	fillBg(&t.Diff.Modified, "#2e2e1e")
	fillFg(&t.Success, "#73c991")
	fillFg(&t.Danger, "#f14c4c")
	fillFg(&t.Warning, "#e2c08d")
	fillFg(&t.Editor.Diagnostics.Error, t.Danger.Fg)
	fillFg(&t.Editor.Diagnostics.Warning, t.Warning.Fg)
	fillFg(&t.Editor.Diagnostics.Info, t.Default.Fg)
	fillFg(&t.Editor.Diagnostics.Hint, t.Default.Fg)
	if !t.Hover.Bold.Bold {
		t.Hover.Bold.Bold = true
	}
	fillFg(&t.Hover.Bold, t.Default.Fg)
	fillFg(&t.Hover.Code, t.Syntax.String.Fg)
	fillFg(&t.Editor.Bookmark, "#d7ba7d")
}

func fillFg(s *StyleDef, color string) {
	if s.Fg == "" {
		s.Fg = color
	}
}

func fillBg(s *StyleDef, color string) {
	if s.Bg == "" {
		s.Bg = color
	}
}
