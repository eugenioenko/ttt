package scm

import "context"

// ConflictReader is optional. Unsupported file types or missing sides return
// errors.ErrUnsupported so callers can keep the provider's normal workflow.
type ConflictReader interface {
	ReadConflict(context.Context, string) (ConflictInputs, error)
}

type ConflictInputs struct {
	Base, Current, Incoming FileContent
	Identity                string
}
