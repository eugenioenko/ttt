// Package jj adapts the Jujutsu CLI to the scm provider contracts.
package jj

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/scm"
)

const binary = "jj"

var lookPath = exec.LookPath

type Provider struct{}

type repository struct{ root string }

var _ scm.Provider = Provider{}
var _ scm.Repository = repository{}

// IsRepository reports whether repo was discovered by this provider.
func IsRepository(repo scm.Repository) bool {
	_, ok := repo.(repository)
	return ok
}

// Discover claims the nearest enclosing .jj workspace. A nearer .git directory
// belongs to a nested Git repository, and a missing jj binary leaves the
// workspace to other providers, so both report ErrNotRepository.
func (Provider) Discover(ctx context.Context, dir string) (scm.Repository, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	// Walk the real path: a symlinked directory's lexical ancestors are not the
	// ancestors of the workspace it points into.
	resolved, err := filepath.EvalSymlinks(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errors.Join(scm.ErrNotRepository, err)
	}
	if err != nil {
		return nil, err
	}
	for current := resolved; ; {
		found, err := isDir(filepath.Join(current, ".jj"))
		if err != nil {
			return nil, err
		}
		if found {
			if _, err := lookPath(binary); err != nil {
				return nil, errors.Join(scm.ErrNotRepository, err)
			}
			return repository{root: current}, nil
		}
		if found, err = exists(filepath.Join(current, ".git")); err != nil {
			return nil, err
		} else if found {
			return nil, scm.ErrNotRepository
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil, scm.ErrNotRepository
		}
		current = parent
	}
}

// isDir and exists report only absence as false; permission and I/O failures
// are errors so discovery never silently skips an unreadable workspace.
func isDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil && info.IsDir(), err
}

func exists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func (r repository) Root() string { return r.root }

func (r repository) Identity(ctx context.Context) (scm.Identity, error) {
	out, err := r.read(ctx, "log", "-r", "@", "--no-graph", "-T", `change_id.short(8) ++ "\0" ++ local_bookmarks.map(|b| b.name()).join(" ") ++ "\0"`)
	if err != nil {
		return scm.Identity{Root: r.root}, err
	}
	fields := splitFields(out)
	if len(fields) < 2 {
		return scm.Identity{Root: r.root}, fmt.Errorf("jj: unexpected identity output %q", out)
	}
	label := fields[1]
	if label == "" {
		label = fields[0]
	}
	return scm.Identity{Root: r.root, Label: label}, nil
}

const changesTemplate = `diff.files().map(|f| "F\0" ++ f.status() ++ "\0" ++ f.source().path() ++ "\0" ++ f.target().path() ++ "\0").join("") ++ self.conflicted_files().map(|e| "C\0" ++ e.path() ++ "\0").join("")`

// Changes snapshots the working copy, as every jj command does by default, so
// the result reflects files edited since the last jj invocation.
func (r repository) Changes(ctx context.Context) ([]scm.Change, error) {
	out, err := r.run(ctx, "log", "-r", "@", "--no-graph", "-T", changesTemplate)
	if err != nil {
		return nil, err
	}
	return parseChanges(out)
}

// Revision identifies the working-copy parents, which are the baseline every
// gutter and working-copy diff compares against.
func (r repository) Revision(ctx context.Context) (string, error) {
	parents, err := r.parents(ctx, "@")
	return strings.Join(parents, ","), err
}

func (r repository) parents(ctx context.Context, rev string) ([]string, error) {
	out, err := r.read(ctx, "log", "-r", rev, "--no-graph", "-T", `parents.map(|c| c.commit_id() ++ "\0").join("")`)
	if err != nil {
		return nil, err
	}
	return splitFields(out), nil
}

var changeKinds = map[string]scm.ChangeKind{
	"modified": scm.Modified,
	"added":    scm.Added,
	"removed":  scm.Deleted,
	"renamed":  scm.Renamed,
	"copied":   scm.Copied,
}

func parseChanges(out []byte) ([]scm.Change, error) {
	fields := splitFields(out)
	changes := []scm.Change{}
	index := map[string]int{}
	for i := 0; i < len(fields); {
		switch fields[i] {
		case "F":
			if i+3 >= len(fields) {
				return nil, fmt.Errorf("jj: truncated change record")
			}
			status, source, target := fields[i+1], filepath.ToSlash(fields[i+2]), filepath.ToSlash(fields[i+3])
			kind, ok := changeKinds[status]
			change := scm.Change{Path: target, Kind: kind, Group: scm.WorkingTree}
			if !ok {
				change.Kind, change.Detail = scm.Unknown, status
			}
			if (kind == scm.Renamed || kind == scm.Copied) && source != target {
				change.OldPath = source
			}
			index[target] = len(changes)
			changes = append(changes, change)
			i += 4
		case "C":
			if i+1 >= len(fields) {
				return nil, fmt.Errorf("jj: truncated conflict record")
			}
			path := filepath.ToSlash(fields[i+1])
			conflict := scm.Change{Path: path, Kind: scm.Conflicted, Group: scm.WorkingTree, Detail: "conflict"}
			if at, ok := index[path]; ok {
				changes[at] = conflict
			} else {
				changes = append(changes, conflict)
			}
			i += 2
		default:
			return nil, fmt.Errorf("jj: unexpected change record %q", fields[i])
		}
	}
	return changes, nil
}

// splitFields splits NUL-terminated template output. Paths may contain any
// other byte, so templates terminate every field with NUL.
func splitFields(out []byte) []string {
	out = bytes.TrimSuffix(out, []byte{0})
	if len(out) == 0 {
		return nil
	}
	parts := bytes.Split(out, []byte{0})
	fields := make([]string, len(parts))
	for i, part := range parts {
		fields[i] = string(part)
	}
	return fields
}

var filesetEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`, "\x00", `\0`)

// fileset matches literal repository-relative paths; jj otherwise parses path
// arguments as fileset expressions relative to the current directory.
func fileset(paths ...string) string {
	parts := make([]string, 0, len(paths))
	for _, path := range paths {
		if path != "" {
			parts = append(parts, `root-file:"`+filesetEscaper.Replace(path)+`"`)
		}
	}
	return strings.Join(parts, " | ")
}

// read runs a query that must not snapshot the working copy.
func (r repository) read(ctx context.Context, args ...string) ([]byte, error) {
	return r.run(ctx, append([]string{"--ignore-working-copy"}, args...)...)
}

func (r repository) run(ctx context.Context, args ...string) ([]byte, error) {
	// Word wrapping would split parsed template output at spaces in paths.
	cmd := commandContext(ctx, append([]string{"--no-pager", "--color=never", "--config=ui.log-word-wrap=false", "-R", r.root}, args...)...)
	cmd.Dir = r.root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, &CommandError{Args: args, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return out, nil
}

type CommandError struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *CommandError) Error() string {
	name := "jj"
	for _, arg := range e.Args {
		if !strings.HasPrefix(arg, "-") {
			name = "jj " + arg
			break
		}
	}
	if e.Stderr != "" {
		return name + ": " + e.Stderr
	}
	return name + ": " + e.Err.Error()
}

func (e *CommandError) Unwrap() error { return e.Err }
