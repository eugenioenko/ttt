package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v2"
)

// CommentItem represents a single comment displayed in the panel.
type CommentItem struct {
	ID        int
	Author    string
	Timestamp string
	Body      string
	FilePath  string // non-empty for inline comments
	Line      int    // line number for inline comments
	IsInline  bool
	InReplyTo int
}

// CommentPanelWidget displays PR review comments in a sliding right panel
// with an email/chat thread aesthetic.
type CommentPanelWidget struct {
	BaseWidget

	Title    string // e.g. "PR #42: Fix bug"
	Comments []CommentItem
	Borders  *term.BorderSet
	Loading  bool

	// Callbacks
	OnClose    func()
	OnSubmit   func(body string)
	OnOpenFile func(path string, line int)

	// Scroll state
	scrollTop  int
	totalLines int // total visual lines in rendered thread

	// Compose area
	Input     *InputWidget
	composing bool

	// Close button hit region
	closeHit HitRegion

	// File reference hit regions
	fileHits []fileHitRegion

	// Scrollbar
	scrollbar Scrollbar
}

type fileHitRegion struct {
	HitRegion
	Path string
	Line int
}

// NewCommentPanelWidget creates a new comment panel.
func NewCommentPanelWidget(title string) *CommentPanelWidget {
	inp := NewInputWidget()
	inp.Prefix = " "
	inp.Placeholder = "Write a comment..."
	return &CommentPanelWidget{
		Title: title,
		Input: inp,
	}
}

func (c *CommentPanelWidget) Focusable() bool { return true }

// SetComments replaces the comment list and resets scroll.
func (c *CommentPanelWidget) SetComments(items []CommentItem) {
	c.Comments = items
	c.scrollTop = 0
	c.Loading = false
}

// panelWidth calculates the panel width (~40% of screen, min 30, max 80).
func (c *CommentPanelWidget) panelWidth(screenW int) int {
	w := screenW * 40 / 100
	if w < 30 {
		w = 30
	}
	if w > 80 {
		w = 80
	}
	if w > screenW-10 {
		w = screenW - 10
	}
	return w
}

// wrapText breaks text into lines that fit within the given width.
func wrapText(text string, width int) []string {
	if width <= 0 {
		return nil
	}
	var result []string
	for _, paragraph := range strings.Split(text, "\n") {
		if paragraph == "" {
			result = append(result, "")
			continue
		}
		runes := []rune(paragraph)
		for len(runes) > 0 {
			if len(runes) <= width {
				result = append(result, string(runes))
				break
			}
			// Find break point
			breakAt := width
			for breakAt > 0 && runes[breakAt] != ' ' {
				breakAt--
			}
			if breakAt == 0 {
				breakAt = width // force break
			}
			result = append(result, string(runes[:breakAt]))
			runes = runes[breakAt:]
			// Skip leading space on next line
			if len(runes) > 0 && runes[0] == ' ' {
				runes = runes[1:]
			}
		}
	}
	return result
}

// formatTimestamp converts an ISO 8601 timestamp to a short relative time.
func formatTimestamp(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		// Try without timezone
		t, err = time.Parse("2006-01-02T15:04:05Z", ts)
		if err != nil {
			return ts
		}
	}
	now := time.Now()
	diff := now.Sub(t)

	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		m := int(diff.Minutes())
		if m == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", m)
	case diff < 24*time.Hour:
		h := int(diff.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case diff < 30*24*time.Hour:
		d := int(diff.Hours() / 24)
		if d == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", d)
	default:
		return t.Format("Jan 2, 2006")
	}
}

// Render draws the comment panel as a right-side overlay.
func (c *CommentPanelWidget) Render(surface *RenderSurface) {
	sw, sh := surface.Size()
	panelW := c.panelWidth(sw)
	panelX := sw - panelW
	panelH := sh

	b := term.SingleBorderSet()
	if c.Borders != nil {
		b = *c.Borders
	}

	// Clear panel area
	surface.ClearRect(panelX, 0, panelW, panelH, term.StyleDefault)

	// Draw left border
	for y := 0; y < panelH; y++ {
		surface.SetCell(panelX, y, term.Cell{Ch: b.Vertical, Style: term.StyleBorder})
	}

	contentX := panelX + 1
	contentW := panelW - 2 // 1 for left border, 1 for scrollbar/right padding

	// --- Header row ---
	headerY := 0
	// Draw header background
	for x := contentX; x < panelX+panelW; x++ {
		surface.SetCell(x, headerY, term.Cell{Ch: ' ', Style: term.StyleStatusBar})
	}

	// Title (bold)
	titleRunes := []rune(c.Title)
	maxTitleW := contentW - 4 // leave room for close button
	tx := contentX + 1
	for i, ch := range titleRunes {
		if i >= maxTitleW {
			break
		}
		surface.SetCell(tx+i, headerY, term.Cell{Ch: ch, Style: term.StyleStatusBar})
	}

	// Close button [X]
	closeX := panelX + panelW - 4
	surface.SetCell(closeX, headerY, term.Cell{Ch: '[', Style: term.StyleStatusBar})
	surface.SetCell(closeX+1, headerY, term.Cell{Ch: 'X', Style: term.StyleDanger})
	surface.SetCell(closeX+2, headerY, term.Cell{Ch: ']', Style: term.StyleStatusBar})
	ox, oy := surface.Origin()
	c.closeHit = HitRegion{X: ox + closeX, Y: oy + headerY, W: 3}

	// Header divider
	headerDivY := 1
	for x := contentX; x < panelX+panelW; x++ {
		surface.SetCell(x, headerDivY, term.Cell{Ch: b.Horizontal, Style: term.StyleBorder})
	}
	// Tees at border intersections
	surface.SetCell(panelX, headerDivY, term.Cell{Ch: b.LeftTee, Style: term.StyleBorder})

	// --- Compose area (bottom) ---
	composeH := 3 // divider + input + hint
	composeDivY := panelH - composeH
	composeInputY := composeDivY + 1
	composeHintY := composeDivY + 2

	// Compose divider
	for x := contentX; x < panelX+panelW; x++ {
		surface.SetCell(x, composeDivY, term.Cell{Ch: b.Horizontal, Style: term.StyleBorder})
	}
	surface.SetCell(panelX, composeDivY, term.Cell{Ch: b.LeftTee, Style: term.StyleBorder})

	// Render input
	inputSurface := surface.Sub(Rect{X: contentX, Y: composeInputY, W: contentW, H: 1})
	c.Input.Render(inputSurface, 0, 0, contentW)

	// Hint text
	hint := " Enter to submit"
	for i, ch := range hint {
		if contentX+i >= panelX+panelW-1 {
			break
		}
		surface.SetCell(contentX+i, composeHintY, term.Cell{Ch: ch, Style: term.StyleMuted})
	}

	// --- Thread area ---
	threadY := 2
	threadH := composeDivY - threadY
	if threadH <= 0 {
		return
	}

	// Compute rendered lines
	c.fileHits = nil
	rendered := c.renderComments(contentW - 1) // -1 for scrollbar space
	c.totalLines = len(rendered)

	// Clamp scroll
	maxScroll := c.totalLines - threadH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if c.scrollTop > maxScroll {
		c.scrollTop = maxScroll
	}
	if c.scrollTop < 0 {
		c.scrollTop = 0
	}

	// Draw visible lines
	for dy := 0; dy < threadH; dy++ {
		lineIdx := c.scrollTop + dy
		if lineIdx >= len(rendered) {
			break
		}
		rl := rendered[lineIdx]
		x := contentX
		for _, cell := range rl.cells {
			if x >= panelX+panelW-1 {
				break
			}
			surface.SetCell(x, threadY+dy, cell)
			x++
		}

		// Track file hit regions
		if rl.filePath != "" {
			c.fileHits = append(c.fileHits, fileHitRegion{
				HitRegion: HitRegion{
					X: ox + contentX,
					Y: oy + threadY + dy,
					W: contentW,
				},
				Path: rl.filePath,
				Line: rl.fileLine,
			})
		}
	}

	// Loading indicator
	if c.Loading {
		msg := "Loading comments..."
		msgX := contentX + (contentW-len(msg))/2
		msgY := threadY + threadH/2
		for i, ch := range msg {
			surface.SetCell(msgX+i, msgY, term.Cell{Ch: ch, Style: term.StyleMuted})
		}
	} else if len(c.Comments) == 0 && !c.Loading {
		msg := "No comments yet"
		msgX := contentX + (contentW-len(msg))/2
		msgY := threadY + threadH/2
		for i, ch := range msg {
			surface.SetCell(msgX+i, msgY, term.Cell{Ch: ch, Style: term.StyleMuted})
		}
	}

	// Scrollbar
	scrollX := panelX + panelW - 1
	c.scrollbar = Scrollbar{
		X:          ox + scrollX,
		Y:          oy + threadY,
		Height:     threadH,
		TotalItems: c.totalLines,
		TopItem:    c.scrollTop,
	}
	c.scrollbar.Render(surface, scrollX, threadY)
}

// renderedLine represents one visual line in the comment thread.
type renderedLine struct {
	cells    []term.Cell
	filePath string // non-empty if this line is a clickable file reference
	fileLine int
}

// renderComments builds the full list of visual lines for all comments.
func (c *CommentPanelWidget) renderComments(width int) []renderedLine {
	if width <= 0 {
		return nil
	}
	var lines []renderedLine

	for i, comment := range c.Comments {
		if i > 0 {
			// Separator line
			sep := make([]term.Cell, width)
			for j := range sep {
				sep[j] = term.Cell{Ch: ' ', Style: term.StyleDefault}
			}
			b := term.SingleBorderSet()
			if c.Borders != nil {
				b = *c.Borders
			}
			for j := 0; j < width; j++ {
				sep[j] = term.Cell{Ch: b.Horizontal, Style: term.StyleBorder}
			}
			lines = append(lines, renderedLine{cells: sep})
			// Blank line after separator
			blank := make([]term.Cell, width)
			for j := range blank {
				blank[j] = term.Cell{Ch: ' ', Style: term.StyleDefault}
			}
			lines = append(lines, renderedLine{cells: blank})
		}

		// Author line: " @author  timestamp"
		authorLine := make([]term.Cell, width)
		for j := range authorLine {
			authorLine[j] = term.Cell{Ch: ' ', Style: term.StyleDefault}
		}
		x := 1
		// Author icon
		authorStr := "@" + comment.Author
		for _, ch := range authorStr {
			if x >= width {
				break
			}
			authorLine[x] = term.Cell{
				Ch: ch, Style: term.StyleHoverBold,
			}
			x++
		}

		// Timestamp (right-aligned or after spacing)
		ts := formatTimestamp(comment.Timestamp)
		tsRunes := []rune(ts)
		tsStart := width - len(tsRunes) - 1
		if tsStart < x+2 {
			tsStart = x + 2
		}
		for j, ch := range tsRunes {
			pos := tsStart + j
			if pos >= width {
				break
			}
			authorLine[pos] = term.Cell{Ch: ch, Style: term.StyleMuted}
		}
		lines = append(lines, renderedLine{cells: authorLine})

		// File reference line for inline comments
		if comment.IsInline && comment.FilePath != "" {
			refLine := make([]term.Cell, width)
			for j := range refLine {
				refLine[j] = term.Cell{Ch: ' ', Style: term.StyleDefault}
			}
			ref := fmt.Sprintf("  %s:%d", comment.FilePath, comment.Line)
			x = 1
			for _, ch := range ref {
				if x >= width {
					break
				}
				refLine[x] = term.Cell{Ch: ch, Style: term.StyleSyntaxString}
				x++
			}
			lines = append(lines, renderedLine{
				cells:    refLine,
				filePath: comment.FilePath,
				fileLine: comment.Line,
			})
		}

		// Blank line before body
		blankBeforeBody := make([]term.Cell, width)
		for j := range blankBeforeBody {
			blankBeforeBody[j] = term.Cell{Ch: ' ', Style: term.StyleDefault}
		}
		lines = append(lines, renderedLine{cells: blankBeforeBody})

		// Body text with word wrapping
		bodyWidth := width - 3 // indentation
		if bodyWidth < 10 {
			bodyWidth = 10
		}
		wrapped := wrapText(comment.Body, bodyWidth)
		for _, wl := range wrapped {
			bodyLine := make([]term.Cell, width)
			for j := range bodyLine {
				bodyLine[j] = term.Cell{Ch: ' ', Style: term.StyleDefault}
			}
			x = 2 // indent body
			for _, ch := range wl {
				if x >= width {
					break
				}
				bodyLine[x] = term.Cell{Ch: ch, Style: term.StyleDefault}
				x++
			}
			lines = append(lines, renderedLine{cells: bodyLine})
		}

		// Trailing blank line
		blankAfterBody := make([]term.Cell, width)
		for j := range blankAfterBody {
			blankAfterBody[j] = term.Cell{Ch: ' ', Style: term.StyleDefault}
		}
		lines = append(lines, renderedLine{cells: blankAfterBody})
	}

	return lines
}

// HandleEvent handles keyboard and mouse events for the comment panel.
func (c *CommentPanelWidget) HandleEvent(ev tcell.Event) EventResult {
	switch tev := ev.(type) {
	case *tcell.EventKey:
		// If composing, route to input
		if c.composing {
			switch tev.Key() {
			case tcell.KeyEscape:
				c.composing = false
				return EventConsumed
			case tcell.KeyEnter:
				text := strings.TrimSpace(c.Input.Text)
				if text != "" && c.OnSubmit != nil {
					c.OnSubmit(text)
					c.Input.Clear()
				}
				c.composing = false
				return EventConsumed
			default:
				return c.Input.HandleEvent(ev)
			}
		}

		switch tev.Key() {
		case tcell.KeyEscape:
			if c.OnClose != nil {
				c.OnClose()
			}
			return EventConsumed
		case tcell.KeyUp:
			if c.scrollTop > 0 {
				c.scrollTop--
			}
			return EventConsumed
		case tcell.KeyDown:
			c.scrollTop++
			return EventConsumed
		case tcell.KeyPgUp:
			c.scrollTop -= 10
			if c.scrollTop < 0 {
				c.scrollTop = 0
			}
			return EventConsumed
		case tcell.KeyPgDn:
			c.scrollTop += 10
			return EventConsumed
		case tcell.KeyRune:
			if tev.Rune() == 'i' || tev.Rune() == 'c' {
				c.composing = true
				return EventConsumed
			}
			if tev.Rune() == 'q' {
				if c.OnClose != nil {
					c.OnClose()
				}
				return EventConsumed
			}
		}

	case *tcell.EventMouse:
		mx, my := tev.Position()
		btn := tev.Buttons()

		// Close button
		if btn&tcell.Button1 != 0 && c.closeHit.Contains(mx, my) {
			if c.OnClose != nil {
				c.OnClose()
			}
			return EventConsumed
		}

		// File reference clicks
		if btn&tcell.Button1 != 0 {
			for _, fh := range c.fileHits {
				if fh.HitRegion.Contains(mx, my) {
					if c.OnOpenFile != nil {
						c.OnOpenFile(fh.Path, fh.Line)
					}
					return EventConsumed
				}
			}
		}

		// Input click (compose area)
		if btn&tcell.Button1 != 0 {
			if c.Input.HandleClick(mx, my) {
				c.composing = true
				return EventConsumed
			}
		}

		// Scrollbar
		if newTop, consumed := c.scrollbar.HandleEvent(ev); consumed {
			c.scrollTop = newTop
			return EventConsumed
		}

		// Scroll wheel
		if btn&tcell.WheelUp != 0 {
			c.scrollTop -= 3
			if c.scrollTop < 0 {
				c.scrollTop = 0
			}
			return EventConsumed
		}
		if btn&tcell.WheelDown != 0 {
			c.scrollTop += 3
			return EventConsumed
		}

		// Consume clicks within panel bounds
		rect := c.GetRect()
		sw := rect.W
		panelW := c.panelWidth(sw)
		panelX := sw - panelW + rect.X
		if mx >= panelX {
			return EventConsumed
		}
	}
	return EventIgnored
}

// CursorPosition returns the cursor position when composing a comment.
func (c *CommentPanelWidget) CursorPosition() (x, y int, visible bool) {
	if !c.composing {
		return 0, 0, false
	}
	rect := c.GetRect()
	sw := rect.W
	panelW := c.panelWidth(sw)
	panelX := sw - panelW + rect.X
	contentX := panelX + 1

	composeInputY := rect.Y + rect.H - 2
	cx := c.Input.CursorX(contentX)
	return cx, composeInputY, true
}

// FocusedInput returns the InputWidget when composing.
func (c *CommentPanelWidget) FocusedInput() *InputWidget {
	if c.composing {
		return c.Input
	}
	return nil
}
