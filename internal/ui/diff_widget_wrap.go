package ui

import (
	"github.com/eugenioenko/ttt/internal/core/diff"
)

type diffWrapEntry struct {
	line         int
	leftStart    int
	rightStart   int
	continuation bool
}

type diffLogicalAnchor struct {
	lineNum    int
	right      bool
	sourceLine int
}

func diffLineVisualRows(line diff.DiffLine, leftW, rightW int) int {
	leftRows := len(diffWrapStarts(line.Left.Text, leftW))
	rightRows := len(diffWrapStarts(line.Right.Text, rightW))
	if rightRows > leftRows {
		return rightRows
	}
	return leftRows
}

func totalDiffVisualRows(lines []diff.DiffLine, leftW, rightW int) int {
	total := 0
	for _, line := range lines {
		total += diffLineVisualRows(line, leftW, rightW)
	}
	return total
}

func diffLineToVisualRow(lines []diff.DiffLine, line, leftW, rightW int) int {
	row := 0
	for i := 0; i < line && i < len(lines); i++ {
		row += diffLineVisualRows(lines[i], leftW, rightW)
	}
	return row
}

func diffVisualRowToTop(lines []diff.DiffLine, target, leftW, rightW int) (line, offset int) {
	if target <= 0 {
		return 0, 0
	}
	row := 0
	for i, dl := range lines {
		rows := diffLineVisualRows(dl, leftW, rightW)
		if row+rows > target {
			return i, target - row
		}
		row += rows
	}
	if len(lines) == 0 {
		return 0, 0
	}
	return len(lines) - 1, diffLineVisualRows(lines[len(lines)-1], leftW, rightW) - 1
}

func buildDiffWrapMap(lines []diff.DiffLine, topLine, topOffset, height, leftW, rightW int) []diffWrapEntry {
	entries := make([]diffWrapEntry, 0, height)
	line := topLine
	for len(entries) < height {
		if line >= len(lines) {
			entries = append(entries, diffWrapEntry{line: line, leftStart: -1, rightStart: -1})
			line++
			continue
		}

		leftStarts := diffWrapStarts(lines[line].Left.Text, leftW)
		rightStarts := diffWrapStarts(lines[line].Right.Text, rightW)
		rows := len(leftStarts)
		if len(rightStarts) > rows {
			rows = len(rightStarts)
		}
		startRow := 0
		if line == topLine && topOffset < rows {
			startRow = topOffset
		}
		for row := startRow; row < rows && len(entries) < height; row++ {
			leftStart, rightStart := -1, -1
			if row < len(leftStarts) {
				leftStart = leftStarts[row]
			}
			if row < len(rightStarts) {
				rightStart = rightStarts[row]
			}
			entries = append(entries, diffWrapEntry{
				line:         line,
				leftStart:    leftStart,
				rightStart:   rightStart,
				continuation: row > 0,
			})
		}
		line++
	}
	return entries
}

func diffSegmentVisualColToRune(text string, startCol, visualCol int) int {
	if startCol < 0 {
		return 0
	}
	runes := []rune(text)
	if startCol >= len(runes) {
		return len(runes)
	}
	return startCol + visualColToBufCol(string(runes[startCol:]), visualCol, diffTabWidth)
}

func (d *DiffViewWidget) topVisualRow() int {
	if !d.IsWrapped() {
		return d.TopLine
	}
	if d.IsUnified() {
		return unifiedLineToVisualRow(d.unifiedLines, d.TopLine, d.layoutLeftW) + d.wrapTopOffset
	}
	return diffLineToVisualRow(d.Lines, d.TopLine, d.layoutLeftW, d.layoutRightW) + d.wrapTopOffset
}

func (d *DiffViewWidget) topSourceLine() int {
	if !d.IsUnified() {
		return d.TopLine
	}
	if len(d.unifiedLines) == 0 {
		return 0
	}
	line := min(max(d.TopLine, 0), len(d.unifiedLines)-1)
	return d.unifiedLines[line].sourceLine
}

func (d *DiffViewWidget) displayLineForSourceLine(sourceLine int) int {
	if !d.IsUnified() {
		return min(max(sourceLine, 0), max(len(d.Lines)-1, 0))
	}
	for line, unified := range d.unifiedLines {
		if unified.sourceLine >= sourceLine {
			return line
		}
	}
	return max(len(d.unifiedLines)-1, 0)
}

func (d *DiffViewWidget) logicalTopAnchor() diffLogicalAnchor {
	sourceLine := d.topSourceLine()
	right := false
	if d.IsUnified() && d.TopLine >= 0 && d.TopLine < len(d.unifiedLines) {
		right = d.unifiedLines[d.TopLine].right
	}
	anchor := diffLogicalAnchor{right: right, sourceLine: sourceLine}
	for distance := 0; distance < len(d.Lines); distance++ {
		indexes := []int{sourceLine - distance}
		if distance > 0 {
			indexes = append(indexes, sourceLine+distance)
		}
		for _, index := range indexes {
			if index < 0 || index >= len(d.Lines) {
				continue
			}
			line := d.Lines[index]
			if line.Left.Num == 0 && line.Right.Num == 0 {
				continue
			}
			if (anchor.right && line.Right.Num > 0) || line.Left.Num == 0 {
				anchor.lineNum = line.Right.Num
				anchor.right = true
			} else {
				anchor.lineNum = line.Left.Num
				anchor.right = false
			}
			return anchor
		}
	}
	return anchor
}

func (d *DiffViewWidget) displayLineForLogicalAnchor(anchor diffLogicalAnchor) int {
	sourceLine := -1
	bestDistance := -1
	for index, line := range d.Lines {
		lineNum := line.Left.Num
		if anchor.right {
			lineNum = line.Right.Num
		}
		if lineNum == 0 {
			continue
		}
		distance := diffLineNumberDistance(anchor.lineNum, lineNum)
		if bestDistance >= 0 && distance >= bestDistance {
			continue
		}
		sourceLine = index
		bestDistance = distance
	}
	if sourceLine < 0 {
		sourceLine = min(max(anchor.sourceLine, 0), max(len(d.Lines)-1, 0))
	}
	if !d.IsUnified() {
		return sourceLine
	}
	if line := d.unifiedIndexForSource(sourceLine, anchor.right); line >= 0 {
		return line
	}
	for line, unified := range d.unifiedLines {
		if unified.sourceLine == sourceLine {
			return line
		}
	}
	return max(len(d.unifiedLines)-1, 0)
}

func diffLineNumberDistance(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

func (d *DiffViewWidget) setTopVisualRow(row int) {
	maxTop := d.totalVisualRows - d.viewH
	if maxTop < 0 {
		maxTop = 0
	}
	if row < 0 {
		row = 0
	}
	if row > maxTop {
		row = maxTop
	}
	if d.IsWrapped() {
		if d.IsUnified() {
			d.TopLine, d.wrapTopOffset = unifiedVisualRowToTop(d.unifiedLines, row, d.layoutLeftW)
		} else {
			d.TopLine, d.wrapTopOffset = diffVisualRowToTop(d.Lines, row, d.layoutLeftW, d.layoutRightW)
		}
	} else {
		d.TopLine = row
		d.wrapTopOffset = 0
	}
}

func (d *DiffViewWidget) scrollVertical(rows int) {
	d.setTopVisualRow(d.topVisualRow() + rows)
}
