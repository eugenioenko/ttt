package ui

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/eugenioenko/ttt/internal/core/clipboard"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/terminal"

	"github.com/eugenioenko/vt10x"
	"github.com/gdamore/tcell/v3"
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

type linkSpan struct {
	StartCol int
	EndCol   int // exclusive
	URL      string
	IsFile   bool
	FilePath string
	Line     int // 1-based line number for file links (0 = no line)
	Col      int // 1-based column number for file links (0 = no column)
}

var (
	urlRe      = regexp.MustCompile(`https?://[^\s)>\]'"` + "`" + `]+`)
	fileLineRe = regexp.MustCompile(`(?:^|[\s(])([^\s:*?"<>|]+):(\d+)(?::(\d+))?`)
)

// linkMemoMax bounds the per-widget link detection memo. When the memo grows
// past this many unique line texts it is reset, so long sessions with lots of
// distinct output do not accumulate memory indefinitely.
const linkMemoMax = 512

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

	OnOpenURL  func(url string)
	OnOpenFile func(path string, line, col int)
	WorkDir    string
	ctrlHeld   bool
	linkCache  map[int][]linkSpan
	linkMemo   map[string][]linkSpan

	// mouseButtonHeld is the SGR button code (0/1/2) of a press currently being
	// forwarded to the PTY, or -1 when none is held. Needed to encode the
	// matching release and to distinguish a drag (button held) from bare
	// hover motion, which use different vt10x mouse-tracking modes.
	mouseButtonHeld int
}

func NewTerminalWidget(t *terminal.Terminal, palette *TerminalColorPalette) *TerminalWidget {
	return &TerminalWidget{
		Term:            t,
		Palette:         palette,
		mouseButtonHeld: -1,
	}
}

func (tw *TerminalWidget) Focusable() bool { return true }

func (tw *TerminalWidget) SetFocused(f bool) { tw.focused = f }

func (tw *TerminalWidget) WantsRawKeys() bool { return tw.focused }

func (tw *TerminalWidget) PasteText(text string) {
	if tw.Term == nil || text == "" {
		return
	}
	if tw.Term.Mode()&vt10x.ModeBracketedPaste != 0 {
		tw.Term.WriteString("\x1b[200~")
		tw.Term.WriteString(text)
		tw.Term.WriteString("\x1b[201~")
	} else {
		tw.Term.WriteString(text)
	}
	tw.ClearSelection()
	tw.scrollOffset = 0
}

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

func byteToRunePos(s string, byteIdx int) int {
	return len([]rune(s[:byteIdx]))
}

func detectLinks(text string, workDir string) []linkSpan {
	var spans []linkSpan

	for _, loc := range urlRe.FindAllStringIndex(text, -1) {
		url := text[loc[0]:loc[1]]
		for len(url) > 0 {
			last := url[len(url)-1]
			if last == '.' || last == ',' || last == ';' || last == ':' {
				url = url[:len(url)-1]
			} else {
				break
			}
		}
		startCol := byteToRunePos(text, loc[0])
		endCol := startCol + len([]rune(url))
		spans = append(spans, linkSpan{
			StartCol: startCol,
			EndCol:   endCol,
			URL:      url,
		})
	}

	for _, match := range fileLineRe.FindAllStringSubmatchIndex(text, -1) {
		filePath := text[match[2]:match[3]]
		lineStr := text[match[4]:match[5]]
		lineNum, err := strconv.Atoi(lineStr)
		if err != nil {
			continue
		}

		resolvedPath := resolveFilePath(filePath, workDir)
		if resolvedPath == "" {
			continue
		}

		spanEnd := match[5]
		colNum := 0
		if match[6] != -1 {
			spanEnd = match[7]
			if c, err := strconv.Atoi(text[match[6]:match[7]]); err == nil {
				colNum = c
			}
		}

		startCol := byteToRunePos(text, match[2])
		endCol := byteToRunePos(text, spanEnd)
		overlaps := false
		for _, existing := range spans {
			if startCol < existing.EndCol && endCol > existing.StartCol {
				overlaps = true
				break
			}
		}
		if overlaps {
			continue
		}

		spans = append(spans, linkSpan{
			StartCol: startCol,
			EndCol:   endCol,
			IsFile:   true,
			FilePath: resolvedPath,
			Line:     lineNum,
			Col:      colNum,
		})
	}

	return spans
}

func resolveFilePath(path string, workDir string) string {
	if path == "" {
		return ""
	}

	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	if filepath.IsAbs(path) {
		if _, err := os.Stat(path); err == nil {
			return path
		}
		return ""
	}

	if workDir != "" {
		abs := filepath.Join(workDir, path)
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
	}

	return ""
}

func extractLineText(view vt10x.View, unifiedLine int, maxCols int) string {
	cols, rows := view.Size()
	if maxCols > cols {
		maxCols = cols
	}
	sbLen := view.ScrollbackLen()

	var sb strings.Builder
	if unifiedLine < sbLen {
		sl := view.ScrollbackLine(unifiedLine)
		for x := 0; x < maxCols; x++ {
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
		liveRow := unifiedLine - sbLen
		if liveRow >= 0 && liveRow < rows {
			for x := 0; x < maxCols; x++ {
				ch := view.Cell(x, liveRow).Char
				if ch == 0 {
					ch = ' '
				}
				sb.WriteRune(ch)
			}
		}
	}
	return sb.String()
}

// linksForLine returns the link spans for a line of terminal text, memoized
// by the line's text. Detection runs the regexes plus an os.Stat per file
// candidate, so caching per unique text keeps Render cheap while Ctrl is held
// (unchanged lines cost a map lookup per frame instead of stats).
func (tw *TerminalWidget) linksForLine(text string) []linkSpan {
	if spans, ok := tw.linkMemo[text]; ok {
		return spans
	}
	if tw.linkMemo == nil || len(tw.linkMemo) >= linkMemoMax {
		tw.linkMemo = make(map[string][]linkSpan)
	}
	spans := detectLinks(text, tw.WorkDir)
	tw.linkMemo[text] = spans
	return spans
}

func (tw *TerminalWidget) linkAt(unifiedLine, col int) *linkSpan {
	spans, ok := tw.linkCache[unifiedLine]
	if !ok {
		return nil
	}
	for i := range spans {
		if col >= spans[i].StartCol && col < spans[i].EndCol {
			return &spans[i]
		}
	}
	return nil
}

func (tw *TerminalWidget) ScrollToBottom() {
	tw.scrollOffset = 0
}

func (tw *TerminalWidget) IsScrolledUp() bool {
	return tw.scrollOffset > 0
}

func (tw *TerminalWidget) Render(surface Surface) {
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

		if tw.ctrlHeld {
			tw.linkCache = make(map[int][]linkSpan)
		} else {
			tw.linkCache = nil
		}

		if tw.scrollOffset == 0 {
			for y := 0; y < h && y < rows; y++ {
				unifiedLine := sbLen + y
				if tw.ctrlHeld {
					lineText := extractLineText(view, unifiedLine, contentW)
					tw.linkCache[unifiedLine] = tw.linksForLine(lineText)
				}

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
					} else if tw.linkAt(unifiedLine, x) != nil {
						c.Attrs |= term.CellAttrUnderline
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
				if tw.ctrlHeld {
					lineText := extractLineText(view, srcLine, contentW)
					tw.linkCache[srcLine] = tw.linksForLine(lineText)
				}

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
						} else if tw.linkAt(srcLine, x) != nil {
							c.Attrs |= term.CellAttrUnderline
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
							} else if tw.linkAt(srcLine, x) != nil {
								c.Attrs |= term.CellAttrUnderline
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
		tw.ctrlHeld = tev.Modifiers()&tcell.ModCtrl != 0

		if newTop, consumed := tw.scrollbar.HandleEvent(ev); consumed {
			if newTop >= tw.scrollbar.TotalItems-tw.scrollbar.Height {
				// Track end: go live instead of using TotalItems, stale vs a streaming PTY.
				tw.scrollOffset = 0
			} else {
				sbLen := tw.Term.ScrollbackLen()
				tw.scrollOffset = sbLen - newTop
				if tw.scrollOffset < 0 {
					tw.scrollOffset = 0
				}
			}
			if tw.scrollbar.IsDragging() {
				return EventCaptured
			}
			return EventConsumed
		}
		btn := tev.Buttons()
		mx, my := tev.Position()
		mouseReporting := tw.Term.Mode()&vt10x.ModeMouseMask != 0

		if btn&(tcell.WheelUp|tcell.WheelDown|tcell.WheelLeft|tcell.WheelRight) != 0 {
			if mouseReporting {
				if code, ok := sgrWheelCode(btn); ok {
					col, row := tw.mousePTYCoords(mx, my)
					tw.Term.WriteString(encodeSGRMouse(code|sgrModifiers(tev.Modifiers()), col, row, false))
				}
				return EventConsumed
			}
			switch {
			case btn&tcell.WheelUp != 0:
				tw.scrollUp(3)
			case btn&tcell.WheelDown != 0:
				tw.scrollDown(3)
			}
			return EventConsumed
		}

		if btn&tcell.Button1 != 0 && tw.ctrlHeld {
			pos := tw.screenToLine(mx, my)
			if link := tw.linkAt(pos.Line, pos.Col); link != nil {
				if link.IsFile && tw.OnOpenFile != nil {
					tw.OnOpenFile(link.FilePath, link.Line, link.Col)
				} else if !link.IsFile && tw.OnOpenURL != nil {
					tw.OnOpenURL(link.URL)
				}
				return EventConsumed
			}
			// ctrl+click with no link underneath: fall through to local
			// selection below, even if the app wants mouse reporting —
			// ctrl+click is a ttt-level affordance, not something to forward.
		}

		if mouseReporting && !tw.ctrlHeld {
			if code, ok := sgrButtonCode(btn); ok {
				held := tw.mouseButtonHeld == code
				if !held || tw.Term.Mode()&(vt10x.ModeMouseMotion|vt10x.ModeMouseMany) != 0 {
					sgrCode := code
					if held {
						sgrCode |= sgrMotionFlag
					}
					col, row := tw.mousePTYCoords(mx, my)
					tw.Term.WriteString(encodeSGRMouse(sgrCode|sgrModifiers(tev.Modifiers()), col, row, false))
				}
				tw.mouseButtonHeld = code
				return EventCaptured
			}
			if btn == tcell.ButtonNone {
				if tw.mouseButtonHeld >= 0 {
					col, row := tw.mousePTYCoords(mx, my)
					tw.Term.WriteString(encodeSGRMouse(tw.mouseButtonHeld|sgrModifiers(tev.Modifiers()), col, row, true))
					tw.mouseButtonHeld = -1
					return EventConsumed
				}
				if tw.Term.Mode()&vt10x.ModeMouseMany != 0 {
					col, row := tw.mousePTYCoords(mx, my)
					tw.Term.WriteString(encodeSGRMouse(sgrButtonNone|sgrMotionFlag|sgrModifiers(tev.Modifiers()), col, row, false))
					return EventConsumed
				}
				return EventIgnored
			}
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

// SGR extended mouse mode (\x1b[<Cb;Cx;CyM/m, DECSET 1006) encoding. This is
// the modern mouse-reporting encoding virtually every full-screen TUI that
// asks for mouse input expects; legacy X10 encoding (byte-limited to 223
// columns/rows) is intentionally not supported.
const (
	sgrButtonLeft   = 0
	sgrButtonMiddle = 1
	sgrButtonRight  = 2
	sgrButtonNone   = 3 // "no button" motion code, used only with the motion flag
	sgrWheelUp      = 64
	sgrWheelDown    = 65
	sgrWheelLeft    = 66
	sgrWheelRight   = 67
	sgrMotionFlag   = 32
	sgrShiftMod     = 4
	sgrAltMod       = 8
	sgrCtrlMod      = 16
)

func sgrModifiers(mod tcell.ModMask) int {
	m := 0
	if mod&tcell.ModShift != 0 {
		m |= sgrShiftMod
	}
	if mod&tcell.ModAlt != 0 {
		m |= sgrAltMod
	}
	if mod&tcell.ModCtrl != 0 {
		m |= sgrCtrlMod
	}
	return m
}

func sgrButtonCode(btn tcell.ButtonMask) (code int, ok bool) {
	switch {
	case btn&tcell.Button1 != 0:
		return sgrButtonLeft, true
	case btn&tcell.Button2 != 0:
		return sgrButtonMiddle, true
	case btn&tcell.Button3 != 0:
		return sgrButtonRight, true
	}
	return 0, false
}

func sgrWheelCode(btn tcell.ButtonMask) (code int, ok bool) {
	switch {
	case btn&tcell.WheelUp != 0:
		return sgrWheelUp, true
	case btn&tcell.WheelDown != 0:
		return sgrWheelDown, true
	case btn&tcell.WheelLeft != 0:
		return sgrWheelLeft, true
	case btn&tcell.WheelRight != 0:
		return sgrWheelRight, true
	}
	return 0, false
}

func encodeSGRMouse(code, col, row int, release bool) string {
	suffix := byte('M')
	if release {
		suffix = 'm'
	}
	return fmt.Sprintf("\x1b[<%d;%d;%d%c", code, col, row, suffix)
}

// mousePTYCoords maps a screen position to 1-based coordinates within the
// terminal's own column/row grid, clamped to its bounds.
func (tw *TerminalWidget) mousePTYCoords(mx, my int) (col, row int) {
	r := tw.GetRect()
	col = mx - r.X + 1
	row = my - r.Y + 1
	if col < 1 {
		col = 1
	}
	if row < 1 {
		row = 1
	}
	if cols, rows := tw.Term.Size(); cols > 0 && rows > 0 {
		if col > cols {
			col = cols
		}
		if row > rows {
			row = rows
		}
	}
	return col, row
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
		// tcell v3 reports ctrl+non-letter printables (ctrl+space, ctrl+/,
		// ctrl+`...) as KeyRune with ModCtrl instead of folding them into
		// control-key constants. Encode them as the control byte the shell
		// expects rather than the literal character.
		if mod&tcell.ModCtrl != 0 {
			if ctrl, ok := ctrlByteForRune(term.KeyRune(ev)); ok {
				if mod&tcell.ModAlt != 0 {
					return "\x1b" + ctrl
				}
				return ctrl
			}
		}
		if mod&tcell.ModAlt != 0 {
			return "\x1b" + term.KeyStr(ev)
		}
		return term.KeyStr(ev)
	}

	switch ev.Key() {
	case tcell.KeyEnter:
		return "\r"
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return "\x7f"
	case tcell.KeyTab:
		return "\t"
	case tcell.KeyBacktab:
		return "\x1b[Z"
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

// ctrlByteForRune maps a printable character typed with Ctrl held to the
// ASCII control byte a terminal would traditionally emit (ch & 0x1F for
// @ A-Z a-z [ \ ] ^ _, NUL for space and backtick, DEL for ?).
func ctrlByteForRune(r rune) (string, bool) {
	switch {
	case r == ' ', r == '`', r == '@':
		return "\x00", true
	case r >= 'a' && r <= 'z':
		return string(rune(r - 'a' + 1)), true
	case r >= 'A' && r <= 'Z':
		return string(rune(r - 'A' + 1)), true
	case r == '[', r == '\\', r == ']', r == '^', r == '_':
		return string(rune(r & 0x1f)), true
	case r == '/':
		return "\x1f", true
	case r == '?':
		return "\x7f", true
	}
	return "", false
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
	cube := [6]byte{0, 95, 135, 175, 215, 255}
	for i := 16; i < 232; i++ {
		idx := i - 16
		p[i] = term.DirectColor{R: cube[idx/36], G: cube[(idx/6)%6], B: cube[idx%6], Set: true}
	}
	// 232-255: grayscale
	for i := 232; i < 256; i++ {
		v := byte((i-232)*10 + 8)
		p[i] = term.DirectColor{R: v, G: v, B: v, Set: true}
	}
	return p
}
