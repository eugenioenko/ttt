package git

import "testing"

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

func TestConflictCodeFromAvailableSides(t *testing.T) {
	for code, stages := range map[string][]int{"DD": {1}, "AU": {2}, "UA": {3}, "UD": {1, 2}, "DU": {1, 3}, "AA": {2, 3}, "UU": {1, 2, 3}} {
		entries := make([]IndexEntry, len(stages))
		for i, stage := range stages {
			entries[i].Stage = stage
		}
		if got := conflictCodeFromEntries(entries); got != code {
			t.Fatalf("sides %v: got %q, want %q", stages, got, code)
		}
	}
}
