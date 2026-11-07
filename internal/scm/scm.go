package scm

import "context"

type Provider interface {
	// A successful discovery returns a repository with an absolute root.
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
