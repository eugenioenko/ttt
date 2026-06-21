package ui

import (
	"regexp"
	"sort"
	"strings"

	"github.com/eugenioenko/ttt/internal/core/diff"
)

type SearchOptions struct {
	CaseSensitive bool
	UseRegex      bool
}

func FindInLines(lines []string, query string, opts SearchOptions) ([]FindMatch, error) {
	if query == "" {
		return nil, nil
	}

	if opts.UseRegex {
		return findRegex(lines, query, opts)
	}
	return findPlain(lines, query, opts), nil
}

func findPlain(lines []string, query string, opts SearchOptions) []FindMatch {
	searchQuery := query
	if !opts.CaseSensitive {
		searchQuery = strings.ToLower(query)
	}
	queryLen := len([]rune(searchQuery))
	var matches []FindMatch

	for lineIdx, line := range lines {
		searchLine := line
		if !opts.CaseSensitive {
			searchLine = strings.ToLower(line)
		}
		offset := 0
		for {
			idx := strings.Index(searchLine[offset:], searchQuery)
			if idx < 0 {
				break
			}
			bytePos := offset + idx
			col := len([]rune(searchLine[:bytePos]))
			matches = append(matches, FindMatch{Line: lineIdx, Col: col, Len: queryLen})
			offset = bytePos + len(searchQuery)
		}
	}
	return matches
}

func findRegex(lines []string, query string, opts SearchOptions) ([]FindMatch, error) {
	pattern := query
	if !opts.CaseSensitive {
		pattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	var matches []FindMatch
	for lineIdx, line := range lines {
		locs := re.FindAllStringIndex(line, -1)
		for _, loc := range locs {
			col := len([]rune(line[:loc[0]]))
			matchLen := len([]rune(line[loc[0]:loc[1]]))
			matches = append(matches, FindMatch{Line: lineIdx, Col: col, Len: matchLen})
		}
	}
	return matches, nil
}

type searchMatchByPos []SearchMatch

func (s searchMatchByPos) Len() int      { return len(s) }
func (s searchMatchByPos) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s searchMatchByPos) Less(i, j int) bool {
	if s[i].LineNum != s[j].LineNum {
		return s[i].LineNum < s[j].LineNum
	}
	return s[i].ColStart < s[j].ColStart
}

func ApplyReplacements(lines []string, matches []SearchMatch, replacement string, opts SearchOptions) []string {
	result := make([]string, len(lines))
	copy(result, lines)

	sorted := make([]SearchMatch, len(matches))
	copy(sorted, matches)
	sort.Sort(sort.Reverse(searchMatchByPos(sorted)))

	for _, m := range sorted {
		idx := m.LineNum - 1
		if idx < 0 || idx >= len(result) {
			continue
		}
		line := result[idx]
		start := m.ColStart
		end := m.ColEnd
		if start > len(line) {
			start = len(line)
		}
		if end > len(line) {
			end = len(line)
		}
		result[idx] = line[:start] + replacement + line[end:]
	}
	return result
}

func BuildReplaceDiff(filePath string, lines []string, matches []SearchMatch, replacement string, opts SearchOptions) diff.FileDiff {
	newLines := ApplyReplacements(lines, matches, replacement, opts)
	unified := diff.Generate(lines, newLines, filePath)
	return diff.Parse(unified)
}
