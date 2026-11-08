package git

type normalizedChangeStatus struct {
	status       FileStatus
	conflictCode string
}

func normalizeChangeStatuses(statuses []FileStatus) []normalizedChangeStatus {
	type pathStatuses struct {
		count            int
		staged, unstaged int
	}
	pairs := make(map[string]pathStatuses, len(statuses))
	for i, status := range statuses {
		pair := pairs[status.Path]
		if pair.count == 0 {
			pair.staged = -1
			pair.unstaged = -1
		}
		pair.count++
		if status.Staged {
			pair.staged = i
		} else {
			pair.unstaged = i
		}
		pairs[status.Path] = pair
	}

	conflicts := make(map[int]string)
	skip := make(map[int]struct{})
	for _, pair := range pairs {
		if pair.count != 2 || pair.staged < 0 || pair.unstaged < 0 {
			continue
		}
		code, ok := porcelainConflictCode(statuses[pair.staged].Status, statuses[pair.unstaged].Status)
		if !ok {
			continue
		}
		first, second := pair.staged, pair.unstaged
		if second < first {
			first, second = second, first
		}
		conflicts[first] = code
		skip[second] = struct{}{}
	}

	normalized := make([]normalizedChangeStatus, 0, len(statuses)-len(skip))
	for i, status := range statuses {
		if _, omitted := skip[i]; omitted {
			continue
		}
		code := conflicts[i]
		if code != "" {
			status.Status = "U"
			status.OldPath = ""
			status.Staged = false
		}
		normalized = append(normalized, normalizedChangeStatus{status: status, conflictCode: code})
	}
	return normalized
}

func porcelainConflictCode(staged, unstaged string) (string, bool) {
	code := staged + unstaged
	switch code {
	case "DD", "AU", "UD", "UA", "DU", "AA", "UU":
		return code, true
	default:
		return "", false
	}
}

func preferredConflictEntry(entries []IndexEntry) (IndexEntry, bool) {
	for _, stage := range []int{2, 3, 1} {
		for _, entry := range entries {
			if entry.Stage == stage {
				return entry, true
			}
		}
	}
	return IndexEntry{}, false
}
