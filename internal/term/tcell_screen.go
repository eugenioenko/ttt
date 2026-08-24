package term

import (
	"github.com/gdamore/tcell/v3"
)

const StyleCount = styleCount

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
	s.EnablePaste()
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
		bg := t.styleMap[c.BgStyle].GetBackground()
		s = tcell.StyleDefault. //nolint:staticcheck // Attributes is deprecated but individual calls don't replicate the exact reset-and-copy semantics
			Foreground(s.GetForeground()).
			Background(bg).
			Attributes(s.GetAttributes())
	}
	if c.UlStyle != 0 {
		us := t.styleMap[c.UlStyle]
		ulStyle := us.GetUnderlineStyle()
		ulColor := us.GetUnderlineColor()
		if ulStyle == tcell.UnderlineStyleNone {
			// The style carries no underline of its own (e.g. a plain colour
			// style a plugin passed for a diagnostic). Still draw a squiggle:
			// force curly, coloured by the style's foreground.
			ulStyle = tcell.UnderlineStyleCurly
			if fg := us.GetForeground(); fg != tcell.ColorDefault {
				ulColor = fg
			}
		}
		s = s.Underline(ulStyle, ulColor)
	}
	if c.Underline {
		s = s.Underline(true)
	}
	if c.Bold {
		s = s.Bold(true)
	}
	t.scr.SetContent(x, y, c.Ch, nil, s)
}

func (t *TcellScreen) Show() {
	t.scr.Show()
}

func (t *TcellScreen) Clear() {
	t.scr.Clear()
}

// PollEvent blocks until the next event is available. tcell v3 replaced
// PollEvent with a plain event channel; receiving from it after Fini
// (closed channel) yields nil, matching v2 PollEvent semantics.
func (t *TcellScreen) PollEvent() tcell.Event {
	return <-t.scr.EventQ()
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

// PostEvent injects an event into the screen's event queue (tcell v3 has no
// PostEvent; the queue channel is written directly). The send is non-blocking:
// PostEvent is occasionally called from the event-loop goroutine itself (e.g.
// plugin redraw requests), which also drains this queue, so a blocking send
// on a full queue would deadlock. When the queue is full the event is
// delivered from a goroutine instead of being dropped — async wakeups must
// never be lost.
//
// Fini closes the queue, and a send on a closed channel panics even inside a
// select with a default case. Async posters (PTY output, LSP, file watcher)
// can race shutdown, so both send paths recover and drop the event instead.
func (t *TcellScreen) PostEvent(ev tcell.Event) error {
	defer func() { _ = recover() }()
	q := t.scr.EventQ()
	select {
	case q <- ev:
	default:
		go func() {
			defer func() { _ = recover() }()
			q <- ev
		}()
	}
	return nil
}

// GetContent returns the cell contents at (x, y). tcell v3 replaced
// GetContent (rune-based) with Get (string-based).
func (t *TcellScreen) GetContent(x, y int) (string, tcell.Style, int) {
	return t.scr.Get(x, y)
}

func (t *TcellScreen) Tty() (tcell.Tty, bool) {
	return t.scr.Tty()
}
