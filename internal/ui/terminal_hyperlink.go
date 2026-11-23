package ui

import (
	"fmt"
	"sync/atomic"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/xterm-go"
)

var terminalLinkNamespace atomic.Uint64

func terminalLinkID(line *xterm.BufferLine, col int) int {
	if line == nil || col < 0 || col >= line.Len || line.GetBg(col)&xterm.BgFlagHasExtended == 0 {
		return 0
	}
	return line.GetExtended(col).URLID()
}

func (tw *TerminalWidget) setCellLink(xt *xterm.Terminal, line *xterm.BufferLine, col int, cell *term.Cell, links map[int]term.Hyperlink, urls []terminalURLSpan) {
	id := terminalLinkID(line, col)
	if id == 0 {
		cell.Link = urlLinkAt(urls, col)
		return
	}
	data, ok := links[id]
	if !ok {
		if link := xt.GetLinkData(id); link != nil && linkURI(link.URI) != nil && (tw.NativeLinks || linkTarget(link.URI) != nil) {
			// Link IDs belong to one emulator; isolate them across terminal panes.
			if tw.linkNamespace == 0 {
				tw.linkNamespace = terminalLinkNamespace.Add(1)
			}
			data = term.NewHyperlink(link.URI, fmt.Sprintf("ttt-%d-%d", tw.linkNamespace, id))
		}
		links[id] = data
	}
	cell.Link = data
}

type terminalURLSpan struct {
	StartCol, EndCol int
	Link             term.Hyperlink
}

func urlLinkAt(spans []terminalURLSpan, col int) term.Hyperlink {
	for _, span := range spans {
		if col < span.StartCol {
			break
		}
		if col < span.EndCol {
			return span.Link
		}
	}
	return term.Hyperlink{}
}

func (tw *TerminalWidget) urlLinksForText(text []byte) []webLink {
	if spans, ok := tw.urlMemo[string(text)]; ok {
		return spans
	}
	if tw.urlMemo == nil || len(tw.urlMemo) >= linkMemoMax {
		tw.urlMemo = make(map[string][]webLink)
	}
	lineText := string(text)
	spans := webLinks(lineText)
	tw.urlMemo[lineText] = spans
	return spans
}

func (tw *TerminalWidget) urlLinksForLine(buf *xterm.Buffer, row int, cache map[int][]terminalURLSpan) []terminalURLSpan {
	if spans, ok := cache[row]; ok {
		return spans
	}
	if row < 0 || row >= buf.Lines.Length() {
		return nil
	}
	start := row
	for start > 0 {
		line := buf.Lines.Get(start)
		if line == nil || !line.IsWrapped {
			break
		}
		start--
	}
	end := start
	tw.urlText = tw.urlText[:0]
	tw.urlCols = tw.urlCols[:0]
	offset := 0
	for end < buf.Lines.Length() {
		line := buf.Lines.Get(end)
		if line == nil || (end > start && !line.IsWrapped) {
			break
		}
		width := terminalURLLineWidth(buf, end)
		for x := 0; x < width; x += max(1, line.GetWidth(x)) {
			if line.IsCombined(x) != 0 {
				tw.urlText = append(tw.urlText, line.GetString(x)...)
			} else {
				ch := rune(line.GetCodePoint(x))
				if ch == 0 {
					ch = ' '
				}
				tw.urlText = utf8.AppendRune(tw.urlText, ch)
			}
			for len(tw.urlCols) <= len(tw.urlText) {
				tw.urlCols = append(tw.urlCols, offset+x)
			}
			tw.urlCols[len(tw.urlText)] = offset + x + max(1, line.GetWidth(x))
		}
		offset += width
		end++
	}
	spans := tw.urlLinksForText(tw.urlText)
	offset, first := 0, 0
	for y := start; y < end; y++ {
		width := terminalURLLineWidth(buf, y)
		cache[y] = nil
		for first < len(spans) && tw.urlCols[spans[first].End] <= offset {
			first++
		}
		for i := first; i < len(spans); i++ {
			span := spans[i]
			startCol, endCol := tw.urlCols[span.Start], tw.urlCols[span.End]
			if startCol >= offset+width {
				break
			}
			cache[y] = append(cache[y], terminalURLSpan{
				StartCol: max(0, startCol-offset), EndCol: min(width, endCol-offset), Link: span.Link,
			})
		}
		offset += width
	}
	return cache[row]
}

func terminalURLLineWidth(buf *xterm.Buffer, row int) int {
	line := buf.Lines.Get(row)
	width := line.Len
	if width > 0 && line.HasContent(width-1) == 0 && line.GetWidth(width-1) == 1 && row+1 < buf.Lines.Length() {
		next := buf.Lines.Get(row + 1)
		if next != nil && next.IsWrapped && next.Len > 0 && next.GetWidth(0) == 2 {
			// A wide glyph's wrap padding is not part of the logical line text.
			width--
		}
	}
	return width
}
