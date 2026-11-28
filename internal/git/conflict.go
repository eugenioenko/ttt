package git

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/scm"
)

func (r repository) ReadConflict(ctx context.Context, path string) (scm.ConflictInputs, error) {
	entries, err := IndexEntriesContext(ctx, r.root, path)
	if err != nil {
		return scm.ConflictInputs{}, err
	}
	var inputs scm.ConflictInputs
	sides := [...]*scm.FileContent{nil, &inputs.Base, &inputs.Current, &inputs.Incoming}
	for _, entry := range entries {
		if entry.Stage < 1 || entry.Stage > 3 || (entry.Mode != "100644" && entry.Mode != "100755") {
			return scm.ConflictInputs{}, errors.ErrUnsupported
		}
		// Read the immutable object: the index can change between these reads.
		content, err := showString(ctx, r.root, entry.Object)
		if err != nil {
			return scm.ConflictInputs{}, err
		}
		if strings.IndexByte(content, 0) >= 0 || !utf8.ValidString(content) {
			return scm.ConflictInputs{}, errors.ErrUnsupported
		}
		*sides[entry.Stage] = scm.FileContent{Exists: true, Content: content}
		inputs.Identity += string(rune('0'+entry.Stage)) + ":" + entry.Mode + ":" + entry.Object + ";"
	}
	if !inputs.Current.Exists || !inputs.Incoming.Exists {
		return scm.ConflictInputs{}, errors.ErrUnsupported
	}
	return inputs, nil
}
