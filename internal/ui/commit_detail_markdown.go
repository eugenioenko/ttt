package ui

import (
	"strings"

	"github.com/eugenioenko/ttt/internal/markdown"
	"github.com/eugenioenko/ttt/internal/term"
)

func (d *CommitDetailWidget) SetMarkdownMaxWidth(width int) {
	if d.markdownMaxWidth != width {
		d.markdownMaxWidth = width
		d.visualRowsW = -1
	}
}

func (d *CommitDetailWidget) buildMessageRows() []commitDetailRow {
	message := strings.TrimRight(d.Message, "\r\n")
	if message == "" {
		message = d.EmptyMessage
		if message == "" {
			message = "(No commit message)"
			if d.CurrentChanges {
				message = "Working tree clean"
			}
		}
	}
	if d.messageRows != nil && d.messageSource == message {
		return d.messageRows
	}
	d.messageSource = message
	d.messageRows = nil
	d.messageMaxLineW = 0
	for _, line := range markdown.Render(message) {
		text := line.Text()
		d.messageRows = append(d.messageRows, commitDetailRow{
			kind: commitDetailMessageRow, text: text, markdown: line, styles: markdown.FlattenStyles(line),
		})
		if !line.Kind.Wrappable() && line.Kind != markdown.KindDivider {
			d.messageMaxLineW = max(d.messageMaxLineW, diffLineVisualWidth(text))
		}
	}
	return d.messageRows
}

func (d *CommitDetailWidget) messageWrapWidth(viewW int) int {
	width := max(1, viewW-2)
	if d.markdownMaxWidth > 0 {
		width = min(width, d.markdownMaxWidth)
	}
	return width
}

func (d *CommitDetailWidget) messageVisualRows(index int, row commitDetailRow, viewW int) []commitDetailVisualRow {
	lines := []markdown.Line{row.markdown}
	if row.markdown.Kind.Wrappable() {
		lines = markdown.WrapLine(row.markdown, d.messageWrapWidth(viewW))
	}
	rows := make([]commitDetailVisualRow, 0, len(lines))
	start := 0
	for _, line := range lines {
		text := line.Text()
		end := start + len([]rune(text))
		rows = append(rows, commitDetailVisualRow{row: index, leftStart: start, messageEnd: end, messageText: text, continuation: start > 0})
		start = end
	}
	return rows
}

func (d *CommitDetailWidget) renderMarkdownRow(surface Surface, index int, row commitDetailRow, visual commitDetailVisualRow, y, viewW int) {
	width := max(0, viewW-2)
	if row.markdown.Kind == markdown.KindDivider {
		for x := 1; x <= width; x++ {
			surface.SetCell(x, y, term.Cell{Ch: '─', Style: term.StyleBorder})
		}
		return
	}
	left := d.LeftCol
	if row.markdown.Kind.Wrappable() {
		left = 0
		width = min(width, d.messageWrapWidth(viewW))
	}
	blank := term.Cell{Ch: ' ', BgStyle: term.StyleCommitHeader}
	drawTextSegment(surface, 1, y, width, visual.messageText, 0, left, blank, func(col int, ch rune) term.Cell {
		col += visual.leftStart
		cell := term.Cell{Ch: ch, Style: row.styles[col], BgStyle: term.StyleCommitHeader}
		if d.hasSelection && d.selection.Contains(index, col) {
			cell.BgStyle = term.StyleSelection
		}
		return cell
	})
}

func (d *CommitDetailWidget) horizontalLimit(viewW int) int {
	limit := 0
	if viewW > 2 {
		limit = max(0, d.messageMaxLineW-(viewW-2))
	}
	if !d.IsWrapped() {
		_, leftW, _, rightW := d.sideGeometry(viewW)
		if d.mode == DiffModeSplit {
			leftW = min(leftW, rightW)
		}
		if leftW > 0 {
			limit = max(limit, d.maxLineW-leftW)
		}
	}
	return limit
}
