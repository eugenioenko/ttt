package git

import "github.com/eugenioenko/ttt/internal/scm"

var changeKinds = map[string]scm.ChangeKind{
	"M": scm.Modified, "A": scm.Added, "D": scm.Deleted,
	"R": scm.Renamed, "C": scm.Copied, "?": scm.Untracked,
	"T": scm.TypeChanged, "U": scm.Conflicted,
	"DD": scm.Conflicted, "AU": scm.Conflicted, "UD": scm.Conflicted,
	"UA": scm.Conflicted, "DU": scm.Conflicted, "AA": scm.Conflicted, "UU": scm.Conflicted,
}

func ToChange(file FileStatus) scm.Change {
	kind, ok := changeKinds[file.Status]
	if !ok {
		kind = scm.Unknown
	}
	change := scm.Change{Path: file.Path, OldPath: file.OldPath, Kind: kind, Group: scm.WorkingTree}
	if file.Staged {
		change.Group = scm.Staged
	}
	if kind == scm.Conflicted || kind == scm.Unknown {
		change.Detail = file.Status
	}
	return change
}

// FromChange is a compatibility adapter for Git consumers not yet migrated.
func FromChange(change scm.Change) FileStatus {
	status := ""
	switch change.Kind {
	case scm.Modified:
		status = "M"
	case scm.Added:
		status = "A"
	case scm.Deleted:
		status = "D"
	case scm.Renamed:
		status = "R"
	case scm.Copied:
		status = "C"
	case scm.Untracked:
		status = "?"
	case scm.TypeChanged:
		status = "T"
	case scm.Conflicted:
		status = "U"
		if changeKinds[change.Detail] == scm.Conflicted {
			status = change.Detail
		}
	case scm.Unknown:
		status = change.Detail
	}
	return FileStatus{Path: change.Path, OldPath: change.OldPath, Status: status, Staged: change.Group == scm.Staged}
}

func ToChanges(files []FileStatus) []scm.Change {
	if files == nil {
		return nil
	}
	changes := make([]scm.Change, 0, len(files))
	for _, item := range normalizeChangeStatuses(files) {
		change := ToChange(item.status)
		if item.conflictCode != "" {
			change.Detail = item.conflictCode
		}
		changes = append(changes, change)
	}
	return changes
}

func FromChanges(changes []scm.Change) []FileStatus {
	if changes == nil {
		return nil
	}
	files := make([]FileStatus, 0, len(changes))
	for _, change := range changes {
		files = append(files, FromChange(change))
	}
	return files
}
