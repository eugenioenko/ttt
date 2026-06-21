package ui

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/terminal"

	"github.com/eugenioenko/vt10x"
	"github.com/gdamore/tcell/v2"
)

type TerminalColorPalette struct {
	Fg       term.DirectColor
	Bg       term.DirectColor
	ANSI     [16]term.DirectColor
	Color256 [256]term.DirectColor
}

type termSelPos struct {
	Line, Col int // Line = unified index (0 = oldest scrollback)
}

type TerminalWidget struct {
	BaseWidget
	Term         *terminal.Terminal
	Palette      *TerminalColorPalette
	focused      bool
	scrollOffset int
	scrollbar    Scrollbar
	selecting    bool
	hasSelection bool
	selAnchor    termSelPos
	selCurrent   termSelPos
}

func NewTerminalWidget(t *terminal.Terminal, palette *TerminalColorPalette) *TerminalWidget {
	return &TerminalWidget{
		Term:    t,
		Palette: palette,
	}
}

func (tw *TerminalWidget) Focusable() bool { return true }

func (tw *TerminalWidget) SetFocused(f bool) { tw.focused = f }

func (tw *TerminalWidget) WantsRawKeys() bool { return tw.focused }

func (tw *TerminalWidget) CursorPosition() (x, y int, visible bool) {
	if tw.Term == nil {
		return 0, 0, false
	}
	if tw.scrollOffset > 0 || tw.hasSelection {
		return 0, 0, false
	}
	r := tw.GetRect()
	cx, cy := tw.Term.CursorPos()
	return r.X + cx, r.Y + cy, tw.focused
}

func (tw *TerminalWidget) ScrollToBottom() {
	tw.scrollOffset = 0
}

func (tw *TerminalWidget) IsScrolledUp() bool {
	return tw.scrollOffset > 0
}

func (tw *TerminalWidget) Render(surface *RenderSurface) {
	if tw.Term == nil {
		return
	}
	w, h := surface.Size()
	r := tw.GetRect()

	tw.Term.Snapshot(func(view vt10x.View) {
		cols, rows := view.Size()
		sbLen := view.ScrollbackLen()
		totalLines := sbLen + rows

		if tw.scrollOffset > sbLen {
			tw.scrollOffset = sbLen
		}

		showScrollbar := sbLen > 0
		contentW := w
		if showScrollbar {
			contentW = w - 1
		}

		if tw.scrollOffset == 0 {
			for y := 0; y < h && y < rows; y++ {
				unifiedLine := sbLen + y
				for x := 0; x < contentW && x < cols; x++ {
					c := tw.glyphToCell(view.Cell(x, y))
					if tw.isCellSelected(unifiedLine, x) {
						c.Fg, c.Bg = c.Bg, c.Fg
						if !c.Fg.Set {
							c.Fg = tw.Palette.Bg
						}
						if !c.Bg.Set {
							c.Bg = tw.Palette.Fg
						}
					}
					surface.SetCell(x, y, c)
				}
			}
		} else {
			startLine := totalLines - tw.scrollOffset - h
			if startLine < 0 {
				startLine = 0
			}

			for screenY := 0; screenY < h; screenY++ {
				srcLine := startLine + screenY
				if srcLine < sbLen {
					sl := view.ScrollbackLine(srcLine)
					for x := 0; x < contentW; x++ {
						var c term.Cell
						if sl != nil && x < len(sl) {
							c = tw.glyphToCell(sl[x])
						} else {
							c = term.Cell{Ch: ' ', Direct: true, Bg: tw.Palette.Bg}
						}
						if tw.isCellSelected(srcLine, x) {
							c.Fg, c.Bg = c.Bg, c.Fg
							if !c.Fg.Set {
								c.Fg = tw.Palette.Bg
							}
							if !c.Bg.Set {
								c.Bg = tw.Palette.Fg
							}
						}
						surface.SetCell(x, screenY, c)
					}
				} else {
					liveRow := srcLine - sbLen
					if liveRow >= 0 && liveRow < rows {
						for x := 0; x < contentW && x < cols; x++ {
							c := tw.glyphToCell(view.Cell(x, liveRow))
							if tw.isCellSelected(srcLine, x) {
								c.Fg, c.Bg = c.Bg, c.Fg
								if !c.Fg.Set {
									c.Fg = tw.Palette.Bg
								}
								if !c.Bg.Set {
									c.Bg = tw.Palette.Fg
								}
							}
							surface.SetCell(x, screenY, c)
						}
					}
				}
			}
		}

		if showScrollbar {
			topItem := sbLen - tw.scrollOffset
			if topItem < 0 {
				topItem = 0
			}
			tw.scrollbar.X = r.X + w - 1
			tw.scrollbar.Y = r.Y
			tw.scrollbar.Height = h
			tw.scrollbar.TotalItems = totalLines
			tw.scrollbar.TopItem = topItem
			tw.scrollbar.Render(surface, w-1, 0)
		}
	})
}

func (tw *TerminalWidget) glyphToCell(g vt10x.Glyph) term.Cell {
	ch := g.Char
	if ch == 0 {
		ch = ' '
	}

	cell := term.Cell{
		Ch:     ch,
		Direct: true,
		Fg:     tw.resolveColor(g.FG, true),
		Bg:     tw.resolveColor(g.BG, false),
	}

	if g.Mode&terminal.AttrBold != 0 {
		cell.Attrs |= term.CellAttrBold
	}
	if g.Mode&terminal.AttrUnderline != 0 {
		cell.Attrs |= term.CellAttrUnderline
	}
	if g.Mode&terminal.AttrItalic != 0 {
		cell.Attrs |= term.CellAttrItalic
	}
	if g.Mode&terminal.AttrReverse != 0 {
		cell.Attrs |= term.CellAttrReverse
	}
	if g.Mode&terminal.AttrBlink != 0 {
		cell.Attrs |= term.CellAttrBlink
	}

	return cell
}

func (tw *TerminalWidget) resolveColor(c vt10x.Color, isFg bool) term.DirectColor {
	if c == vt10x.DefaultFG {
		return tw.Palette.Fg
	}
	if c == vt10x.DefaultBG {
		return tw.Palette.Bg
	}
	if c.TrueColor() {
		r, g, b := c.RGB()
		return term.DirectColor{R: r, G: g, B: b, Set: true}
	}
	idx := int(c)
	if idx >= 0 && idx < 16 {
		return tw.Palette.ANSI[idx]
	}
	if idx >= 16 && idx < 256 {
		return tw.Palette.Color256[idx]
	}
	if isFg {
		return tw.Palette.Fg
	}
	return tw.Palette.Bg
}

func (tw *TerminalWidget) HandleEvent(ev tcell.Event) EventResult {
	if tw.Term == nil {
		return EventIgnored
	}

	switch tev := ev.(type) {
	case *tcell.EventKey:
		if tev.Modifiers()&tcell.ModShift != 0 {
			_, h := tw.Term.Size()
			switch tev.Key() {
			case tcell.KeyPgUp:
				tw.scrollUp(h / 2)
				return EventConsumed
			case tcell.KeyPgDn:
				tw.scrollDown(h / 2)
				return EventConsumed
			}
		}

		if tev.Key() == tcell.KeyCtrlC && tw.hasSelection {
			text := tw.selectedText()
			if text != "" {
				clipboard.Set(text)
			}
			tw.ClearSelection()
			return EventConsumed
		}

		if tev.Key() == tcell.KeyCtrlV {
			text := clipboard.Get()
			if text != "" {
				if tw.Term.Mode()&vt10x.ModeBracketedPaste != 0 {
					tw.Term.WriteString("\x1b[200~")
					tw.Term.WriteString(text)
					tw.Term.WriteString("\x1b[201~")
				} else {
					tw.Term.WriteString(text)
				}
			}
			tw.ClearSelection()
			return EventConsumed
		}

		tw.ClearSelection()
		tw.scrollOffset = 0
		data := keyToVT(tev)
		if data != "" {
			tw.Term.WriteString(data)
			return EventConsumed
		}
	case *tcell.EventMouse:
		if newTop, consumed := tw.scrollbar.HandleEvent(ev); consumed {
			sbLen := tw.Term.ScrollbackLen()
			tw.scrollOffset = sbLen - newTop
			if tw.scrollOffset < 0 {
				tw.scrollOffset = 0
			}
			if tw.scrollbar.IsDragging() {
				return EventCaptured
			}
			return EventConsumed
		}
		btn := tev.Buttons()
		mx, my := tev.Position()
		if btn&tcell.WheelUp != 0 {
			tw.scrollUp(3)
			return EventConsumed
		}
		if btn&tcell.WheelDown != 0 {
			tw.scrollDown(3)
			return EventConsumed
		}
		if btn&tcell.Button1 != 0 {
			pos := tw.screenToLine(mx, my)
			if !tw.selecting {
				tw.selecting = true
				tw.hasSelection = true
				tw.selAnchor = pos
				tw.selCurrent = pos
			} else {
				tw.selCurrent = pos
			}
			return EventCaptured
		}
		if tw.selecting {
			tw.selecting = false
			start, end := tw.selectionRange()
			if start.Line == end.Line && start.Col == end.Col {
				tw.hasSelection = false
			}
		}
		return EventIgnored
	}

	return EventIgnored
}

func (tw *TerminalWidget) ClearSelection() {
	tw.hasSelection = false
	tw.selecting = false
}

func (tw *TerminalWidget) selectionRange() (start, end termSelPos) {
	a, b := tw.selAnchor, tw.selCurrent
	if a.Line < b.Line || (a.Line == b.Line && a.Col <= b.Col) {
		return a, b
	}
	return b, a
}

func (tw *TerminalWidget) screenToLine(mx, my int) termSelPos {
	r := tw.GetRect()
	col := mx - r.X
	screenY := my - r.Y
	unifiedLine := 0

	tw.Term.Snapshot(func(view vt10x.View) {
		_, rows := view.Size()
		sbLen := view.ScrollbackLen()
		totalLines := sbLen + rows

		if tw.scrollOffset == 0 {
			unifiedLine = sbLen + screenY
		} else {
			startLine := totalLines - tw.scrollOffset - r.H
			if startLine < 0 {
				startLine = 0
			}
			unifiedLine = startLine + screenY
		}
	})
	return termSelPos{Line: unifiedLine, Col: col}
}

func (tw *TerminalWidget) isCellSelected(unifiedLine, col int) bool {
	if !tw.hasSelection {
		return false
	}
	start, end := tw.selectionRange()
	if unifiedLine < start.Line || unifiedLine > end.Line {
		return false
	}
	if start.Line == end.Line {
		return col >= start.Col && col < end.Col
	}
	if unifiedLine == start.Line {
		return col >= start.Col
	}
	if unifiedLine == end.Line {
		return col < end.Col
	}
	return true
}

func (tw *TerminalWidget) selectedText() string {
	if !tw.hasSelection {
		return ""
	}
	start, end := tw.selectionRange()
	var lines []string

	tw.Term.Snapshot(func(view vt10x.View) {
		cols, rows := view.Size()
		sbLen := view.ScrollbackLen()

		for line := start.Line; line <= end.Line; line++ {
			startCol := 0
			endCol := cols
			if line == start.Line {
				startCol = start.Col
			}
			if line == end.Line {
				endCol = end.Col
			}

			var sb strings.Builder
			if line < sbLen {
				sl := view.ScrollbackLine(line)
				for x := startCol; x < endCol; x++ {
					if sl != nil && x < len(sl) {
						ch := sl[x].Char
						if ch == 0 {
							ch = ' '
						}
						sb.WriteRune(ch)
					} else {
						sb.WriteByte(' ')
					}
				}
			} else {
				liveRow := line - sbLen
				if liveRow >= 0 && liveRow < rows {
					for x := startCol; x < endCol && x < cols; x++ {
						ch := view.Cell(x, liveRow).Char
						if ch == 0 {
							ch = ' '
						}
						sb.WriteRune(ch)
					}
				}
			}
			lines = append(lines, strings.TrimRight(sb.String(), " "))
		}
	})

	return strings.Join(lines, "\n")
}

func (tw *TerminalWidget) scrollUp(n int) {
	maxOffset := tw.Term.ScrollbackLen()
	tw.scrollOffset += n
	if tw.scrollOffset > maxOffset {
		tw.scrollOffset = maxOffset
	}
	slog.Debug("terminal scroll up", "scrollOffset", tw.scrollOffset, "maxOffset", maxOffset)
}

func (tw *TerminalWidget) scrollDown(n int) {
	tw.scrollOffset -= n
	if tw.scrollOffset < 0 {
		tw.scrollOffset = 0
	}
	slog.Debug("terminal scroll down", "scrollOffset", tw.scrollOffset)
}

func keyToVT(ev *tcell.EventKey) string {
	if ev.Key() == tcell.KeyRune {
		mod := ev.Modifiers()
		if mod&tcell.ModAlt != 0 {
			return fmt.Sprintf("\x1b%c", ev.Rune())
		}
		return string(ev.Rune())
	}

	switch ev.Key() {
	case tcell.KeyEnter:
		return "\r"
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return "\x7f"
	case tcell.KeyTab:
		return "\t"
	case tcell.KeyEscape:
		return "\x1b"
	case tcell.KeyUp:
		return "\x1b[A"
	case tcell.KeyDown:
		return "\x1b[B"
	case tcell.KeyRight:
		return "\x1b[C"
	case tcell.KeyLeft:
		return "\x1b[D"
	case tcell.KeyHome:
		return "\x1b[H"
	case tcell.KeyEnd:
		return "\x1b[F"
	case tcell.KeyInsert:
		return "\x1b[2~"
	case tcell.KeyDelete:
		return "\x1b[3~"
	case tcell.KeyPgUp:
		return "\x1b[5~"
	case tcell.KeyPgDn:
		return "\x1b[6~"
	case tcell.KeyF1:
		return "\x1bOP"
	case tcell.KeyF2:
		return "\x1bOQ"
	case tcell.KeyF3:
		return "\x1bOR"
	case tcell.KeyF4:
		return "\x1bOS"
	case tcell.KeyF5:
		return "\x1b[15~"
	case tcell.KeyF6:
		return "\x1b[17~"
	case tcell.KeyF7:
		return "\x1b[18~"
	case tcell.KeyF8:
		return "\x1b[19~"
	case tcell.KeyF9:
		return "\x1b[20~"
	case tcell.KeyF10:
		return "\x1b[21~"
	case tcell.KeyF11:
		return "\x1b[23~"
	case tcell.KeyF12:
		return "\x1b[24~"
	}

	if ev.Key() >= tcell.KeyCtrlA && ev.Key() <= tcell.KeyCtrlZ {
		return string(rune(ev.Key() - tcell.KeyCtrlA + 1))
	}

	return ""
}

func ParseHexColor(hex string) term.DirectColor {
	if len(hex) == 0 {
		return term.DirectColor{}
	}
	if hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) != 6 {
		return term.DirectColor{}
	}
	r := hexByte(hex[0:2])
	g := hexByte(hex[2:4])
	b := hexByte(hex[4:6])
	return term.DirectColor{R: r, G: g, B: b, Set: true}
}

func hexByte(s string) byte {
	var v byte
	for _, c := range s {
		v <<= 4
		switch {
		case c >= '0' && c <= '9':
			v |= byte(c - '0')
		case c >= 'a' && c <= 'f':
			v |= byte(c - 'a' + 10)
		case c >= 'A' && c <= 'F':
			v |= byte(c - 'A' + 10)
		}
	}
	return v
}

func Build256Palette() [256]term.DirectColor {
	var p [256]term.DirectColor
	// 16-231: 6x6x6 color cube
	for i := 16; i < 232; i++ {
		idx := i - 16
		r := byte((idx / 36) * 51)
		g := byte(((idx / 6) % 6) * 51)
		b := byte((idx % 6) * 51)
		p[i] = term.DirectColor{R: r, G: g, B: b, Set: true}
	}
	// 232-255: grayscale
	for i := 232; i < 256; i++ {
		v := byte((i-232)*10 + 8)
		p[i] = term.DirectColor{R: v, G: v, B: v, Set: true}
	}
	return p
}
