package git

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/scm"
)

func (r repository) ReadConflict(ctx context.Context, path string) (scm.ConflictInputs, error) {
	entries, err := IndexEntriesContext(ctx, r.root, path)
	if err != nil {
		return scm.ConflictInputs{}, err
	}
	width, err := r.conflictMarkerSize(ctx, path)
	if err != nil {
		return scm.ConflictInputs{}, err
	}
	inputs := scm.ConflictInputs{MarkerSize: width, Identity: strconv.Itoa(width) + ":"}
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

func (r repository) conflictMarkerSize(ctx context.Context, path string) (int, error) {
	out, err := gitCommandContext(ctx, "-C", r.root, "check-attr", "-z", "merge", "conflict-marker-size", "--", path).Output()
	if err != nil {
		return 0, err
	}
	attrs := strings.Split(string(out), "\x00")
	if len(attrs) != 7 {
		return 0, errors.ErrUnsupported
	}
	driver := attrs[2]
	if driver == "unspecified" {
		out, err = gitCommandContext(ctx, "-C", r.root, "config", "--get", "merge.default").Output()
		var exit *exec.ExitError
		if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
			return 0, err
		}
		driver = strings.TrimSpace(string(out))
	}
	switch driver {
	case "", "set", "text":
	default:
		return 0, errors.ErrUnsupported
	}
	width, err := strconv.Atoi(attrs[5])
	if err != nil || width < 1 {
		width = 7
	}
	return width, nil
}
