package ui

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/core/diff"
)

// compactDiffLinesWithContext keeps Git's hunk projection but can replace an
// individual collapsed separator with the omitted unchanged rows. The map
// identifies which rendered rows remain expandable.
func compactDiffLinesWithContext(fileDiff diff.FileDiff, oldLines, newLines []string, expanded map[int]bool) ([]diff.DiffLine, map[int]int) {
	var lines []diff.DiffLine
	gaps := make(map[int]int)
	for i, hunk := range fileDiff.Hunks {
		if i > 0 {
			leftDistance := hunkLineDistance(fileDiff.Hunks[i-1], hunk, false)
			rightDistance := hunkLineDistance(fileDiff.Hunks[i-1], hunk, true)
			if leftDistance <= 0 {
				leftDistance = rightDistance
			}
			if rightDistance <= 0 {
				rightDistance = leftDistance
			}

			if leftDistance > 0 || rightDistance > 0 {
				gap := i - 1
				if expanded[gap] && (oldLines != nil || newLines != nil) {
					lines = append(lines, expandedHunkGap(fileDiff.Hunks[i-1], hunk, oldLines, newLines)...)
					lines = append(lines, hunk.Lines...)
					continue
				}
				leftLabel, rightLabel := hunk.Header, hunk.Header
				if leftDistance > 0 {
					leftLabel = collapsedDistanceLabel(leftDistance)
				}
				if rightDistance > 0 {
					rightLabel = collapsedDistanceLabel(rightDistance)
				}
				gaps[len(lines)] = gap
				lines = append(lines, diff.DiffLine{
					Left:  diff.SideLine{Kind: diff.Collapsed, Text: leftLabel},
					Right: diff.SideLine{Kind: diff.Collapsed, Text: rightLabel},
				})
			}
		}
		lines = append(lines, hunk.Lines...)
	}
	return lines, gaps
}

func expandedHunkGap(previous, next diff.Hunk, oldLines, newLines []string) []diff.DiffLine {
	previousLeft, previousRight := hunkLastNumbers(previous)
	nextLeft, nextRight := hunkFirstNumbers(next)
	leftCount := nextLeft - previousLeft - 1
	rightCount := nextRight - previousRight - 1
	if leftCount < 0 {
		leftCount = 0
	}
	if rightCount < 0 {
		rightCount = 0
	}
	count := max(leftCount, rightCount)
	lines := make([]diff.DiffLine, 0, count)
	for offset := 1; offset <= count; offset++ {
		line := diff.DiffLine{}
		if offset <= leftCount {
			num := previousLeft + offset
			line.Left = diff.SideLine{Num: num, Text: lineAt(oldLines, num), Kind: diff.Context}
		} else {
			line.Left = diff.SideLine{Kind: diff.Blank}
		}
		if offset <= rightCount {
			num := previousRight + offset
			line.Right = diff.SideLine{Num: num, Text: lineAt(newLines, num), Kind: diff.Context}
		} else {
			line.Right = diff.SideLine{Kind: diff.Blank}
		}
		lines = append(lines, line)
	}
	return lines
}

func hunkLastNumbers(hunk diff.Hunk) (left, right int) {
	for _, line := range hunk.Lines {
		if line.Left.Num > left {
			left = line.Left.Num
		}
		if line.Right.Num > right {
			right = line.Right.Num
		}
	}
	return left, right
}

func hunkFirstNumbers(hunk diff.Hunk) (left, right int) {
	for _, line := range hunk.Lines {
		if left == 0 && line.Left.Num > 0 {
			left = line.Left.Num
		}
		if right == 0 && line.Right.Num > 0 {
			right = line.Right.Num
		}
	}
	return left, right
}

func lineAt(lines []string, oneBased int) string {
	if oneBased <= 0 || oneBased > len(lines) {
		return ""
	}
	return lines[oneBased-1]
}

func hunkLineDistance(previous, next diff.Hunk, right bool) int {
	previousLine := 0
	for _, line := range previous.Lines {
		num := line.Left.Num
		if right {
			num = line.Right.Num
		}
		if num > previousLine {
			previousLine = num
		}
	}

	nextLine := 0
	for _, line := range next.Lines {
		num := line.Left.Num
		if right {
			num = line.Right.Num
		}
		if num > 0 {
			nextLine = num
			break
		}
	}
	return nextLine - previousLine - 1
}

func collapsedDistanceLabel(distance int) string {
	unit := "lines"
	if distance == 1 {
		unit = "line"
	}
	return fmt.Sprintf(" ⋯ %d %s ⋯", distance, unit)
}
