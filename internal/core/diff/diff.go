package diff

import (
	"fmt"
	"strings"
)

type LineKind int

const (
	Blank LineKind = iota
	Context
	Added
	Deleted
)

type SideLine struct {
	Num  int
	Text string
	Kind LineKind
}

type DiffLine struct {
	Left  SideLine
	Right SideLine
}

type Hunk struct {
	Header string
	Lines  []DiffLine
}

type FileDiff struct {
	Hunks []Hunk
}

func (f *FileDiff) AllLines() []DiffLine {
	var lines []DiffLine
	for i, h := range f.Hunks {
		if i > 0 {
			lines = append(lines, DiffLine{
				Left:  SideLine{Kind: Blank, Text: h.Header},
				Right: SideLine{Kind: Blank, Text: h.Header},
			})
		}
		lines = append(lines, h.Lines...)
	}
	return lines
}

func FullDiffLines(oldLines, newLines []string) []DiffLine {
	lcs := computeLCS(oldLines, newLines)
	var lines []DiffLine
	oi, ni, li := 0, 0, 0

	for oi < len(oldLines) || ni < len(newLines) {
		if li < len(lcs) && oi < len(oldLines) && ni < len(newLines) &&
			oldLines[oi] == lcs[li] && newLines[ni] == lcs[li] {
			lines = append(lines, DiffLine{
				Left:  SideLine{Num: oi + 1, Text: oldLines[oi], Kind: Context},
				Right: SideLine{Num: ni + 1, Text: newLines[ni], Kind: Context},
			})
			oi++
			ni++
			li++
			continue
		}

		var delBuf []int
		var addBuf []int
		for oi < len(oldLines) && (li >= len(lcs) || oldLines[oi] != lcs[li]) {
			delBuf = append(delBuf, oi)
			oi++
		}
		for ni < len(newLines) && (li >= len(lcs) || newLines[ni] != lcs[li]) {
			addBuf = append(addBuf, ni)
			ni++
		}

		maxLen := len(delBuf)
		if len(addBuf) > maxLen {
			maxLen = len(addBuf)
		}
		for i := 0; i < maxLen; i++ {
			dl := DiffLine{}
			if i < len(delBuf) {
				dl.Left = SideLine{Num: delBuf[i] + 1, Text: oldLines[delBuf[i]], Kind: Deleted}
			} else {
				dl.Left = SideLine{Kind: Blank}
			}
			if i < len(addBuf) {
				dl.Right = SideLine{Num: addBuf[i] + 1, Text: newLines[addBuf[i]], Kind: Added}
			} else {
				dl.Right = SideLine{Kind: Blank}
			}
			lines = append(lines, dl)
		}
	}
	return lines
}

func Parse(unified string) FileDiff {
	var fd FileDiff
	lines := strings.Split(unified, "\n")

	var curHunk *Hunk
	var delBuf []string
	var addBuf []string
	oldNum, newNum := 0, 0

	flush := func() {
		if curHunk == nil {
			delBuf = nil
			addBuf = nil
			return
		}
		maxLen := len(delBuf)
		if len(addBuf) > maxLen {
			maxLen = len(addBuf)
		}
		for i := 0; i < maxLen; i++ {
			dl := DiffLine{}
			if i < len(delBuf) {
				dl.Left = SideLine{Num: oldNum, Text: delBuf[i], Kind: Deleted}
				oldNum++
			} else {
				dl.Left = SideLine{Kind: Blank}
			}
			if i < len(addBuf) {
				dl.Right = SideLine{Num: newNum, Text: addBuf[i], Kind: Added}
				newNum++
			} else {
				dl.Right = SideLine{Kind: Blank}
			}
			curHunk.Lines = append(curHunk.Lines, dl)
		}
		delBuf = nil
		addBuf = nil
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "--- ") || strings.HasPrefix(line, "+++ ") {
			continue
		}
		if strings.HasPrefix(line, "@@ ") {
			flush()
			if curHunk != nil {
				fd.Hunks = append(fd.Hunks, *curHunk)
			}
			curHunk = &Hunk{Header: line}
			oldNum, newNum = parseHunkHeader(line)
			continue
		}
		if strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ") || strings.HasPrefix(line, "\\ ") {
			continue
		}
		if curHunk == nil {
			continue
		}
		if strings.HasPrefix(line, "-") {
			delBuf = append(delBuf, line[1:])
		} else if strings.HasPrefix(line, "+") {
			addBuf = append(addBuf, line[1:])
		} else {
			flush()
			text := line
			if len(text) > 0 && text[0] == ' ' {
				text = text[1:]
			}
			curHunk.Lines = append(curHunk.Lines, DiffLine{
				Left:  SideLine{Num: oldNum, Text: text, Kind: Context},
				Right: SideLine{Num: newNum, Text: text, Kind: Context},
			})
			oldNum++
			newNum++
		}
	}

	flush()
	if curHunk != nil {
		fd.Hunks = append(fd.Hunks, *curHunk)
	}

	return fd
}

func Generate(oldLines, newLines []string, fileName string) string {
	lcs := computeLCS(oldLines, newLines)

	var hunks []string
	oi, ni, li := 0, 0, 0
	contextLines := 3

	for oi < len(oldLines) || ni < len(newLines) {
		if li < len(lcs) && oi < len(oldLines) && ni < len(newLines) && oldLines[oi] == lcs[li] && newLines[ni] == lcs[li] {
			oi++
			ni++
			li++
			continue
		}

		hunkOldStart := oi
		hunkNewStart := ni
		ctxStart := hunkOldStart - contextLines
		if ctxStart < 0 {
			ctxStart = 0
		}
		ctxNewStart := hunkNewStart - (hunkOldStart - ctxStart)

		var hunkLines []string
		for i := ctxStart; i < hunkOldStart; i++ {
			hunkLines = append(hunkLines, " "+oldLines[i])
		}

		for oi < len(oldLines) || ni < len(newLines) {
			if li < len(lcs) && oi < len(oldLines) && ni < len(newLines) && oldLines[oi] == lcs[li] && newLines[ni] == lcs[li] {
				peekEnd := 0
				for peekEnd < contextLines*2 && oi+peekEnd < len(oldLines) && li+peekEnd < len(lcs) && oldLines[oi+peekEnd] == lcs[li+peekEnd] {
					peekEnd++
				}
				if peekEnd >= contextLines*2 || (oi+peekEnd >= len(oldLines) && li+peekEnd >= len(lcs)) {
					trail := contextLines
					if peekEnd < trail {
						trail = peekEnd
					}
					for i := 0; i < trail; i++ {
						hunkLines = append(hunkLines, " "+oldLines[oi])
						oi++
						ni++
						li++
					}
					break
				}
				hunkLines = append(hunkLines, " "+oldLines[oi])
				oi++
				ni++
				li++
				continue
			}
			if oi < len(oldLines) && (li >= len(lcs) || oldLines[oi] != lcs[li]) {
				hunkLines = append(hunkLines, "-"+oldLines[oi])
				oi++
				continue
			}
			if ni < len(newLines) && (li >= len(lcs) || newLines[ni] != lcs[li]) {
				hunkLines = append(hunkLines, "+"+newLines[ni])
				ni++
				continue
			}
			break
		}

		oldCount := 0
		newCount := 0
		for _, l := range hunkLines {
			if len(l) > 0 {
				switch l[0] {
				case '-':
					oldCount++
				case '+':
					newCount++
				case ' ':
					oldCount++
					newCount++
				}
			}
		}

		header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", ctxStart+1, oldCount, ctxNewStart+1, newCount)
		hunks = append(hunks, header)
		hunks = append(hunks, hunkLines...)
	}

	if len(hunks) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("--- a/" + fileName + "\n")
	sb.WriteString("+++ b/" + fileName + "\n")
	for _, line := range hunks {
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

func computeLCS(a, b []string) []string {
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] > dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	lcs := make([]string, 0, dp[m][n])
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			lcs = append(lcs, a[i-1])
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}
	for l, r := 0, len(lcs)-1; l < r; l, r = l+1, r-1 {
		lcs[l], lcs[r] = lcs[r], lcs[l]
	}
	return lcs
}

func parseHunkHeader(header string) (oldStart, newStart int) {
	// @@ -oldStart,oldCount +newStart,newCount @@
	parts := strings.Split(header, " ")
	for _, p := range parts {
		if strings.HasPrefix(p, "-") && strings.Contains(p, ",") {
			n := 0
			for _, ch := range p[1:] {
				if ch >= '0' && ch <= '9' {
					n = n*10 + int(ch-'0')
				} else {
					break
				}
			}
			oldStart = n
		}
		if strings.HasPrefix(p, "+") && strings.Contains(p, ",") {
			n := 0
			for _, ch := range p[1:] {
				if ch >= '0' && ch <= '9' {
					n = n*10 + int(ch-'0')
				} else {
					break
				}
			}
			newStart = n
		}
	}
	return oldStart, newStart
}
