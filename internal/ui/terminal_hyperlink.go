package ui

import (
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"unicode"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/textwidth"
	"github.com/eugenioenko/xterm-go"
)

var terminalLinkNamespace atomic.Uint64

func terminalLinkID(line *xterm.BufferLine, col int) int {
	if line == nil || col < 0 || col >= line.Len || line.GetBg(col)&xterm.BgFlagHasExtended == 0 {
		return 0
	}
	return line.GetExtended(col).URLID()
}

func terminalLinkURI(raw string) *url.URL {
	if strings.ContainsFunc(raw, unicode.IsControl) {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" {
		return nil
	}
	switch u.Scheme {
	case "http", "https":
		if u.Hostname() != "" {
			return u
		}
	case "file":
		if strings.HasPrefix(u.Path, "/") && !strings.ContainsFunc(u.Path, unicode.IsControl) {
			return u
		}
	}
	return nil
}

func (tw *TerminalWidget) setCellLink(xt *xterm.Terminal, line *xterm.BufferLine, col int, cell *term.Cell, links map[int]term.Hyperlink, urls []terminalURLSpan) {
	id := terminalLinkID(line, col)
	if id == 0 {
		for _, span := range urls {
			if col >= span.StartCol && col < span.EndCol {
				cell.Link = span.Link
				break
			}
		}
		return
	}
	data, ok := links[id]
	if !ok {
		if link := xt.GetLinkData(id); link != nil && terminalLinkURI(link.URI) != nil {
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

func (tw *TerminalWidget) urlLinksForText(text []byte) []terminalURLSpan {
	if spans, ok := tw.urlMemo[string(text)]; ok {
		return spans
	}
	if tw.urlMemo == nil || len(tw.urlMemo) >= linkMemoMax {
		tw.urlMemo = make(map[string][]terminalURLSpan)
	}
	var spans []terminalURLSpan
	lineText := string(text)
	for _, candidate := range detectURLCandidates(lineText) {
		if terminalLinkURI(candidate.URL) == nil {
			continue
		}
		runes := []rune(lineText)
		start := textwidth.GraphemeWidth(string(runes[:candidate.StartCol]))
		spans = append(spans, terminalURLSpan{
			StartCol: start, EndCol: start + textwidth.GraphemeWidth(candidate.URL),
			Link: term.NewHyperlink(candidate.URL, ""),
		})
	}
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
		}
		end++
	}
	spans := tw.urlLinksForText(tw.urlText)
	offset := 0
	for y := start; y < end; y++ {
		width := terminalURLLineWidth(buf, y)
		cache[y] = nil
		for _, span := range spans {
			if span.StartCol < offset+width && span.EndCol > offset {
				cache[y] = append(cache[y], terminalURLSpan{
					StartCol: max(0, span.StartCol-offset), EndCol: min(width, span.EndCol-offset), Link: span.Link,
				})
			}
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
