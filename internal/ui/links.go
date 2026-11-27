package ui

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/term"
)

type linkSpan struct {
	StartCol int
	EndCol   int // exclusive
	URL      string
	Link     term.Hyperlink
	IsFile   bool
	FilePath string
	Line     int // 1-based line number for file links (0 = no line)
	Col      int // 1-based column number for file links (0 = no column)
}

var (
	urlRe      = regexp.MustCompile(`https?://[^\s)>\]'"` + "`" + `]+`)
	fileLineRe = regexp.MustCompile(`(?:^|[\s(])((?:[A-Za-z]:[\\/])?[^\s:*?"<>|]+):(\d+)(?::(\d+))?`)
)

// linkMemoMax bounds the per-widget link detection memo. When the memo grows
// past this many unique line texts it is reset, so long sessions with lots of
// distinct output do not accumulate memory indefinitely.
const linkMemoMax = 512

func byteToRunePos(s string, byteIdx int) int {
	return len([]rune(s[:byteIdx]))
}

func detectLinks(text string, workDir string) []linkSpan {
	spans := detectLinkCandidates(text)
	resolved := spans[:0]
	for _, span := range spans {
		if span.IsFile {
			span.FilePath = ResolveFilePath(span.FilePath, workDir)
			if span.FilePath == "" {
				continue
			}
		}
		resolved = append(resolved, span)
	}
	return resolved
}

type webLink struct {
	Start, End int // byte offsets
	Link       term.Hyperlink
}

func webLinks(text string) []webLink {
	var links []webLink
	for _, loc := range urlRe.FindAllStringIndex(text, -1) {
		url := strings.TrimRight(text[loc[0]:loc[1]], ".,;:")
		if linkURI(url) != nil {
			links = append(links, webLink{loc[0], loc[0] + len(url), term.NewHyperlink(url, "")})
		}
	}
	return links
}

func detectURLCandidates(text string) []linkSpan {
	var spans []linkSpan
	lastByte, col := 0, 0
	for _, link := range webLinks(text) {
		col += utf8.RuneCountInString(text[lastByte:link.Start])
		end := col + utf8.RuneCountInString(text[link.Start:link.End])
		spans = append(spans, linkSpan{StartCol: col, EndCol: end, URL: link.Link.URL(), Link: link.Link})
		lastByte, col = link.End, end
	}
	return spans
}

func detectLinkCandidates(text string) []linkSpan {
	spans := detectURLCandidates(text)
	for _, match := range fileLineRe.FindAllStringSubmatchIndex(text, -1) {
		filePath := text[match[2]:match[3]]
		lineStr := text[match[4]:match[5]]
		lineNum, err := strconv.Atoi(lineStr)
		if err != nil {
			continue
		}

		spanEnd := match[5]
		colNum := 0
		if match[6] != -1 {
			spanEnd = match[7]
			if c, err := strconv.Atoi(text[match[6]:match[7]]); err == nil {
				colNum = c
			}
		}

		startCol := byteToRunePos(text, match[2])
		endCol := byteToRunePos(text, spanEnd)
		overlaps := false
		for _, existing := range spans {
			if startCol < existing.EndCol && endCol > existing.StartCol {
				overlaps = true
				break
			}
		}
		if overlaps {
			continue
		}

		spans = append(spans, linkSpan{
			StartCol: startCol,
			EndCol:   endCol,
			IsFile:   true,
			FilePath: filePath,
			Line:     lineNum,
			Col:      colNum,
		})
	}

	return spans
}

func ResolveFilePath(path string, workDir string) string {
	if path == "" {
		return ""
	}

	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	if filepath.IsAbs(path) {
		if _, err := os.Stat(path); err == nil {
			return path
		}
		return ""
	}

	if workDir != "" {
		abs := filepath.Join(workDir, path)
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
	}

	return ""
}

func linkURI(raw string) *url.URL {
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

func linkTarget(raw string) *linkSpan {
	u := linkURI(raw)
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

func openLink(span linkSpan, onURL func(string), onFile func(string, int, int)) {
	if span.IsFile {
		if onFile != nil {
			onFile(span.FilePath, span.Line, span.Col)
		}
	} else if onURL != nil {
		onURL(span.URL)
	}
}
