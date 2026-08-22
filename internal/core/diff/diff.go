package diff

import (
	"context"
	"fmt"
	"strings"
)

const cancellationCheckInterval = 1024

type LineKind int

const (
	Blank LineKind = iota
	Context
	Added
	Deleted
	Collapsed
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

// FullDiffLinesFromHunks expands a parsed hunk projection in linear time. It
// returns false unless every parsed row and omitted interval agrees with the
// supplied snapshots, allowing callers to preserve the LCS fallback for input
// that did not originate from those exact snapshots.
func FullDiffLinesFromHunks(fileDiff FileDiff, oldLines, newLines []string) ([]DiffLine, bool) {
	capacity := len(oldLines)
	if len(newLines) > capacity {
		capacity = len(newLines)
	}
	lines := make([]DiffLine, 0, capacity)
	oldNext, newNext := 1, 1

	appendContextUntil := func(oldTarget, newTarget int) bool {
		if oldTarget < oldNext || newTarget < newNext || oldTarget-oldNext != newTarget-newNext {
			return false
		}
		for oldNext < oldTarget {
			if oldNext > len(oldLines) || newNext > len(newLines) || oldLines[oldNext-1] != newLines[newNext-1] {
				return false
			}
			lines = append(lines, DiffLine{
				Left:  SideLine{Num: oldNext, Text: oldLines[oldNext-1], Kind: Context},
				Right: SideLine{Num: newNext, Text: newLines[newNext-1], Kind: Context},
			})
			oldNext++
			newNext++
		}
		return true
	}

	for hunkIndex, hunk := range fileDiff.Hunks {
		for lineIndex, line := range hunk.Lines {
			if hunkIndex == len(fileDiff.Hunks)-1 && lineIndex == len(hunk.Lines)-1 &&
				line.Left.Kind == Context && line.Right.Kind == Context &&
				line.Left.Num == len(oldLines)+1 && line.Right.Num == len(newLines)+1 &&
				line.Left.Text == "" && line.Right.Text == "" {
				continue
			}
			oldTarget, newTarget := oldNext, newNext
			if line.Left.Num > 0 {
				oldTarget = line.Left.Num
			}
			if line.Right.Num > 0 {
				newTarget = line.Right.Num
			}
			if !appendContextUntil(oldTarget, newTarget) ||
				!parsedSideMatches(line.Left, oldLines, oldNext, Context, Deleted) ||
				!parsedSideMatches(line.Right, newLines, newNext, Context, Added) {
				return nil, false
			}
			if line.Left.Kind == Context || line.Right.Kind == Context {
				if line.Left.Kind != Context || line.Right.Kind != Context || line.Left.Text != line.Right.Text {
					return nil, false
				}
			}
			lines = append(lines, line)
			if line.Left.Num > 0 {
				oldNext++
			}
			if line.Right.Num > 0 {
				newNext++
			}
		}
	}
	if !appendContextUntil(len(oldLines)+1, len(newLines)+1) {
		return nil, false
	}
	return lines, true
}

func parsedSideMatches(side SideLine, snapshot []string, next int, allowed ...LineKind) bool {
	if side.Num == 0 {
		return side.Kind == Blank && side.Text == ""
	}
	if side.Num != next || side.Num > len(snapshot) || side.Text != snapshot[side.Num-1] {
		return false
	}
	for _, kind := range allowed {
		if side.Kind == kind {
			return true
		}
	}
	return false
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
	generated, _ := GenerateContext(context.Background(), oldLines, newLines, fileName)
	return generated
}

func GenerateContext(ctx context.Context, oldLines, newLines []string, fileName string) (string, error) {
	lcs, err := computeLCSContext(ctx, oldLines, newLines)
	if err != nil {
		return "", err
	}

	var hunks []string
	oi, ni, li := 0, 0, 0
	contextLines := 3
	work := 0
	checkCanceled := func() error {
		work++
		if work%cancellationCheckInterval != 0 {
			return nil
		}
		return ctx.Err()
	}

	for oi < len(oldLines) || ni < len(newLines) {
		if err := checkCanceled(); err != nil {
			return "", err
		}
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
			if err := checkCanceled(); err != nil {
				return "", err
			}
			hunkLines = append(hunkLines, " "+oldLines[i])
		}

		for oi < len(oldLines) || ni < len(newLines) {
			if err := checkCanceled(); err != nil {
				return "", err
			}
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
			if err := checkCanceled(); err != nil {
				return "", err
			}
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
		return "", ctx.Err()
	}

	var sb strings.Builder
	sb.WriteString("--- a/" + fileName + "\n")
	sb.WriteString("+++ b/" + fileName + "\n")
	for _, line := range hunks {
		if err := checkCanceled(); err != nil {
			return "", err
		}
		sb.WriteString(line + "\n")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func computeLCS(a, b []string) []string {
	lcs, _ := computeLCSContext(context.Background(), a, b)
	return lcs
}

func computeLCSContext(ctx context.Context, a, b []string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		dp[i] = make([]int, n+1)
	}
	work := 0
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			work++
			if work%cancellationCheckInterval == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
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
	work = 0
	for i > 0 && j > 0 {
		work++
		if work%cancellationCheckInterval == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
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
		work++
		if work%cancellationCheckInterval == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		lcs[l], lcs[r] = lcs[r], lcs[l]
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return lcs, nil
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
