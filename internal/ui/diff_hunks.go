package ui

import "github.com/eugenioenko/ttt/internal/core/diff"

// DiffHunkKind classifies a contiguous change block by what it does to the
// file. A block that both removes and adds lines is a modification.
type DiffHunkKind string

const (
	DiffHunkAdded    DiffHunkKind = "added"
	DiffHunkDeleted  DiffHunkKind = "deleted"
	DiffHunkModified DiffHunkKind = "modified"
)

// DiffHunk is one contiguous change block in a diff surface, in document
// order. Row is the block's first row in the surface's own line space; it is
// internal because plugins address hunks by 1-based index, not by row, so that
// wrapping, split/unified and collapsed files cannot shift the target.
type DiffHunk struct {
	FilePath  string
	Kind      DiffHunkKind
	Row       int
	FileIndex int
}

// DiffHunkTarget is the plugin-facing view of a hunk: a stable position and
// the file it belongs to, with no layout-dependent coordinates.
type DiffHunkTarget struct {
	FilePath string
	Kind     string
}

// diffRowChanged reports whether a row carries a change on either side.
func diffRowChanged(line diff.DiffLine) bool {
	return line.Left.Kind == diff.Deleted || line.Right.Kind == diff.Added
}

// diffHunksInRows groups contiguous changed rows into change blocks. A block
// ends at the first unchanged, collapsed or blank row, so a gap never joins
// two separate changes.
func diffHunksInRows(lines []diff.DiffLine, filePath string, fileIndex int) []DiffHunk {
	var hunks []DiffHunk
	for i := 0; i < len(lines); {
		if !diffRowChanged(lines[i]) {
			i++
			continue
		}
		start := i
		removed, added := false, false
		for i < len(lines) && diffRowChanged(lines[i]) {
			if lines[i].Left.Kind == diff.Deleted {
				removed = true
			}
			if lines[i].Right.Kind == diff.Added {
				added = true
			}
			i++
		}
		kind := DiffHunkModified
		switch {
		case added && !removed:
			kind = DiffHunkAdded
		case removed && !added:
			kind = DiffHunkDeleted
		}
		hunks = append(hunks, DiffHunk{FilePath: filePath, Kind: kind, Row: start, FileIndex: fileIndex})
	}
	return hunks
}
