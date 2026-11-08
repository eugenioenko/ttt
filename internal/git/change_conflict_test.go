package git

import (
	"github.com/eugenioenko/ttt/internal/scm"
	"testing"
)

func TestNormalizeCurrentChangeStatusesMapsEveryPorcelainConflictShape(t *testing.T) {
	for _, code := range []string{"DD", "AU", "UD", "UA", "DU", "AA", "UU"} {
		t.Run(code, func(t *testing.T) {
			statuses := []FileStatus{
				{Status: code[:1], Path: "conflict.txt", Staged: true},
				{Status: code[1:], Path: "conflict.txt", Staged: false},
			}
			normalized := normalizeChangeStatuses(statuses)
			if len(normalized) != 1 || normalized[0].conflictCode != code || normalized[0].status.Status != "U" || normalized[0].status.Staged {
				t.Fatalf("normalize %s = %+v", code, normalized)
			}
		})
	}

	ordinary := normalizeChangeStatuses([]FileStatus{
		{Status: "M", Path: "mixed.txt", Staged: true},
		{Status: "M", Path: "mixed.txt", Staged: false},
	})
	if len(ordinary) != 2 || ordinary[0].conflictCode != "" || ordinary[1].conflictCode != "" || !ordinary[0].status.Staged || ordinary[1].status.Staged {
		t.Fatalf("ordinary mixed path was normalized as a conflict: %+v", ordinary)
	}
}

func TestPreferredConflictEntryUsesOursThenTheirsThenBase(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stages []int
		want   int
	}{
		{name: "ours", stages: []int{1, 3, 2}, want: 2},
		{name: "theirs", stages: []int{1, 3}, want: 3},
		{name: "base", stages: []int{1}, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := make([]IndexEntry, len(tc.stages))
			for i, stage := range tc.stages {
				entries[i].Stage = stage
			}
			entry, ok := preferredConflictEntry(entries)
			if !ok || entry.Stage != tc.want {
				t.Fatalf("preferred stage for %v = %d, %v; want %d", tc.stages, entry.Stage, ok, tc.want)
			}
		})
	}
}

func TestToChangesCollapsesPorcelainConflictPairs(t *testing.T) {
	for _, code := range []string{"DD", "AU", "UD", "UA", "DU", "AA", "UU"} {
		changes := ToChanges([]FileStatus{{Path: "conflict", Status: code[:1], Staged: true}, {Path: "conflict", Status: code[1:]}})
		if len(changes) != 1 || changes[0].Kind != scm.Conflicted || changes[0].Detail != code || changes[0].Group != scm.WorkingTree {
			t.Fatalf("%s: %+v", code, changes)
		}
	}
}
