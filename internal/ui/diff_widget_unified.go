package ui

func (d *DiffViewWidget) unifiedIndexForSource(sourceLine int, right bool) int {
	for i, line := range d.unifiedLines {
		if line.sourceLine == sourceLine && line.right == right {
			return i
		}
	}
	return -1
}

func (d *DiffViewWidget) unifiedIndexForSearchLine(sourceLine int) int {
	if index := d.unifiedIndexForSource(sourceLine, d.searchActiveRight); index >= 0 {
		return index
	}
	for i, line := range d.unifiedLines {
		if line.sourceLine == sourceLine {
			return i
		}
	}
	return sourceLine
}

func unifiedLineVisualRows(line diffUnifiedLine, width int) int {
	return len(diffWrapStarts(line.side.Text, width))
}

func totalUnifiedVisualRows(lines []diffUnifiedLine, width int) int {
	total := 0
	for _, line := range lines {
		total += unifiedLineVisualRows(line, width)
	}
	return total
}

func unifiedLineToVisualRow(lines []diffUnifiedLine, line, width int) int {
	row := 0
	for i := 0; i < line && i < len(lines); i++ {
		row += unifiedLineVisualRows(lines[i], width)
	}
	return row
}

func unifiedVisualRowToTop(lines []diffUnifiedLine, target, width int) (line, offset int) {
	if target <= 0 {
		return 0, 0
	}
	row := 0
	for i, displayLine := range lines {
		rows := unifiedLineVisualRows(displayLine, width)
		if row+rows > target {
			return i, target - row
		}
		row += rows
	}
	if len(lines) == 0 {
		return 0, 0
	}
	last := len(lines) - 1
	return last, unifiedLineVisualRows(lines[last], width) - 1
}

func buildUnifiedWrapMap(lines []diffUnifiedLine, topLine, topOffset, height, width int) []diffWrapEntry {
	entries := make([]diffWrapEntry, 0, height)
	line := topLine
	for len(entries) < height {
		if line >= len(lines) {
			entries = append(entries, diffWrapEntry{line: line, leftStart: -1, rightStart: -1})
			line++
			continue
		}

		starts := diffWrapStarts(lines[line].side.Text, width)
		startRow := 0
		if line == topLine && topOffset < len(starts) {
			startRow = topOffset
		}
		for row := startRow; row < len(starts) && len(entries) < height; row++ {
			entries = append(entries, diffWrapEntry{
				line:         line,
				leftStart:    starts[row],
				rightStart:   -1,
				continuation: row > 0,
			})
		}
		line++
	}
	return entries
}
