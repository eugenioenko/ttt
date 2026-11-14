package scm

import (
	"context"
	"errors"
)

var ErrNotRepository = errors.New("not a source control repository")

type Provider interface {
	// A successful discovery returns a repository with an absolute root.
	// ErrNotRepository means no supported repository was found; other failures remain errors.
	Discover(context.Context, string) (Repository, error)
}

type Repository interface {
	Root() string
	Identity(context.Context) (Identity, error)
	Changes(context.Context) ([]Change, error)
	// Revision is an opaque baseline token for invalidation, not a Git ref.
	Revision(context.Context) (string, error)
}

// Staging is optional: repositories without an index need not implement it.
// Paths are literal, repository-relative paths; an empty list is a no-op.
type Staging interface {
	Stage(context.Context, []string) error
	Unstage(context.Context, []string) error
}

// Committer optionally records every working-copy change as a new revision
// with the given message. Providers with staging use their own commit path.
type Committer interface {
	Commit(context.Context, string) error
}

// Discarder optionally reverts literal repository-relative paths to their
// baseline, including removing added files and restoring renamed sources.
type Discarder interface {
	Discard(context.Context, []string) error
}

// BaselineReader optionally supplies the comparison content for working-copy
// edits. Providers choose the baseline; paths are literal and repository-relative.
// Read failures must be errors; Exists=false means the baseline is missing.
type BaselineReader interface {
	ReadBaseline(context.Context, string) (Baseline, error)
}

type Baseline struct {
	Content string
	Exists  bool
	Ignored bool
	// Conflicted content is a fallback for markers, never an editable diff base.
	Conflicted bool
}

// ChangeReader optionally reads a single change using its provider group and
// literal repository-relative paths. Missing endpoints are not read errors.
type ChangeReader interface {
	ReadChange(context.Context, Change) (ChangeDiff, error)
}

type FileContent struct {
	Content string
	Exists  bool
}

type ChangeDiff struct {
	Before, After FileContent
	// Patch is a unified diff, including metadata-only changes. Conflicted
	// results may omit it: the endpoints describe a provider-selected two-way
	// resolution view. Otherwise empty means no change; an empty file may exist.
	Patch          string
	Conflicted     bool
	ConflictDetail string
	// ConflictID opts into one aggregate resolution view for this path and ID,
	// even when the conflict is reported in multiple action groups.
	ConflictID string
	// ConflictSources are opaque identities of the available conflict sides.
	// Consumers use them for invalidation, never to select provider revisions.
	ConflictSources []string
}

type Identity struct {
	Root  string
	Label string
}

type ChangeGroup string

const (
	WorkingTree ChangeGroup = "working-tree"
	Staged      ChangeGroup = "staged"
)

type ChangeKind string

const (
	Unknown     ChangeKind = "unknown"
	Modified    ChangeKind = "modified"
	Added       ChangeKind = "added"
	Deleted     ChangeKind = "deleted"
	Renamed     ChangeKind = "renamed"
	Copied      ChangeKind = "copied"
	Untracked   ChangeKind = "untracked"
	TypeChanged ChangeKind = "type-changed"
	Conflicted  ChangeKind = "conflicted"
)

type Change struct {
	Path    string
	OldPath string
	Kind    ChangeKind
	Group   ChangeGroup
	// Detail preserves provider diagnostics for conflicts and unknown changes.
	// Consumers must use Kind for classification rather than parse this text.
	Detail string
}
