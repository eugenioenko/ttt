package ui

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/core/diff"
)

func compactDiffLines(fileDiff diff.FileDiff) []diff.DiffLine {
	var lines []diff.DiffLine
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
				leftLabel, rightLabel := hunk.Header, hunk.Header
				if leftDistance > 0 {
					leftLabel = collapsedDistanceLabel(leftDistance)
				}
				if rightDistance > 0 {
					rightLabel = collapsedDistanceLabel(rightDistance)
				}
				lines = append(lines, diff.DiffLine{
					Left:  diff.SideLine{Kind: diff.Collapsed, Text: leftLabel},
					Right: diff.SideLine{Kind: diff.Collapsed, Text: rightLabel},
				})
			}
		}
		lines = append(lines, hunk.Lines...)
	}
	return lines
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
	return fmt.Sprintf("⋯ %d %s ⋯", distance, unit)
}
