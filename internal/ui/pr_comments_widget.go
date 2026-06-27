package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v2"
)

// commentItemKind distinguishes the different row types in the rendered list.
type commentItemKind int

const (
	commentItemSection  commentItemKind = iota // section header ("General" / file path)
	commentItemComment                         // a comment body row
	commentItemInputRow                        // the input row at the bottom
)

// commentItem is one logical row in the flattened list used for rendering and
// scrolling. For commentItemComment rows, commentIndex points into the
// widget's Comments slice.
type commentItem struct {
	kind         commentItemKind
	commentIndex int    // valid for commentItemComment
	label        string // section header text
}

// PRCommentsWidget shows PR review comments in a scrollable list with an
// input area at the bottom for adding new general comments.
type PRCommentsWidget struct {
	BaseWidget
	Comments []github.PRComment

	items        []commentItem
	selected     int
	scrollTop    int
	scrollbar    Scrollbar
	inputFocused bool
	Input        *InputWidget

	// PR coordinates needed by the caller for refresh / submit.
	Owner  string
	Repo   string
	Number int

	OnOpenFile      func(path string, line int)
	OnSubmitComment func(body string)
}

func NewPRCommentsWidget() *PRCommentsWidget {
	inp := NewInputWidget()
	inp.Placeholder = "Add a comment..."
	return &PRCommentsWidget{
		Input: inp,
	}
}

func (w *PRCommentsWidget) Focusable() bool { return true }

// SetComments replaces the comment list and rebuilds the internal item list.
func (w *PRCommentsWidget) SetComments(comments []github.PRComment) {
	w.Comments = comments
	w.buildItems()
	if w.selected >= len(w.items) {
		w.selected = len(w.items) - 1
	}
	if w.selected < 0 {
		w.selected = 0
	}
}

// buildItems creates a flat list of rows for rendering. The layout is:
//   - "General Comments" section header (if any general comments)
//   - general comments
//   - One section header per file (if any inline comments)
//   - inline comments grouped under that file
//   - Input row at the very bottom
func (w *PRCommentsWidget) buildItems() {
	w.items = nil

	var general []int
	byFile := make(map[string][]int)
	var fileOrder []string
	filesSeen := make(map[string]bool)

	for i, c := range w.Comments {
		if !c.IsInline {
			general = append(general, i)
		} else {
			if !filesSeen[c.Path] {
				filesSeen[c.Path] = true
				fileOrder = append(fileOrder, c.Path)
			}
			byFile[c.Path] = append(byFile[c.Path], i)
		}
	}

	if len(general) > 0 {
		w.items = append(w.items, commentItem{kind: commentItemSection, label: "General Comments"})
		for _, idx := range general {
			w.items = append(w.items, commentItem{kind: commentItemComment, commentIndex: idx})
		}
	}

	for _, file := range fileOrder {
		w.items = append(w.items, commentItem{kind: commentItemSection, label: file})
		for _, idx := range byFile[file] {
			w.items = append(w.items, commentItem{kind: commentItemComment, commentIndex: idx})
		}
	}

	// Input row always at the end
	w.items = append(w.items, commentItem{kind: commentItemInputRow})
}

func (w *PRCommentsWidget) Render(surface *RenderSurface) {
	sw, sh := surface.Size()
	if sw == 0 || sh == 0 {
		return
	}

	totalRows := len(w.items)
	if totalRows == 0 {
		msg := "No PR comments"
		x := 1
		for _, ch := range msg {
			if x >= sw {
				break
			}
			surface.SetCell(x, 0, term.Cell{Ch: ch, Style: term.StyleMuted})
			x++
		}
		return
	}

	// Scroll management
	if w.scrollTop > w.selected {
		w.scrollTop = w.selected
	}
	if w.selected >= w.scrollTop+sh {
		w.scrollTop = w.selected - sh + 1
	}

	r := w.GetRect()
	w.scrollbar.X = r.X + sw - 1
	w.scrollbar.Y = r.Y
	w.scrollbar.Height = sh
	w.scrollbar.TotalItems = totalRows
	w.scrollbar.TopItem = w.scrollTop

	contentW := sw
	if w.scrollbar.Visible() {
		contentW = sw - 1
	}

	for y := 0; y < sh; y++ {
		idx := w.scrollTop + y
		if idx >= totalRows {
			break
		}
		item := w.items[idx]

		isSelected := idx == w.selected && !w.inputFocused

		switch item.kind {
		case commentItemSection:
			w.renderSection(surface, y, contentW, isSelected, item.label)
		case commentItemComment:
			c := w.Comments[item.commentIndex]
			w.renderComment(surface, y, contentW, isSelected, c)
		case commentItemInputRow:
			w.Input.Render(surface, 0, y, contentW)
		}
	}

	w.scrollbar.Render(surface, sw-1, 0)
}

func (w *PRCommentsWidget) renderSection(surface *RenderSurface, y, width int, selected bool, label string) {
	style := term.StyleMuted
	if selected {
		style = term.StyleSidebarSelected
	}
	for x := 0; x < width; x++ {
		surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
	}
	x := 1
	for _, ch := range label {
		if x >= width {
			break
		}
		surface.SetCell(x, y, term.Cell{Ch: ch, Style: style})
		x++
	}
}

func (w *PRCommentsWidget) renderComment(surface *RenderSurface, y, width int, selected bool, c github.PRComment) {
	style := term.StyleDefault
	if selected {
		style = term.StyleSidebarSelected
	}

	// Clear row
	for x := 0; x < width; x++ {
		surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
	}

	x := 1

	// Author
	authorStyle := term.StyleSuccess
	if selected {
		authorStyle = term.StyleSidebarSelected
	}
	for _, ch := range c.User {
		if x >= width-1 {
			break
		}
		surface.SetCell(x, y, term.Cell{Ch: ch, Style: authorStyle})
		x++
	}
	// separator
	surface.SetCell(x, y, term.Cell{Ch: ':', Style: style})
	x++
	surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
	x++

	// Body (first line only, truncated)
	bodyLine := firstLine(c.Body)
	bodyStyle := style
	if !selected {
		bodyStyle = term.StyleMuted
	}

	// Reserve space for file:line on the right if inline
	rightLabel := ""
	if c.IsInline && c.Path != "" {
		if c.Line > 0 {
			rightLabel = fmt.Sprintf(" %s:%d", filepath.Base(c.Path), c.Line)
		} else {
			rightLabel = " " + filepath.Base(c.Path)
		}
	}
	rightW := len([]rune(rightLabel))
	bodyMaxW := width - x - rightW

	for _, ch := range bodyLine {
		if bodyMaxW <= 0 {
			break
		}
		surface.SetCell(x, y, term.Cell{Ch: ch, Style: bodyStyle})
		x++
		bodyMaxW--
	}

	// Render right-aligned file:line
	if rightLabel != "" {
		rx := width - rightW
		if rx < x {
			rx = x
		}
		locStyle := term.StyleMuted
		if selected {
			locStyle = term.StyleSidebarSelected
		}
		for _, ch := range rightLabel {
			if rx >= width {
				break
			}
			surface.SetCell(rx, y, term.Cell{Ch: ch, Style: locStyle})
			rx++
		}
	}
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}

func (w *PRCommentsWidget) HandleEvent(ev tcell.Event) EventResult {
	// When input is focused, route keys there
	if w.inputFocused {
		if tev, ok := ev.(*tcell.EventKey); ok {
			switch tev.Key() {
			case tcell.KeyEscape:
				w.inputFocused = false
				return EventConsumed
			case tcell.KeyEnter:
				text := strings.TrimSpace(w.Input.Text)
				if text != "" && w.OnSubmitComment != nil {
					w.OnSubmitComment(text)
					w.Input.Clear()
				}
				return EventConsumed
			case tcell.KeyUp:
				w.inputFocused = false
				if w.selected > 0 {
					w.selected--
				}
				return EventConsumed
			default:
				w.Input.HandleEvent(ev)
				return EventConsumed
			}
		}
	}

	// Scrollbar takes priority
	if newTop, consumed := w.scrollbar.HandleEvent(ev); consumed {
		w.scrollTop = newTop
		if w.scrollbar.IsDragging() {
			return EventCaptured
		}
		return EventConsumed
	}

	switch tev := ev.(type) {
	case *tcell.EventKey:
		switch tev.Key() {
		case tcell.KeyUp:
			if w.selected > 0 {
				w.selected--
			}
			return EventConsumed
		case tcell.KeyDown:
			if w.selected < len(w.items)-1 {
				w.selected++
			}
			return EventConsumed
		case tcell.KeyEnter:
			w.activateSelected()
			return EventConsumed
		}
	case *tcell.EventMouse:
		btn := tev.Buttons()
		_, my := tev.Position()
		r := w.GetRect()
		row := my - r.Y
		idx := w.scrollTop + row

		if btn&tcell.Button1 != 0 && idx >= 0 && idx < len(w.items) {
			w.selected = idx
			w.activateSelected()
			return EventConsumed
		}
		if btn&tcell.WheelUp != 0 {
			if w.scrollTop > 0 {
				w.scrollTop--
			}
			return EventConsumed
		}
		if btn&tcell.WheelDown != 0 {
			if w.scrollTop < len(w.items)-1 {
				w.scrollTop++
			}
			return EventConsumed
		}
	}
	return EventIgnored
}

func (w *PRCommentsWidget) activateSelected() {
	if w.selected < 0 || w.selected >= len(w.items) {
		return
	}
	item := w.items[w.selected]
	switch item.kind {
	case commentItemComment:
		c := w.Comments[item.commentIndex]
		if c.IsInline && c.Path != "" && w.OnOpenFile != nil {
			w.OnOpenFile(c.Path, c.Line)
		}
	case commentItemInputRow:
		w.inputFocused = true
	}
}

// CursorPosition implements CursorProvider so the blinking cursor appears
// in the input when it is focused.
func (w *PRCommentsWidget) CursorPosition() (int, int, bool) {
	if !w.inputFocused {
		return 0, 0, false
	}
	r := w.GetRect()
	for i, item := range w.items {
		if item.kind == commentItemInputRow {
			y := r.Y + i - w.scrollTop
			return w.Input.CursorX(r.X), y, true
		}
	}
	return 0, 0, false
}

// FocusedInput implements InputHolder for clipboard routing.
func (w *PRCommentsWidget) FocusedInput() *InputWidget {
	if !w.inputFocused {
		return nil
	}
	return w.Input
}
