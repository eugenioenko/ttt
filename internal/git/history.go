package git

import (
	"context"

	"github.com/eugenioenko/ttt/internal/scm"
)

var _ scm.HistoryReader = repository{}

func (r repository) ReadHistory(ctx context.Context, request scm.HistoryRequest) (scm.HistoryPage, error) {
	anchor := ObjectID(request.Anchor)
	if err := ctx.Err(); err != nil {
		return scm.HistoryPage{}, err
	}
	if anchor == "" {
		var err error
		anchor, err = RevisionObjectIDContext(ctx, r.root)
		if err != nil {
			return scm.HistoryPage{}, err
		}
	}
	page, err := LogPageContext(ctx, r.root, anchor, request.Offset, request.Limit)
	if err != nil {
		return scm.HistoryPage{}, err
	}
	result := scm.HistoryPage{Anchor: string(anchor), HasMore: page.HasMore, Entries: make([]scm.HistoryEntry, 0, len(page.Entries))}
	for _, entry := range page.Entries {
		result.Entries = append(result.Entries, scm.HistoryEntry{Ref: entry.Ref, Label: entry.Hash, Message: entry.Message})
	}
	return result, ctx.Err()
}
