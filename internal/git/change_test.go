package git

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/scm"
)

func TestChangeConversionPreservesGitStatusIdentity(t *testing.T) {
	cases := map[string]scm.ChangeKind{
		"M": scm.Modified, "A": scm.Added, "D": scm.Deleted, "R": scm.Renamed, "C": scm.Copied, "?": scm.Untracked, "T": scm.TypeChanged,
		"U": scm.Conflicted, "DD": scm.Conflicted, "AU": scm.Conflicted, "UD": scm.Conflicted, "UA": scm.Conflicted, "DU": scm.Conflicted, "AA": scm.Conflicted, "UU": scm.Conflicted, "X": scm.Unknown, "": scm.Unknown,
	}
	for code, kind := range cases {
		for _, staged := range []bool{false, true} {
			file := FileStatus{Status: code, Path: "新規/[literal].txt", OldPath: "old name.txt", Staged: staged}
			change := ToChange(file)
			if change.Kind != kind || (change.Group == scm.Staged) != staged {
				t.Fatalf("classification: %+v -> %+v", file, change)
			}
			if got := FromChange(change); got != file {
				t.Fatalf("lossy round trip: %+v -> %+v", file, got)
			}
		}
	}
}
