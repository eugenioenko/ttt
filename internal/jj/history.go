package jj

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/scm"
)

var _ scm.HistoryReader = repository{}
var _ scm.RevisionReader = repository{}

var commitIDPattern = regexp.MustCompile(`^[0-9a-f]+$`)

// Revision refs reach revsets, so only commit IDs this provider produced are
// accepted; anything else could be evaluated as a revset expression.
func validCommitID(ref string) error {
	if !commitIDPattern.MatchString(ref) {
		return fmt.Errorf("jj: invalid commit id %q", ref)
	}
	return nil
}

// ReadHistory lists the working-copy commit's ancestors. The working-copy
// commit itself is the Changes list, so history starts at its parents.
func (r repository) ReadHistory(ctx context.Context, request scm.HistoryRequest) (scm.HistoryPage, error) {
	anchor := request.Anchor
	if anchor == "" {
		out, err := r.read(ctx, "log", "-r", "@", "--no-graph", "-T", `commit_id ++ "\0"`)
		if err != nil {
			return scm.HistoryPage{}, err
		}
		if fields := splitFields(out); len(fields) > 0 {
			anchor = fields[0]
		}
	}
	if err := validCommitID(anchor); err != nil {
		return scm.HistoryPage{}, err
	}
	limit := request.Offset + request.Limit + 1
	out, err := r.read(ctx, "log", "--no-graph", "-r", "::("+anchor+"-) ~ root()", "-n", fmt.Sprint(limit),
		"-T", `commit_id ++ "\0" ++ change_id.short(8) ++ "\0" ++ description.first_line() ++ "\0"`)
	if err != nil {
		return scm.HistoryPage{}, err
	}
	fields := splitFields(out)
	if len(fields)%3 != 0 {
		return scm.HistoryPage{}, fmt.Errorf("jj: unexpected history output")
	}
	page := scm.HistoryPage{Anchor: anchor}
	for i := request.Offset * 3; i+2 < len(fields); i += 3 {
		if len(page.Entries) == request.Limit {
			page.HasMore = true
			break
		}
		page.Entries = append(page.Entries, scm.HistoryEntry{Ref: fields[i], Label: fields[i+1], Message: fields[i+2]})
	}
	return page, ctx.Err()
}

func (r repository) ReadRevision(ctx context.Context, ref string) (scm.Revision, error) {
	if err := validCommitID(ref); err != nil {
		return scm.Revision{}, err
	}
	out, err := r.read(ctx, "log", "-r", ref, "--no-graph",
		"-T", `description ++ "\0" ++ author.timestamp().format("%Y-%m-%dT%H:%M:%S%:z") ++ "\0"`)
	if err != nil {
		return scm.Revision{}, err
	}
	fields := splitFields(out)
	if len(fields) < 2 {
		return scm.Revision{}, fmt.Errorf("jj: unexpected revision output")
	}
	revision := scm.Revision{Message: strings.TrimRight(fields[0], "\n")}
	revision.AuthoredAt, err = time.Parse(time.RFC3339, fields[1])
	revision.AuthoredAtUnavailable = err != nil
	return revision, nil
}

func (r repository) RevisionChanges(ctx context.Context, ref string) ([]scm.Change, error) {
	if err := validCommitID(ref); err != nil {
		return nil, err
	}
	out, err := r.read(ctx, "log", "-r", ref, "--no-graph", "-T", changesTemplate)
	if err != nil {
		return nil, err
	}
	return parseChanges(out)
}

func (r repository) ReadRevisionPatch(ctx context.Context, ref string, change scm.Change) (string, error) {
	if err := validCommitID(ref); err != nil {
		return "", err
	}
	parents, err := r.parents(ctx, ref)
	if err != nil {
		return "", err
	}
	parent := ""
	if len(parents) > 0 {
		parent = parents[0]
	}
	out, err := r.read(ctx, append(diffRange(parent, ref), "--", fileset(change.OldPath, change.Path))...)
	return string(out), err
}

func (r repository) ReadRevisionContent(ctx context.Context, ref, path, oldPath string) (scm.RevisionContent, error) {
	if err := validCommitID(ref); err != nil {
		return scm.RevisionContent{}, err
	}
	parents, err := r.parents(ctx, ref)
	if err != nil {
		return scm.RevisionContent{}, err
	}
	parent := ""
	if len(parents) > 0 {
		parent = parents[0]
	}
	if oldPath == "" {
		oldPath = path
	}
	before, err := r.fileAt(ctx, parent, oldPath)
	if err != nil {
		return scm.RevisionContent{}, err
	}
	after, err := r.fileAt(ctx, ref, path)
	if err != nil {
		return scm.RevisionContent{}, err
	}
	if !before.Exists && !after.Exists {
		return scm.RevisionContent{}, fmt.Errorf("revision path %q does not exist", path)
	}
	return scm.RevisionContent{Before: before, After: after}, ctx.Err()
}
