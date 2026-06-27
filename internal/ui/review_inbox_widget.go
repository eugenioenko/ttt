package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v2"
)

// ReviewInboxItem represents a single item in the review inbox list.
type ReviewInboxItem struct {
	Comment github.PRComment
	State   github.CommentState
}

// ReviewFileGroup groups inline comments by file path.
type ReviewFileGroup struct {
	Path      string
	Comments  []ReviewInboxItem
	Expanded  bool
	OpenCount int // number of open/addressed comments
}

// inboxItemKind identifies the type of a flattened inbox row.
type inboxItemKind int

const (
	inboxItemFileHeader inboxItemKind = iota
	inboxItemComment
	inboxItemGeneralHeader
	inboxItemGeneralComment
	inboxItemBorder
)

// inboxItem is a flattened row for rendering/selection.
type inboxItem struct {
	kind       inboxItemKind
	groupIndex int // index into FileGroups or -1
	itemIndex  int // index into group's Comments or GeneralComments
}

// ReviewInboxWidget is a sidebar panel for PR review comments.
type ReviewInboxWidget struct {
	BaseWidget
	SelectableList

	// Data
	FileGroups      []ReviewFileGroup
	GeneralComments []ReviewInboxItem
	items           []inboxItem
	Loading         bool

	// PR info
	PROwner  string
	PRRepo   string
	PRNumber int

	// State persistence
	State *github.ReviewState

	// Callbacks
	OnOpenFile      func(path string, line int) // navigate to file:line
	OnMarkVerified  func(commentID int)
	OnMarkDismissed func(commentID int)
	OnReopen        func(commentID int)
	OnAddReply      func(comment github.PRComment)
	OnAddComment    func()
	OnRefresh       func()
}

// NewReviewInboxWidget creates a new review inbox widget.
func NewReviewInboxWidget() *ReviewInboxWidget {
	return &ReviewInboxWidget{}
}

func (r *ReviewInboxWidget) Focusable() bool { return true }

// SetComments populates the inbox with PR comments and review state.
func (r *ReviewInboxWidget) SetComments(comments []github.PRComment, state *github.ReviewState) {
	r.State = state

	// Group inline comments by file path
	fileMap := make(map[string][]ReviewInboxItem)
	var fileOrder []string
	r.GeneralComments = nil

	for _, c := range comments {
		itemState := state.GetState(c.ID)
		item := ReviewInboxItem{Comment: c, State: itemState}

		if c.IsInline && c.Path != "" {
			if _, exists := fileMap[c.Path]; !exists {
				fileOrder = append(fileOrder, c.Path)
			}
			fileMap[c.Path] = append(fileMap[c.Path], item)
		} else {
			r.GeneralComments = append(r.GeneralComments, item)
		}
	}

	r.FileGroups = nil
	for _, path := range fileOrder {
		items := fileMap[path]
		openCount := 0
		for _, item := range items {
			if item.State == github.StateOpen || item.State == github.StateAddressed {
				openCount++
			}
		}
		r.FileGroups = append(r.FileGroups, ReviewFileGroup{
			Path:      path,
			Comments:  items,
			Expanded:  true,
			OpenCount: openCount,
		})
	}

	r.buildItems()
	r.ClampSelected(len(r.items))
}

// UpdateCommentState updates the state of a specific comment.
func (r *ReviewInboxWidget) UpdateCommentState(commentID int, state github.CommentState) {
	if r.State != nil {
		r.State.SetState(commentID, state)
	}
	for gi := range r.FileGroups {
		for ci := range r.FileGroups[gi].Comments {
			if r.FileGroups[gi].Comments[ci].Comment.ID == commentID {
				r.FileGroups[gi].Comments[ci].State = state
				// Recalculate open count
				openCount := 0
				for _, item := range r.FileGroups[gi].Comments {
					if item.State == github.StateOpen || item.State == github.StateAddressed {
						openCount++
					}
				}
				r.FileGroups[gi].OpenCount = openCount
				return
			}
		}
	}
	for ci := range r.GeneralComments {
		if r.GeneralComments[ci].Comment.ID == commentID {
			r.GeneralComments[ci].State = state
			return
		}
	}
}

// ProgressText returns a string like "4/7 addressed" for the status bar.
func (r *ReviewInboxWidget) ProgressText() string {
	total := 0
	resolved := 0
	for _, g := range r.FileGroups {
		for _, item := range g.Comments {
			total++
			if item.State == github.StateVerified || item.State == github.StateDismissed {
				resolved++
			}
		}
	}
	for _, item := range r.GeneralComments {
		total++
		if item.State == github.StateVerified || item.State == github.StateDismissed {
			resolved++
		}
	}
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d resolved", resolved, total)
}

// TotalComments returns the total number of comments.
func (r *ReviewInboxWidget) TotalComments() int {
	total := 0
	for _, g := range r.FileGroups {
		total += len(g.Comments)
	}
	total += len(r.GeneralComments)
	return total
}

// UnresolvedComments returns all unresolved inline comments sorted by file and line.
func (r *ReviewInboxWidget) UnresolvedComments() []ReviewInboxItem {
	var result []ReviewInboxItem
	for _, g := range r.FileGroups {
		for _, item := range g.Comments {
			if item.State == github.StateOpen || item.State == github.StateAddressed {
				result = append(result, item)
			}
		}
	}
	return result
}

// SelectedComment returns the currently selected comment, if any.
func (r *ReviewInboxWidget) SelectedComment() *ReviewInboxItem {
	if r.Selected < 0 || r.Selected >= len(r.items) {
		return nil
	}
	item := r.items[r.Selected]
	switch item.kind {
	case inboxItemComment:
		if item.groupIndex >= 0 && item.groupIndex < len(r.FileGroups) {
			g := &r.FileGroups[item.groupIndex]
			if item.itemIndex >= 0 && item.itemIndex < len(g.Comments) {
				return &g.Comments[item.itemIndex]
			}
		}
	case inboxItemGeneralComment:
		if item.itemIndex >= 0 && item.itemIndex < len(r.GeneralComments) {
			return &r.GeneralComments[item.itemIndex]
		}
	}
	return nil
}

// HasData returns true if there are any comments loaded.
func (r *ReviewInboxWidget) HasData() bool {
	return len(r.FileGroups) > 0 || len(r.GeneralComments) > 0
}

// CommentMarkersForFile returns comment markers for the given file path.
// Returns a map of line number -> CommentMarkerInfo.
func (r *ReviewInboxWidget) CommentMarkersForFile(filePath string) map[int]CommentMarkerInfo {
	markers := make(map[int]CommentMarkerInfo)
	for _, g := range r.FileGroups {
		if g.Path != filePath && !strings.HasSuffix(filePath, "/"+g.Path) {
			continue
		}
		for _, item := range g.Comments {
			if item.Comment.Line <= 0 {
				continue
			}
			line := item.Comment.Line - 1 // 0-based
			existing, ok := markers[line]
			if !ok || commentStatePriority(item.State) > commentStatePriority(existing.State) {
				markers[line] = CommentMarkerInfo{
					State:   item.State,
					Count:   1,
					Preview: truncate(item.Comment.Body, 40),
				}
			} else if ok {
				info := markers[line]
				info.Count++
				markers[line] = info
			}
		}
	}
	return markers
}

// CommentMarkerInfo holds info about comment markers for a specific line.
type CommentMarkerInfo struct {
	State   github.CommentState
	Count   int
	Preview string
}

func commentStatePriority(s github.CommentState) int {
	switch s {
	case github.StateOpen:
		return 3
	case github.StateAddressed:
		return 2
	case github.StateVerified:
		return 1
	case github.StateDismissed:
		return 0
	}
	return 0
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen-2]) + ".."
	}
	return s
}

func (r *ReviewInboxWidget) buildItems() {
	r.items = nil

	for gi, g := range r.FileGroups {
		r.items = append(r.items, inboxItem{kind: inboxItemFileHeader, groupIndex: gi})
		if g.Expanded {
			for ci := range g.Comments {
				r.items = append(r.items, inboxItem{kind: inboxItemComment, groupIndex: gi, itemIndex: ci})
			}
		}
	}

	if len(r.GeneralComments) > 0 {
		r.items = append(r.items, inboxItem{kind: inboxItemBorder})
		r.items = append(r.items, inboxItem{kind: inboxItemGeneralHeader})
		for ci := range r.GeneralComments {
			r.items = append(r.items, inboxItem{kind: inboxItemGeneralComment, itemIndex: ci})
		}
	}
}

// Render draws the review inbox widget.
func (r *ReviewInboxWidget) Render(surface *RenderSurface) {
	w, h := surface.Size()
	surface.Fill(term.Cell{Ch: ' '})

	if !r.HasData() {
		msg := "No review comments"
		if r.Loading {
			msg = "Loading comments..."
		}
		for i, ch := range msg {
			if i+1 < w {
				surface.SetCell(i+1, 0, term.Cell{Ch: ch, Style: term.StyleDefault})
			}
		}
		return
	}

	if h <= 0 {
		return
	}

	// Show progress in the first row
	progress := r.ProgressText()
	if progress != "" {
		x := w - len([]rune(progress)) - 1
		if x < 0 {
			x = 0
		}
		for i, ch := range progress {
			if x+i < w {
				surface.SetCell(x+i, 0, term.Cell{Ch: ch, Style: term.StyleMuted})
			}
		}
	}

	r.EnsureVisible(h)

	for i := 0; i < h; i++ {
		idx := r.ScrollTop + i
		if idx >= len(r.items) {
			break
		}
		item := r.items[idx]
		y := i

		style := term.StyleDefault
		if idx == r.Selected {
			style = term.StyleSidebarSelected
		}

		for x := 0; x < w; x++ {
			surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
		}

		switch item.kind {
		case inboxItemFileHeader:
			r.renderFileHeader(surface, y, w, style, item.groupIndex)
		case inboxItemComment:
			r.renderComment(surface, y, w, style, item.groupIndex, item.itemIndex)
		case inboxItemGeneralHeader:
			r.renderGeneralHeader(surface, y, w, style)
		case inboxItemGeneralComment:
			r.renderGeneralComment(surface, y, w, style, item.itemIndex)
		case inboxItemBorder:
			for x := 0; x < w; x++ {
				surface.SetCell(x, y, term.Cell{Ch: '─', Style: term.StyleBorder})
			}
		}
	}
}

func (r *ReviewInboxWidget) renderFileHeader(surface *RenderSurface, y, w int, style term.Style, gi int) {
	g := r.FileGroups[gi]
	x := 0

	// Chevron
	chevron := '▶'
	if g.Expanded {
		chevron = '▼'
	}
	if x < w {
		surface.SetCell(x, y, term.Cell{Ch: chevron, Style: style})
		x++
	}
	if x < w {
		surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
		x++
	}

	// File name (base name only)
	name := filepath.Base(g.Path)
	countStr := fmt.Sprintf(" %d/%d", g.OpenCount, len(g.Comments))
	maxNameW := w - x - len([]rune(countStr)) - 1
	for _, ch := range name {
		if x >= maxNameW+2 { // +2 for chevron+space
			break
		}
		if x < w {
			surface.SetCell(x, y, term.Cell{Ch: ch, Style: style})
			x++
		}
	}

	// Open count at the right
	cx := w - len([]rune(countStr))
	if cx < x {
		cx = x
	}
	countStyle := term.StyleMuted
	if g.OpenCount > 0 {
		countStyle = term.StyleWarning
	}
	for _, ch := range countStr {
		if cx < w {
			surface.SetCell(cx, y, term.Cell{Ch: ch, Style: countStyle})
			cx++
		}
	}
}

func (r *ReviewInboxWidget) renderComment(surface *RenderSurface, y, w int, style term.Style, gi, ci int) {
	g := r.FileGroups[gi]
	item := g.Comments[ci]
	x := 2 // indent

	// State indicator
	indicator, indicatorStyle := stateIndicator(item.State)
	if x < w {
		surface.SetCell(x, y, term.Cell{Ch: indicator, Style: indicatorStyle})
		x++
	}
	if x < w {
		surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
		x++
	}

	// Line number
	lineStr := fmt.Sprintf("L:%d", item.Comment.Line)
	for _, ch := range lineStr {
		if x >= w-1 {
			break
		}
		surface.SetCell(x, y, term.Cell{Ch: ch, Style: term.StyleMuted})
		x++
	}
	if x < w {
		surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
		x++
	}

	// @user
	userStr := "@" + item.Comment.User + ":"
	for _, ch := range userStr {
		if x >= w-1 {
			break
		}
		surface.SetCell(x, y, term.Cell{Ch: ch, Style: term.StyleMuted})
		x++
	}
	if x < w {
		surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
		x++
	}

	// Body preview
	body := truncate(item.Comment.Body, w-x)
	for _, ch := range body {
		if x >= w {
			break
		}
		surface.SetCell(x, y, term.Cell{Ch: ch, Style: style})
		x++
	}
}

func (r *ReviewInboxWidget) renderGeneralHeader(surface *RenderSurface, y, w int, style term.Style) {
	label := "── General "
	x := 0
	for _, ch := range label {
		if x >= w {
			break
		}
		surface.SetCell(x, y, term.Cell{Ch: ch, Style: term.StyleMuted})
		x++
	}
	for x < w {
		surface.SetCell(x, y, term.Cell{Ch: '─', Style: term.StyleMuted})
		x++
	}
}

func (r *ReviewInboxWidget) renderGeneralComment(surface *RenderSurface, y, w int, style term.Style, ci int) {
	item := r.GeneralComments[ci]
	x := 2 // indent

	// State indicator
	indicator, indicatorStyle := stateIndicator(item.State)
	if x < w {
		surface.SetCell(x, y, term.Cell{Ch: indicator, Style: indicatorStyle})
		x++
	}
	if x < w {
		surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
		x++
	}

	// @user
	userStr := "@" + item.Comment.User + ":"
	for _, ch := range userStr {
		if x >= w-1 {
			break
		}
		surface.SetCell(x, y, term.Cell{Ch: ch, Style: term.StyleMuted})
		x++
	}
	if x < w {
		surface.SetCell(x, y, term.Cell{Ch: ' ', Style: style})
		x++
	}

	// Body preview
	body := truncate(item.Comment.Body, w-x)
	for _, ch := range body {
		if x >= w {
			break
		}
		surface.SetCell(x, y, term.Cell{Ch: ch, Style: style})
		x++
	}
}

func stateIndicator(state github.CommentState) (rune, term.Style) {
	switch state {
	case github.StateOpen:
		return '●', term.StyleDanger // ● (filled circle) - red
	case github.StateAddressed:
		return '~', term.StyleWarning // ~ - yellow
	case github.StateVerified:
		return '✓', term.StyleSuccess // ✓ - green
	case github.StateDismissed:
		return '✗', term.StyleMuted // ✗ - dimmed
	}
	return '●', term.StyleDanger
}

// HandleEvent handles keyboard and mouse events for the review inbox.
func (r *ReviewInboxWidget) HandleEvent(ev tcell.Event) EventResult {
	if !r.HasData() {
		return EventIgnored
	}

	// Handle keyboard shortcuts first
	if kev, ok := ev.(*tcell.EventKey); ok {
		switch kev.Key() {
		case tcell.KeyRune:
			switch kev.Rune() {
			case 'v': // mark verified
				if sel := r.SelectedComment(); sel != nil {
					if r.OnMarkVerified != nil {
						r.OnMarkVerified(sel.Comment.ID)
					}
					return EventConsumed
				}
			case 'd': // dismiss
				if sel := r.SelectedComment(); sel != nil {
					if r.OnMarkDismissed != nil {
						r.OnMarkDismissed(sel.Comment.ID)
					}
					return EventConsumed
				}
			case 'r': // reopen or refresh
				if sel := r.SelectedComment(); sel != nil {
					if sel.State == github.StateVerified || sel.State == github.StateDismissed {
						if r.OnReopen != nil {
							r.OnReopen(sel.Comment.ID)
						}
						return EventConsumed
					}
				}
				// If no comment selected or it's not closed, refresh
				if r.OnRefresh != nil {
					r.OnRefresh()
					return EventConsumed
				}
			case 'a': // add reply
				if sel := r.SelectedComment(); sel != nil {
					if r.OnAddReply != nil {
						r.OnAddReply(sel.Comment)
					}
					return EventConsumed
				}
			}
		}
	}

	rect := r.GetRect()
	lr := r.SelectableList.HandleListEvent(ev, rect, len(r.items))
	if lr.Action == ListActionActivate {
		r.handleActivate()
		return EventConsumed
	}
	return lr.Result
}

func (r *ReviewInboxWidget) handleActivate() {
	if r.Selected < 0 || r.Selected >= len(r.items) {
		return
	}
	item := r.items[r.Selected]

	switch item.kind {
	case inboxItemFileHeader:
		// Toggle expansion
		if item.groupIndex >= 0 && item.groupIndex < len(r.FileGroups) {
			r.FileGroups[item.groupIndex].Expanded = !r.FileGroups[item.groupIndex].Expanded
			r.buildItems()
			r.ClampSelected(len(r.items))
		}
	case inboxItemComment:
		// Navigate to file:line
		if item.groupIndex >= 0 && item.groupIndex < len(r.FileGroups) {
			g := r.FileGroups[item.groupIndex]
			if item.itemIndex >= 0 && item.itemIndex < len(g.Comments) {
				c := g.Comments[item.itemIndex]
				if r.OnOpenFile != nil {
					r.OnOpenFile(c.Comment.Path, c.Comment.Line)
				}
			}
		}
	case inboxItemGeneralComment:
		// Could show full comment in future
	}
}
