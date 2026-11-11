package scm

import (
	"context"
	"time"
)

// RevisionReader optionally reads immutable revisions. Ref values are provider
// identities, and paths are literal repository-relative paths. Content is read
// separately so listing revisions and patches does not eagerly load every file.
type RevisionReader interface {
	ReadRevision(context.Context, string) (Revision, error)
	RevisionChanges(context.Context, string) ([]Change, error)
	ReadRevisionPatch(context.Context, string, Change) (string, error)
	ReadRevisionContent(ctx context.Context, ref, path, oldPath string) (RevisionContent, error)
}

type Revision struct {
	Message               string
	AuthoredAt            time.Time
	AuthoredAtUnavailable bool
}

type RevisionContent struct{ Before, After FileContent }
