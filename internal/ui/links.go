package ui

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
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

var (
	urlRe      = regexp.MustCompile(`https?://[^\s)>\]'"` + "`" + `]+`)
	fileLineRe = regexp.MustCompile(`(?:^|[\s(])((?:[A-Za-z]:[\\/])?[^\s:*?"<>|]+):(\d+)(?::(\d+))?`)
)

func byteToRunePos(s string, byteIdx int) int {
	return len([]rune(s[:byteIdx]))
}

func detectLinks(text string, workDir string) []linkSpan {
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

		resolvedPath := resolveFilePath(filePath, workDir)
		if resolvedPath == "" {
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
			FilePath: resolvedPath,
			Line:     lineNum,
			Col:      colNum,
		})
	}

	return spans
}

func resolveFilePath(path string, workDir string) string {
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
