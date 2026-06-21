package term

import (
	"github.com/gdamore/tcell/v2"
)

const StyleCount = 57

type StyleMap [StyleCount]tcell.Style

func DefaultStyleMap() StyleMap {
	var m StyleMap
	for i := range m {
		m[i] = tcell.StyleDefault
	}
	m[StyleSelection] = tcell.StyleDefault.Reverse(true)
	return m
}

// TcellScreen implements the Screen interface using tcell.
type TcellScreen struct {
	scr      tcell.Screen
	styleMap StyleMap
}

func NewTcellScreen() (*TcellScreen, error) {
	s, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}
	if err := s.Init(); err != nil {
		return nil, err
	}
	s.EnableMouse()
	return &TcellScreen{scr: s, styleMap: DefaultStyleMap()}, nil
}

func NewTcellScreenFrom(s tcell.Screen) *TcellScreen {
	return &TcellScreen{scr: s, styleMap: DefaultStyleMap()}
}

func (t *TcellScreen) SetStyleMap(m StyleMap) {
	t.styleMap = m
}

func (t *TcellScreen) GetStyleMap() StyleMap {
	return t.styleMap
}

func (t *TcellScreen) Size() (w, h int) {
	return t.scr.Size()
}

func (t *TcellScreen) SetCell(x, y int, c Cell) {
	if c.Direct {
		s := tcell.StyleDefault
		if c.Fg.Set {
			s = s.Foreground(tcell.NewRGBColor(int32(c.Fg.R), int32(c.Fg.G), int32(c.Fg.B)))
		}
		if c.Bg.Set {
			s = s.Background(tcell.NewRGBColor(int32(c.Bg.R), int32(c.Bg.G), int32(c.Bg.B)))
		}
		if c.Attrs&CellAttrBold != 0 {
			s = s.Bold(true)
		}
		if c.Attrs&CellAttrUnderline != 0 {
			s = s.Underline(true)
		}
		if c.Attrs&CellAttrItalic != 0 {
			s = s.Italic(true)
		}
		if c.Attrs&CellAttrReverse != 0 {
			s = s.Reverse(true)
		}
		if c.Attrs&CellAttrBlink != 0 {
			s = s.Blink(true)
		}
		t.scr.SetContent(x, y, c.Ch, nil, s)
		return
	}
	s := t.styleMap[c.Style]
	if c.BgStyle != 0 {
		_, bg, _ := t.styleMap[c.BgStyle].Decompose()
		fg, _, attrs := s.Decompose()
		s = tcell.StyleDefault.Foreground(fg).Background(bg).Attributes(attrs)
	}
	if c.UlStyle != 0 {
		us := t.styleMap[c.UlStyle]
		s = s.Underline(us.GetUnderlineStyle(), us.GetUnderlineColor())
	}
	if c.Underline {
		s = s.Underline(true)
	}
	t.scr.SetContent(x, y, c.Ch, nil, s)
}

func (t *TcellScreen) Show() {
	t.scr.Show()
}

func (t *TcellScreen) Clear() {
	t.scr.Clear()
}

func (t *TcellScreen) PollEvent() tcell.Event {
	return t.scr.PollEvent()
}

func (t *TcellScreen) Fini() {
	t.scr.Fini()
}

func (t *TcellScreen) ShowCursor(x, y int) {
	t.scr.ShowCursor(x, y)
}

func (t *TcellScreen) HideCursor() {
	t.scr.HideCursor()
}

var cursorStyleMap = map[CursorStyle]tcell.CursorStyle{
	CursorStyleBlinkingBar:       tcell.CursorStyleBlinkingBar,
	CursorStyleSteadyBar:         tcell.CursorStyleSteadyBar,
	CursorStyleBlinkingBlock:     tcell.CursorStyleBlinkingBlock,
	CursorStyleSteadyBlock:       tcell.CursorStyleSteadyBlock,
	CursorStyleBlinkingUnderline: tcell.CursorStyleBlinkingUnderline,
	CursorStyleSteadyUnderline:   tcell.CursorStyleSteadyUnderline,
}

func (t *TcellScreen) SetCursorStyle(style CursorStyle) {
	if cs, ok := cursorStyleMap[style]; ok {
		t.scr.SetCursorStyle(cs)
	}
}

func (t *TcellScreen) PostEvent(ev tcell.Event) error {
	return t.scr.PostEvent(ev)
}

func (t *TcellScreen) Tty() (tcell.Tty, bool) {
	return t.scr.Tty()
}
