package term

import (
	"sync"

	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/color"
)

// SimCell is a snapshot of a single simulation screen cell.
type SimCell struct {
	Str   string
	Style tcell.Style
}

// SimScreen is a minimal in-memory implementation of tcell v3's Screen
// interface, replacing the SimulationScreen that tcell v3 removed. It backs
// the --size debug harness and tests: content is stored in a tcell.CellBuffer
// (so grapheme/wide-rune handling matches the real screen) and events are
// exchanged over a buffered channel, mirroring tScreen's EventQ.
type SimScreen struct {
	mu      sync.Mutex
	cells   tcell.CellBuffer
	width   int
	height  int
	style   tcell.Style
	cursorX int
	cursorY int
	events  chan tcell.Event
	tty     tcell.Tty
}

var _ tcell.Screen = (*SimScreen)(nil)

// NewSimScreen returns a SimScreen with the default size of 80x24.
func NewSimScreen() *SimScreen {
	return &SimScreen{
		width:   80,
		height:  24,
		cursorX: -1,
		cursorY: -1,
		// Same capacity as tcell v3's tScreen event queue.
		events: make(chan tcell.Event, 128),
	}
}

func (s *SimScreen) Init() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cells.Resize(s.width, s.height)
	return nil
}

func (s *SimScreen) Fini() {}

func (s *SimScreen) Clear() {
	s.Fill(' ', tcell.StyleDefault)
}

func (s *SimScreen) Fill(r rune, style tcell.Style) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cells.Fill(r, style)
}

func (s *SimScreen) Put(x, y int, str string, style tcell.Style) (string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cells.Put(x, y, str, style)
}

func (s *SimScreen) PutStr(x, y int, str string) {
	s.PutStrStyled(x, y, str, tcell.StyleDefault)
}

func (s *SimScreen) PutStrStyled(x, y int, str string, style tcell.Style) {
	s.mu.Lock()
	defer s.mu.Unlock()
	width := 0
	for str != "" && x < s.width && y < s.height {
		str, width = s.cells.Put(x, y, str, style)
		if width == 0 {
			break
		}
		x += width
	}
}

func (s *SimScreen) Get(x, y int) (string, tcell.Style, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cells.Get(x, y)
}

func (s *SimScreen) SetContent(x, y int, primary rune, combining []rune, style tcell.Style) {
	s.Put(x, y, string(append([]rune{primary}, combining...)), style)
}

func (s *SimScreen) SetStyle(style tcell.Style) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.style = style
}

func (s *SimScreen) ShowCursor(x, y int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cursorX, s.cursorY = x, y
}

func (s *SimScreen) HideCursor() {
	s.ShowCursor(-1, -1)
}

func (s *SimScreen) SetCursorStyle(tcell.CursorStyle, ...color.Color) {}

// GetCursor returns the last cursor position set via ShowCursor
// ((-1, -1) when hidden).
func (s *SimScreen) GetCursor() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cursorX, s.cursorY
}

func (s *SimScreen) Size() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.width, s.height
}

func (s *SimScreen) EventQ() chan tcell.Event {
	return s.events
}

func (s *SimScreen) EnableMouse(...tcell.MouseFlags) {}
func (s *SimScreen) DisableMouse()                   {}
func (s *SimScreen) EnablePaste()                    {}
func (s *SimScreen) DisablePaste()                   {}
func (s *SimScreen) EnableFocus()                    {}
func (s *SimScreen) DisableFocus()                   {}

func (s *SimScreen) Colors() int {
	return 1 << 24
}

func (s *SimScreen) Show() {}
func (s *SimScreen) Sync() {}

func (s *SimScreen) CharacterSet() string {
	return "UTF-8"
}

func (s *SimScreen) RegisterRuneFallback(rune, string) {}
func (s *SimScreen) UnregisterRuneFallback(rune)       {}

func (s *SimScreen) Resize(int, int, int, int) {}

func (s *SimScreen) Suspend() error { return nil }
func (s *SimScreen) Resume() error  { return nil }
func (s *SimScreen) Beep() error    { return nil }

func (s *SimScreen) SetSize(w, h int) {
	s.mu.Lock()
	s.width, s.height = w, h
	s.cells.Resize(w, h)
	s.mu.Unlock()
	// Mirror real screens: a resize is announced through the event queue.
	select {
	case s.events <- tcell.NewEventResize(w, h):
	default:
	}
}

func (s *SimScreen) LockRegion(int, int, int, int, bool) {}

func (s *SimScreen) SetTty(tty tcell.Tty) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tty = tty
}

func (s *SimScreen) Tty() (tcell.Tty, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tty, s.tty != nil
}

func (s *SimScreen) SetTitle(string)                 {}
func (s *SimScreen) SetClipboard([]byte)             {}
func (s *SimScreen) GetClipboard()                   {}
func (s *SimScreen) HasClipboard() bool              { return false }
func (s *SimScreen) ShowNotification(string, string) {}

func (s *SimScreen) KeyboardProtocol() tcell.KeyProtocol {
	return 0
}

func (s *SimScreen) Terminal() (string, string) {
	return "simulation", ""
}

// GetContents returns a snapshot of the screen contents for test assertions.
// The slice is row-major (index y*width+x). Continuation cells of wide runes
// have Str == "" so lines can be reassembled by simple concatenation.
func (s *SimScreen) GetContents() ([]SimCell, int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SimCell, 0, s.width*s.height)
	for y := 0; y < s.height; y++ {
		for x := 0; x < s.width; x++ {
			str, style, width := s.cells.Get(x, y)
			out = append(out, SimCell{Str: str, Style: style})
			for skip := 1; skip < width && x+1 < s.width; skip++ {
				out = append(out, SimCell{Style: style})
				x++
			}
		}
	}
	return out, s.width, s.height
}
