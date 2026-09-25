package diff

import "context"

// LineChangeKind indicates how a buffer line differs from the HEAD version.
type LineChangeKind int

const (
	LineUnchanged LineChangeKind = iota
	LineAdded
	LineModified
	LineDeleted // marks the line *after* a deletion (the deletion happened between this line and the previous)
)

// ComputeGutterChanges compares oldLines (from git HEAD) with newLines (current
// buffer) and returns a per-line change indicator for newLines. The returned
// slice has len(newLines) entries.
//
// The algorithm maps FullDiffLines output to per-buffer-line indicators:
//   - Context lines are unchanged.
//   - A right-side Added line paired with a left-side Deleted line is Modified.
//   - A right-side Added line with a Blank left side is Added.
//   - A left-side Deleted line with a Blank right side marks the next buffer
//     line as having a deletion above it (LineDeleted indicator).
func ComputeGutterChanges(oldLines, newLines []string) []LineChangeKind {
	changes, _ := ComputeGutterChangesContext(context.Background(), oldLines, newLines)
	return changes
}

func ComputeGutterChangesContext(ctx context.Context, oldLines, newLines []string) ([]LineChangeKind, error) {
	if len(newLines) == 0 {
		return nil, nil
	}

	result := make([]LineChangeKind, len(newLines))

	diffs, err := FullDiffLinesContext(ctx, oldLines, newLines)
	if err != nil {
		return nil, err
	}

	// pendingDelete tracks whether we saw deleted lines that haven't been
	// paired with additions. When we encounter the next buffer line (context
	// or added), we mark it as LineDeleted if pendingDelete is true and it
	// hasn't been assigned a stronger status (added/modified).
	pendingDelete := false

	for _, dl := range diffs {
		hasLeft := dl.Left.Kind == Deleted
		hasRight := dl.Right.Kind == Added
		isContext := dl.Right.Kind == Context

		if hasLeft && hasRight {
			// Modified line: old line was deleted, new line was added at same position
			idx := dl.Right.Num - 1
			if idx >= 0 && idx < len(result) {
				result[idx] = LineModified
			}
			pendingDelete = false
		} else if hasRight {
			// Added line
			idx := dl.Right.Num - 1
			if idx >= 0 && idx < len(result) {
				result[idx] = LineAdded
			}
			pendingDelete = false
		} else if hasLeft {
			// Deleted line with no corresponding new line
			pendingDelete = true
		} else if isContext {
			// Context (unchanged) line
			if pendingDelete {
				idx := dl.Right.Num - 1
				if idx >= 0 && idx < len(result) && result[idx] == LineUnchanged {
					result[idx] = LineDeleted
				}
				pendingDelete = false
			}
		}
	}

	// If there were trailing deletions at the end of the file, mark the last
	// buffer line.
	if pendingDelete && len(result) > 0 {
		lastIdx := len(result) - 1
		if result[lastIdx] == LineUnchanged {
			result[lastIdx] = LineDeleted
		}
	}

	return result, nil
}
