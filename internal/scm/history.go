package scm

import "context"

// HistoryReader optionally supplies bounded pages from an immutable snapshot.
// An empty Anchor requests the current snapshot; later pages must retain it.
type HistoryReader interface {
	ReadHistory(context.Context, HistoryRequest) (HistoryPage, error)
}

type HistoryRequest struct {
	Anchor string
	Offset int
	Limit  int
}

type HistoryPage struct {
	Anchor  string
	Entries []HistoryEntry
	HasMore bool
}

type HistoryEntry struct {
	// Ref is opaque provider identity; Label is presentation-only.
	Ref     string
	Label   string
	Message string
}
