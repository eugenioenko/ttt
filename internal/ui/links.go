package ui

import (
	"regexp"
	"strconv"
)

type linkSpan struct {
	StartCol int
	EndCol   int // exclusive
	URL      string
	IsFile   bool
	FilePath string
	Line     int // 1-based line number for file links (0 = no line)
	Col      int // 1-based column number for file links (0 = no column)
}

const linkMemoMax = 512

func resolveLinks(spans []linkSpan, dir string, resolve func(string, string) string) []linkSpan {
	links := spans[:0]
	for _, span := range spans {
		if span.IsFile {
			if resolve == nil {
				continue
			}
			span.FilePath = resolve(span.FilePath, dir)
			if span.FilePath == "" {
				continue
			}
		}
		links = append(links, span)
	}
	return links
}

var (
	urlRe      = regexp.MustCompile(`https?://[^\s)>\]'"` + "`" + `]+`)
	fileLineRe = regexp.MustCompile(`(?:^|[\s(])((?:[A-Za-z]:[\\/])?[^\s:*?"<>|]+):(\d+)(?::(\d+))?`)
)

func byteToRunePos(s string, byteIdx int) int {
	return len([]rune(s[:byteIdx]))
}

func detectLinks(text string) []linkSpan {
	var spans []linkSpan

	for _, loc := range urlRe.FindAllStringIndex(text, -1) {
		url := text[loc[0]:loc[1]]
		for len(url) > 0 {
			last := url[len(url)-1]
			if last == '.' || last == ',' || last == ';' || last == ':' {
				url = url[:len(url)-1]
			} else {
				break
			}
		}
		startCol := byteToRunePos(text, loc[0])
		endCol := startCol + len([]rune(url))
		spans = append(spans, linkSpan{
			StartCol: startCol,
			EndCol:   endCol,
			URL:      url,
		})
	}

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
