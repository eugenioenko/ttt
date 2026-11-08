package git

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

func conflictCodeFromEntries(entries []IndexEntry) string {
	mask := 0
	for _, entry := range entries {
		mask |= 1 << entry.Stage
	}
	return map[int]string{2: "DD", 4: "AU", 8: "UA", 6: "UD", 10: "DU", 12: "AA", 14: "UU"}[mask]
}
