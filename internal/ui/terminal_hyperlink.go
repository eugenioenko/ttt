package ui

import (
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
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

func terminalLinkTarget(raw string) *linkSpan {
	u := terminalLinkURI(raw)
	if u == nil {
		return nil
	}
	if u.Scheme != "file" {
		return &linkSpan{URL: raw}
	}
	if (u.Host != "" && !strings.EqualFold(u.Host, "localhost")) || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil
	}
	path := u.Path
	if runtime.GOOS == "windows" && len(path) >= 3 && path[2] == ':' {
		path = path[1:]
	}
	return &linkSpan{IsFile: true, FilePath: filepath.FromSlash(path)}
}

func (tw *TerminalWidget) setCellLink(xt *xterm.Terminal, line *xterm.BufferLine, col int, cell *term.Cell, links map[int]xterm.OscLinkData) {
	id := terminalLinkID(line, col)
	if id == 0 {
		return
	}
	data, ok := links[id]
	if !ok {
		if link := xt.GetLinkData(id); link != nil && terminalLinkURI(link.URI) != nil {
			data.URI = link.URI
			// Link IDs belong to one emulator; isolate them across terminal panes.
			if tw.linkNamespace == 0 {
				tw.linkNamespace = terminalLinkNamespace.Add(1)
			}
			data.ID = fmt.Sprintf("ttt-%d-%d", tw.linkNamespace, id)
		}
		links[id] = data
	}
	cell.URL, cell.URLID = data.URI, data.ID
}
