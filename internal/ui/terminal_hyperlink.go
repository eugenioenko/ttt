package ui

import (
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"unicode"

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

func (tw *TerminalWidget) setCellLink(xt *xterm.Terminal, line *xterm.BufferLine, col int, cell *term.Cell, links map[int]term.Hyperlink) {
	id := terminalLinkID(line, col)
	if id == 0 {
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
