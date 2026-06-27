package ui

import (
	"fmt"
	"strings"

	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/gdamore/tcell/v2"
)

// reviewItemKind distinguishes different row types in the flattened list.
type reviewItemKind int

const (
	reviewItemComment reviewItemKind = iota
	reviewItemHeader
	reviewItemSpacer
)

// reviewItem is a single row in the flat list rendered by ReviewsWidget.
type reviewItem struct {
	kind         reviewItemKind
	commentIndex int    // index into ReviewsWidget.Comments
	headerLabel  string // for header rows
}

// ReviewsWidget displays PR review comments in the sidebar.
type ReviewsWidget struct {
	BaseWidget
	SelectableList

	Comments []github.PRComment
	items    []reviewItem

	// PR coordinates, needed for adding comments.
	Owner  string
	Repo   string
	Number int

	Loading bool

	// Input for adding a general comment.
	Input        *InputWidget
	inputFocused bool

	// Callbacks set by the app layer.
	OnOpenFile         func(path string, line int)
	OnAddComment       func(body string)
	OnAddInlineComment func(body, path string, line int)

	scrollbar Scrollbar
}

// NewReviewsWidget returns a ready-to-use ReviewsWidget.
func NewReviewsWidget() *ReviewsWidget {
	inp := NewInputWidget()
	inp.Placeholder = "Add a comment..."
	return &ReviewsWidget{Input: inp}
}

func (r *ReviewsWidget) Focusable() bool { return true }

// SetComments replaces the current comments and rebuilds the flat list.
func (r *ReviewsWidget) SetComments(comments []github.PRComment) {
	r.Comments = comments
	r.buildItems()
	r.ClampSelected(len(r.items))
}

// SetPR stores the PR coordinates used for posting new comments.
func (r *ReviewsWidget) SetPR(owner, repo string, number int) {
	r.Owner = owner
	r.Repo = repo
	r.Number = number
}

// buildItems flattens comments into a renderable list.
// Inline comments are grouped by file path; general comments appear under
// a separate "General" header.
func (r *ReviewsWidget) buildItems() {
	r.items = nil

	// Count general comments to decide whether to add a "General" header.
	hasGeneral := false
	for _, c := range r.Comments {
		if !c.IsInline {
			hasGeneral = true
			break
		}
	}

	// Group inline comments by file path, preserving order of first
	// appearance.
	type fileGroup struct {
		path     string
		comments []int // indices into r.Comments
	}
	var groups []fileGroup
	groupIdx := map[string]int{}
	for i, c := range r.Comments {
		if !c.IsInline {
			continue
		}
		gi, ok := groupIdx[c.Path]
		if !ok {
			gi = len(groups)
			groupIdx[c.Path] = gi
			groups = append(groups, fileGroup{path: c.Path})
		}
		groups[gi].comments = append(groups[gi].comments, i)
	}

	// Render inline groups.
	for _, fg := range groups {
		r.items = append(r.items, reviewItem{kind: reviewItemHeader, headerLabel: fg.path})
		for _, ci := range fg.comments {
			r.items = append(r.items, reviewItem{kind: reviewItemComment, commentIndex: ci})
		}
		r.items = append(r.items, reviewItem{kind: reviewItemSpacer})
	}

	// Render general comments.
	if hasGeneral {
		r.items = append(r.items, reviewItem{kind: reviewItemHeader, headerLabel: "General"})
		for i := range r.Comments {
			if !r.Comments[i].IsInline {
				r.items = append(r.items, reviewItem{kind: reviewItemComment, commentIndex: i})
			}
		}
		r.items = append(r.items, reviewItem{kind: reviewItemSpacer})
	}
}

// formatTimestamp returns a short display string for an ISO 8601 timestamp.
func formatTimestamp(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	return ts
}

// truncate returns s shortened to at most maxLen runes, adding "..." if
// truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

func (r *ReviewsWidget) Render(surface *RenderSurface) {
	w, h := surface.Size()
	surface.Fill(term.Cell{Ch: ' '})

	if len(r.Comments) == 0 {
		msg := "No comments"
		if r.Loading {
			msg = "Loading..."
		}
		for i, ch := range msg {
			if i+1 < w {
				surface.SetCell(i+1, 0, term.Cell{Ch: ch, Style: term.StyleMuted})
			}
		}
		return
	}

	if h <= 0 {
		return
	}

	// Reserve 2 rows at the bottom for the input area.
	listH := h - 2
	if listH < 1 {
		listH = h
	}

	r.EnsureVisible(listH)

	rect := r.GetRect()
	r.scrollbar.X = rect.X + w - 1
	r.scrollbar.Y = rect.Y
	r.scrollbar.Height = listH
	r.scrollbar.TotalItems = len(r.items)
	r.scrollbar.TopItem = r.ScrollTop

	contentW := w
	if r.scrollbar.Visible() {
		contentW = w - 1
	}

	for y := 0; y < listH; y++ {
		idx := r.ScrollTop + y
		if idx >= len(r.items) {
			break
		}
		item := r.items[idx]

		style := term.StyleDefault
		if idx == r.Selected && !r.inputFocused {
			style = term.StyleSidebarSelected
		}

		// Clear the row.
		for x := 0; x < contentW; x++ {
			surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
		}

		switch item.kind {
		case reviewItemHeader:
			r.renderHeader(surface, y, contentW, style, item.headerLabel)
		case reviewItemComment:
			r.renderComment(surface, y, contentW, style, idx == r.Selected && !r.inputFocused, item.commentIndex)
		case reviewItemSpacer:
			// already cleared
		}
	}

	r.scrollbar.Render(surface, w-1, 0)

	// Draw input area at the bottom.
	if listH < h {
		// Separator line.
		for x := 0; x < w; x++ {
			surface.SetCell(x, listH, term.Cell{Ch: '─', Style: term.StyleBorder})
		}
		r.Input.Render(surface, 0, listH+1, w)
	}
}

func (r *ReviewsWidget) renderHeader(surface *RenderSurface, y, w int, style term.Style, label string) {
	x := 0
	headerStyle := term.StyleMuted
	if style == term.StyleSidebarSelected {
		headerStyle = style
	}

	// File path or "General" label
	for _, ch := range label {
		if x >= w {
			break
		}
		surface.SetCell(x, y, term.Cell{Ch: ch, Style: headerStyle})
		x++
	}
}

func (r *ReviewsWidget) renderComment(surface *RenderSurface, y, w int, style term.Style, selected bool, ci int) {
	if ci < 0 || ci >= len(r.Comments) {
		return
	}
	c := r.Comments[ci]

	x := 1

	// Author
	author := "@" + c.User
	authorStyle := style
	for _, ch := range author {
		if x >= w-12 {
			break
		}
		surface.SetCell(x, y, term.Cell{Ch: ch, Style: authorStyle})
		x++
	}
	x++

	// For inline comments show line number.
	if c.IsInline && c.Line > 0 {
		loc := fmt.Sprintf("L%d", c.Line)
		locStyle := term.StyleMuted
		if selected {
			locStyle = style
		}
		for _, ch := range loc {
			if x >= w-12 {
				break
			}
			surface.SetCell(x, y, term.Cell{Ch: ch, Style: locStyle})
			x++
		}
		x++
	}

	// Timestamp (right-aligned)
	ts := formatTimestamp(c.CreatedAt)
	tsRunes := []rune(ts)
	if len(tsRunes) > 0 {
		tsX := w - len(tsRunes) - 1
		if tsX > x {
			tsStyle := term.StyleMuted
			if selected {
				tsStyle = style
			}
			for i, ch := range tsRunes {
				surface.SetCell(tsX+i, y, term.Cell{Ch: ch, Style: tsStyle})
			}
		}
	}

	// Body (first line, truncated to fill space between metadata and timestamp)
	body := strings.ReplaceAll(c.Body, "\n", " ")
	body = strings.ReplaceAll(body, "\r", "")
	maxBodyW := w - x - len(tsRunes) - 2
	if maxBodyW < 4 {
		// Not enough room; skip the body preview altogether since the
		// timestamp already fills the row.
		return
	}
	body = truncate(body, maxBodyW)
	bodyStyle := term.StyleMuted
	if selected {
		bodyStyle = style
	}
	for _, ch := range body {
		if x >= w-len(tsRunes)-2 {
			break
		}
		surface.SetCell(x, y, term.Cell{Ch: ch, Style: bodyStyle})
		x++
	}
}

// CursorPosition implements CursorProvider so the input caret blinks.
func (r *ReviewsWidget) CursorPosition() (int, int, bool) {
	if !r.inputFocused {
		return 0, 0, false
	}
	rect := r.GetRect()
	h := rect.H
	inputY := rect.Y + h - 1
	return r.Input.CursorX(rect.X), inputY, true
}

// FocusedInput implements InputHolder so copy/paste routes to the input.
func (r *ReviewsWidget) FocusedInput() *InputWidget {
	if r.inputFocused {
		return r.Input
	}
	return nil
}

func (r *ReviewsWidget) HandleEvent(ev tcell.Event) EventResult {
	// When the input is focused, route keys there.
	if r.inputFocused {
		if tev, ok := ev.(*tcell.EventKey); ok {
			switch tev.Key() {
			case tcell.KeyEscape:
				r.inputFocused = false
				return EventConsumed
			case tcell.KeyEnter:
				body := strings.TrimSpace(r.Input.Text)
				if body != "" && r.OnAddComment != nil {
					r.OnAddComment(body)
					r.Input.Clear()
				}
				r.inputFocused = false
				return EventConsumed
			default:
				r.Input.HandleEvent(ev)
				return EventConsumed
			}
		}
	}

	// Scrollbar
	if newTop, consumed := r.scrollbar.HandleEvent(ev); consumed {
		r.ScrollTop = newTop
		if r.scrollbar.IsDragging() {
			return EventCaptured
		}
		return EventConsumed
	}

	rect := r.GetRect()
	h := rect.H
	listH := h - 2
	if listH < 1 {
		listH = h
	}

	// Mouse click on the input row.
	if tev, ok := ev.(*tcell.EventMouse); ok {
		if tev.Buttons()&tcell.Button1 != 0 {
			_, my := tev.Position()
			row := my - rect.Y
			if row >= listH {
				r.inputFocused = true
				r.Input.HandleClick(tev.Position())
				return EventConsumed
			}
		}
	}

	// SelectableList handles up/down/enter/wheel/click for the list area.
	listRect := Rect{X: rect.X, Y: rect.Y, W: rect.W, H: listH}
	res := r.SelectableList.HandleListEvent(ev, listRect, len(r.items))
	if res.Result == EventConsumed {
		if res.Action == ListActionActivate {
			r.activateSelected()
		}
		return EventConsumed
	}

	// 'i' to focus the input.
	if tev, ok := ev.(*tcell.EventKey); ok {
		if tev.Key() == tcell.KeyRune && tev.Rune() == 'i' {
			r.inputFocused = true
			return EventConsumed
		}
	}

	return EventIgnored
}

func (r *ReviewsWidget) activateSelected() {
	if r.Selected < 0 || r.Selected >= len(r.items) {
		return
	}
	item := r.items[r.Selected]
	if item.kind != reviewItemComment {
		return
	}
	ci := item.commentIndex
	if ci < 0 || ci >= len(r.Comments) {
		return
	}
	c := r.Comments[ci]
	if c.IsInline && r.OnOpenFile != nil {
		r.OnOpenFile(c.Path, c.Line)
	}
}
