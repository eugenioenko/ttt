package ui

import (
	"fmt"
	"strings"

	"github.com/eugenioenko/ttt/internal/term"

	"github.com/gdamore/tcell/v2"
)

// ReviewComment mirrors github.PRComment but lives in the ui package to
// avoid a circular import.
type ReviewComment struct {
	ID        int
	Body      string
	User      string
	CreatedAt string
	Path      string
	Line      int
	IsInline  bool
}

// reviewRow represents a single rendered row in the review hub. Each row
// is either a section header (file group), a comment body line, or a
// separator. Tracking the type lets us map clicks/navigation back to the
// originating comment.
type reviewRow struct {
	kind       rowKind
	text       string
	style      term.Style
	boldPrefix int // number of leading runes to render bold
	commentIdx int // index into Comments slice (-1 for non-comment rows)
	indent     int // left indent in cells
}

type rowKind int

const (
	rowHeader    rowKind = iota // file path header
	rowSeparator                // blank/divider
	rowUser                     // "@ user  2024-01-15"
	rowBody                     // comment body text
	rowGeneral                  // "General Comments" header
)

// ReviewHubWidget shows all PR review comments in a full-screen modal,
// organized by file. Users can scroll through comments, press Enter to
// navigate to a comment's location in the editor, or press 'a' to add
// a new comment.
type ReviewHubWidget struct {
	BaseWidget

	Comments  []ReviewComment
	PRTitle   string
	PRNumber  int
	Borders   *term.BorderSet
	OnDismiss func()

	// OnNavigate is called when the user activates a comment to jump
	// to its file/line in the editor.
	OnNavigate func(path string, line int)

	// OnAddComment is called when the user wants to add a general comment.
	OnAddComment func()

	// OnAddInlineComment is called with the selected comment's file+line
	// so the app can prompt for an inline reply.
	OnAddInlineComment func(path string, line int)

	rows      []reviewRow
	list      SelectableList
	rowsDirty bool

	// layout values (single source of truth)
	boxX, boxY, boxW, boxH int
	contentY               int // Y of first content row
	visibleRows            int // number of visible content rows
}

// NewReviewHubWidget creates a new review hub with the given comments.
func NewReviewHubWidget(comments []ReviewComment, prTitle string, prNumber int) *ReviewHubWidget {
	w := &ReviewHubWidget{
		Comments:  comments,
		PRTitle:   prTitle,
		PRNumber:  prNumber,
		rowsDirty: true,
	}
	return w
}

func (w *ReviewHubWidget) Focusable() bool { return true }

// buildRows converts the flat comment list into a list of displayable rows
// grouped by file path.
func (w *ReviewHubWidget) buildRows() {
	w.rows = nil

	// Separate inline vs general comments
	var inline []ReviewComment
	var general []ReviewComment
	for _, c := range w.Comments {
		if c.IsInline {
			inline = append(inline, c)
		} else {
			general = append(general, c)
		}
	}

	// Group inline comments by file
	type fileGroup struct {
		path     string
		comments []ReviewComment
	}
	seen := map[string]int{}
	var groups []fileGroup
	for _, c := range inline {
		if idx, ok := seen[c.Path]; ok {
			groups[idx].comments = append(groups[idx].comments, c)
		} else {
			seen[c.Path] = len(groups)
			groups = append(groups, fileGroup{path: c.Path, comments: []ReviewComment{c}})
		}
	}

	commentIdx := 0

	// Render inline comments grouped by file
	for gi, group := range groups {
		if gi > 0 {
			w.rows = append(w.rows, reviewRow{kind: rowSeparator, commentIdx: -1})
		}

		// File header
		w.rows = append(w.rows, reviewRow{
			kind:       rowHeader,
			text:       " " + group.path,
			style:      term.StyleCommentFile,
			commentIdx: -1,
		})

		for _, c := range group.comments {
			ci := commentIdx
			commentIdx++

			// User + date line
			dateStr := formatCommentDate(c.CreatedAt)
			userLine := fmt.Sprintf("  @%s  L%d  %s", c.User, c.Line, dateStr)
			w.rows = append(w.rows, reviewRow{
				kind:       rowUser,
				text:       userLine,
				style:      term.StyleCommentUser,
				boldPrefix: len([]rune("  @" + c.User)),
				commentIdx: ci,
				indent:     2,
			})

			// Body lines
			for _, line := range strings.Split(c.Body, "\n") {
				w.rows = append(w.rows, reviewRow{
					kind:       rowBody,
					text:       "    " + line,
					style:      term.StyleCommentBody,
					commentIdx: ci,
					indent:     4,
				})
			}
		}
	}

	// General comments section
	if len(general) > 0 {
		if len(w.rows) > 0 {
			w.rows = append(w.rows, reviewRow{kind: rowSeparator, commentIdx: -1})
		}
		w.rows = append(w.rows, reviewRow{
			kind:       rowGeneral,
			text:       " General Comments",
			style:      term.StyleCommentFile,
			commentIdx: -1,
		})

		for _, c := range general {
			ci := commentIdx
			commentIdx++

			dateStr := formatCommentDate(c.CreatedAt)
			userLine := fmt.Sprintf("  @%s  %s", c.User, dateStr)
			w.rows = append(w.rows, reviewRow{
				kind:       rowUser,
				text:       userLine,
				style:      term.StyleCommentUser,
				boldPrefix: len([]rune("  @" + c.User)),
				commentIdx: ci,
				indent:     2,
			})

			for _, line := range strings.Split(c.Body, "\n") {
				w.rows = append(w.rows, reviewRow{
					kind:       rowBody,
					text:       "    " + line,
					style:      term.StyleCommentBody,
					commentIdx: ci,
					indent:     4,
				})
			}
		}
	}

	if len(w.rows) == 0 {
		w.rows = append(w.rows, reviewRow{
			kind:       rowBody,
			text:       "  No comments on this PR",
			style:      term.StyleMuted,
			commentIdx: -1,
		})
	}

	w.rowsDirty = false
}

func (w *ReviewHubWidget) Render(surface *RenderSurface) {
	if w.rowsDirty {
		w.buildRows()
	}

	sw, sh := surface.Size()

	// Full-screen-ish box with some margin
	marginX := 4
	marginY := 1
	if sw < 60 {
		marginX = 1
	}
	if sh < 20 {
		marginY = 0
	}
	w.boxX = marginX
	w.boxY = marginY
	w.boxW = sw - 2*marginX
	w.boxH = sh - 2*marginY
	if w.boxW < 20 {
		w.boxW = 20
	}
	if w.boxH < 8 {
		w.boxH = 8
	}

	b := term.DoubleBorderSet()
	if w.Borders != nil {
		b = *w.Borders
	}

	// Clear and draw border
	surface.ClearRect(w.boxX, w.boxY, w.boxW, w.boxH, term.StylePaletteItem)
	surface.DrawBorder(w.boxX, w.boxY, w.boxW, w.boxH, b, term.StyleBorder)

	innerW := w.boxW - 2

	// Title bar: "PR #123: Title"
	titleText := fmt.Sprintf(" PR #%d: %s ", w.PRNumber, w.PRTitle)
	titleRunes := []rune(titleText)
	if len(titleRunes) > innerW {
		titleRunes = append(titleRunes[:innerW-1], '~')
	}
	titleX := w.boxX + (w.boxW-len(titleRunes))/2
	for i, ch := range titleRunes {
		surface.SetCell(titleX+i, w.boxY, term.Cell{Ch: ch, Style: term.StyleBorder})
	}

	// Subtitle: comment count + help
	totalComments := len(w.Comments)
	subtitle := fmt.Sprintf(" %d comments ", totalComments)
	surface.DrawText(w.boxX+2, w.boxY+1, subtitle, w.boxX+w.boxW-2, term.StyleMuted)

	// Help line at top-right
	helpText := "[a]dd [Enter]go [Esc]close"
	helpX := w.boxX + w.boxW - 2 - len([]rune(helpText))
	if helpX > w.boxX+2+len([]rune(subtitle)) {
		surface.DrawText(helpX, w.boxY+1, helpText, w.boxX+w.boxW-2, term.StyleMuted)
	}

	// Separator under subtitle
	sepY := w.boxY + 2
	for x := w.boxX + 1; x < w.boxX+w.boxW-1; x++ {
		surface.SetCell(x, sepY, term.Cell{Ch: b.Horizontal, Style: term.StyleBorder})
	}

	// Content area
	w.contentY = w.boxY + 3
	w.visibleRows = w.boxH - 4 // border + title + sep + bottom border
	if w.visibleRows < 1 {
		w.visibleRows = 1
	}

	w.list.ClampSelected(len(w.rows))
	w.list.EnsureVisible(w.visibleRows)

	for i := 0; i < w.visibleRows; i++ {
		rowIdx := w.list.ScrollTop + i
		if rowIdx >= len(w.rows) {
			break
		}
		row := w.rows[rowIdx]
		y := w.contentY + i

		isSelected := rowIdx == w.list.Selected
		style := row.style
		if isSelected {
			// Draw selection highlight
			surface.ClearRect(w.boxX+1, y, innerW, 1, term.StylePaletteSelected)
			style = term.StylePaletteSelected
		}

		// Draw row content
		text := row.text
		runes := []rune(text)
		if len(runes) > innerW {
			runes = append(runes[:innerW-1], '~')
		}

		switch row.kind {
		case rowHeader, rowGeneral:
			// File headers get a distinctive marker
			marker := "  "
			if row.kind == rowGeneral {
				marker = "  "
			}
			markerStyle := term.StyleCommentMarker
			if isSelected {
				markerStyle = term.StylePaletteSelected
			}
			mx := w.boxX + 1
			for j, ch := range []rune(marker) {
				surface.SetCell(mx+j, y, term.Cell{Ch: ch, Style: markerStyle})
			}
			textX := mx + len([]rune(marker))
			for j, ch := range runes {
				surface.SetCell(textX+j, y, term.Cell{Ch: ch, Style: style})
			}

		case rowUser:
			// User line: bold the @username portion
			for j, ch := range runes {
				s := style
				if j < row.boldPrefix && !isSelected {
					s = term.StyleCommentUser
				}
				surface.SetCell(w.boxX+1+j, y, term.Cell{Ch: ch, Style: s})
			}

		case rowBody:
			for j, ch := range runes {
				surface.SetCell(w.boxX+1+j, y, term.Cell{Ch: ch, Style: style})
			}

		case rowSeparator:
			// leave blank (already cleared)
		}
	}

	// Scrollbar
	if len(w.rows) > w.visibleRows && w.visibleRows > 1 {
		sbX := w.boxX + w.boxW - 2
		ratio := float64(w.list.ScrollTop) / float64(len(w.rows)-w.visibleRows)
		thumbY := w.contentY + int(ratio*float64(w.visibleRows-1))
		for y := w.contentY; y < w.contentY+w.visibleRows; y++ {
			style := term.StyleScrollbar
			if y == thumbY {
				style = term.StyleScrollbarThumb
			}
			surface.SetCell(sbX, y, term.Cell{Ch: ' ', Style: style})
		}
	}
}

func (w *ReviewHubWidget) HandleEvent(ev tcell.Event) EventResult {
	switch tev := ev.(type) {
	case *tcell.EventMouse:
		btn := tev.Buttons()
		_, my := tev.Position()

		if btn&tcell.Button1 != 0 {
			if my >= w.contentY && my < w.contentY+w.visibleRows {
				idx := w.list.ScrollTop + (my - w.contentY)
				if idx >= 0 && idx < len(w.rows) {
					w.list.Selected = idx
					w.activateSelected()
				}
			}
			return EventConsumed
		}
		if btn&tcell.WheelUp != 0 {
			w.list.ScrollTop -= 3
			if w.list.ScrollTop < 0 {
				w.list.ScrollTop = 0
			}
			return EventConsumed
		}
		if btn&tcell.WheelDown != 0 {
			max := len(w.rows) - w.visibleRows
			if max < 0 {
				max = 0
			}
			w.list.ScrollTop += 3
			if w.list.ScrollTop > max {
				w.list.ScrollTop = max
			}
			return EventConsumed
		}
		return EventConsumed

	case *tcell.EventKey:
		switch tev.Key() {
		case tcell.KeyEscape:
			if w.OnDismiss != nil {
				w.OnDismiss()
			}
			return EventConsumed

		case tcell.KeyUp:
			if w.list.Selected > 0 {
				w.list.Selected--
			}
			return EventConsumed

		case tcell.KeyDown:
			if w.list.Selected < len(w.rows)-1 {
				w.list.Selected++
			}
			return EventConsumed

		case tcell.KeyPgUp:
			w.list.Selected -= w.visibleRows
			if w.list.Selected < 0 {
				w.list.Selected = 0
			}
			return EventConsumed

		case tcell.KeyPgDn:
			w.list.Selected += w.visibleRows
			if w.list.Selected >= len(w.rows) {
				w.list.Selected = len(w.rows) - 1
			}
			return EventConsumed

		case tcell.KeyHome:
			w.list.Selected = 0
			w.list.ScrollTop = 0
			return EventConsumed

		case tcell.KeyEnd:
			w.list.Selected = len(w.rows) - 1
			return EventConsumed

		case tcell.KeyEnter:
			w.activateSelected()
			return EventConsumed

		case tcell.KeyRune:
			switch tev.Rune() {
			case 'a', 'A':
				// Add comment
				w.handleAddComment()
				return EventConsumed
			case 'r', 'R':
				// Reply to inline comment
				w.handleReplyInline()
				return EventConsumed
			case 'j':
				if w.list.Selected < len(w.rows)-1 {
					w.list.Selected++
				}
				return EventConsumed
			case 'k':
				if w.list.Selected > 0 {
					w.list.Selected--
				}
				return EventConsumed
			case 'n', 'N':
				// Jump to next file section
				w.jumpNextSection()
				return EventConsumed
			case 'p', 'P':
				// Jump to previous file section
				w.jumpPrevSection()
				return EventConsumed
			}
		}
		return EventConsumed
	}

	return EventConsumed
}

// activateSelected navigates to the file/line of the selected comment.
func (w *ReviewHubWidget) activateSelected() {
	if w.list.Selected < 0 || w.list.Selected >= len(w.rows) {
		return
	}
	row := w.rows[w.list.Selected]
	if row.commentIdx < 0 {
		return
	}

	// Find the original comment
	comment := w.commentByRowIdx(row.commentIdx)
	if comment == nil {
		return
	}
	if comment.IsInline && comment.Path != "" && w.OnNavigate != nil {
		w.OnNavigate(comment.Path, comment.Line)
	}
}

func (w *ReviewHubWidget) handleAddComment() {
	if w.OnAddComment != nil {
		w.OnAddComment()
	}
}

func (w *ReviewHubWidget) handleReplyInline() {
	if w.list.Selected < 0 || w.list.Selected >= len(w.rows) {
		return
	}
	row := w.rows[w.list.Selected]
	if row.commentIdx < 0 {
		return
	}
	comment := w.commentByRowIdx(row.commentIdx)
	if comment == nil || !comment.IsInline {
		return
	}
	if w.OnAddInlineComment != nil {
		w.OnAddInlineComment(comment.Path, comment.Line)
	}
}

// commentByRowIdx returns the comment at the given sequential index
// (inline comments first, then general).
func (w *ReviewHubWidget) commentByRowIdx(idx int) *ReviewComment {
	count := 0
	for i := range w.Comments {
		if w.Comments[i].IsInline {
			if count == idx {
				return &w.Comments[i]
			}
			count++
		}
	}
	for i := range w.Comments {
		if !w.Comments[i].IsInline {
			if count == idx {
				return &w.Comments[i]
			}
			count++
		}
	}
	return nil
}

func (w *ReviewHubWidget) jumpNextSection() {
	for i := w.list.Selected + 1; i < len(w.rows); i++ {
		if w.rows[i].kind == rowHeader || w.rows[i].kind == rowGeneral {
			w.list.Selected = i
			return
		}
	}
}

func (w *ReviewHubWidget) jumpPrevSection() {
	for i := w.list.Selected - 1; i >= 0; i-- {
		if w.rows[i].kind == rowHeader || w.rows[i].kind == rowGeneral {
			w.list.Selected = i
			return
		}
	}
}

// formatCommentDate extracts a readable date from an ISO 8601 timestamp.
func formatCommentDate(iso string) string {
	if len(iso) >= 10 {
		return iso[:10]
	}
	return iso
}
