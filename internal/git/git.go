package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type FileStatus struct {
	Status  string
	Path    string
	OldPath string
	Staged  bool
}

func RepoRoot(dir string) string {
	return RepoRootContext(context.Background(), dir)
}

func RepoRootContext(ctx context.Context, dir string) string {
	root, _ := RepoRootWithErrorContext(ctx, dir)
	return root
}

// RepoRootWithErrorContext preserves discovery failures for callers that must
// retain a previously verified repository identity and retry.
func RepoRootWithErrorContext(ctx context.Context, dir string) (string, error) {
	cmd := gitCommandContext(ctx, "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", err
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", fmt.Errorf("git returned an empty repository root")
	}
	return root, nil
}

func IsRepo(dir string) bool {
	return IsRepoContext(context.Background(), dir)
}

func IsRepoContext(ctx context.Context, dir string) bool {
	cmd := gitCommandContext(ctx, "-C", dir, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func StatusFiles(dir string) ([]FileStatus, error) {
	return StatusFilesContext(context.Background(), dir)
}

func StatusFilesContext(ctx context.Context, dir string) ([]FileStatus, error) {
	cmd := gitCommandContext(ctx, "-C", dir, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseStatusPorcelainZ(out), nil
}

func parseStatusPorcelainZ(out []byte) []FileStatus {
	var files []FileStatus
	for len(out) > 0 {
		end := bytes.IndexByte(out, 0)
		if end < 0 {
			break
		}
		record := out[:end]
		out = out[end+1:]
		if len(record) < 3 || record[2] != ' ' {
			continue
		}
		x := record[0]
		y := record[1]
		path := string(record[3:])

		var oldPath string
		renameOrCopy := x == 'R' || x == 'C' || y == 'R' || y == 'C'
		if renameOrCopy {
			oldEnd := bytes.IndexByte(out, 0)
			if oldEnd < 0 {
				break
			}
			oldPath = string(out[:oldEnd])
			out = out[oldEnd+1:]
		}
		if path == "" || renameOrCopy && oldPath == "" {
			continue
		}

		if x != ' ' && x != '?' {
			files = append(files, FileStatus{Status: string(x), Path: path, OldPath: oldPath, Staged: true})
		}
		if y != ' ' {
			st := string(y)
			if x == '?' && y == '?' {
				st = "?"
			}
			files = append(files, FileStatus{Status: st, Path: path, OldPath: oldPath, Staged: false})
		}
	}
	return files
}

// Stage, Unstage and Discard take any number of paths so bulk actions spawn one
// git process instead of one per file.
func Stage(dir string, paths ...string) error {
	return runPaths(dir, []string{"add", "--"}, paths)
}

func Unstage(dir string, paths ...string) error {
	return runPaths(dir, []string{"reset", "HEAD", "--"}, paths)
}

func Discard(dir string, paths ...string) error {
	return runPaths(dir, []string{"checkout", "--"}, paths)
}

func DiscardUntracked(dir string, paths ...string) error {
	if len(paths) == 0 {
		return nil
	}
	args := []string{"-f"}
	for _, p := range paths {
		args = append(args, filepath.Join(dir, p))
	}
	cmd := exec.Command("rm", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runPaths(dir string, gitArgs, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"-C", dir}, gitArgs...)
	args = append(args, paths...)
	cmd := exec.Command("git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func Commit(dir, message string) error {
	cmd := exec.Command("git", "-C", dir, "commit", "-m", message)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func Pull(dir string) error {
	cmd := exec.Command("git", "-C", dir, "pull")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func hasUpstream(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	return cmd.Run() == nil
}

func Push(dir string) error {
	args := []string{"-C", dir, "push"}
	if !hasUpstream(dir) {
		branch := BranchName(dir)
		if branch != "" {
			args = []string{"-C", dir, "push", "--set-upstream", "origin", branch}
		}
	}
	cmd := exec.Command("git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func BranchName(dir string) string {
	name, _ := BranchNameContext(context.Background(), dir)
	return name
}

func BranchNameContext(ctx context.Context, dir string) (string, error) {
	cmd := gitCommandContext(ctx, "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		revision, revisionErr := RevisionIdentityContext(ctx, dir)
		if revisionErr != nil || revision != "" {
			return "", err
		}
		out, err = gitCommandContext(ctx, "-C", dir, "symbolic-ref", "--short", "-q", "HEAD").Output()
		if err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "", err
		}
	}
	return strings.TrimSpace(string(out)), nil
}

type LogEntry struct {
	// Hash is presentation-only; Ref is the stable full object identity.
	Hash    string
	Ref     string
	Message string
}

func Log(dir string, n int) []LogEntry {
	entries, _ := LogWithError(dir, n)
	return entries
}

func LogWithError(dir string, n int) ([]LogEntry, error) {
	return LogWithErrorContext(context.Background(), dir, n)
}

func LogWithErrorContext(ctx context.Context, dir string, n int) ([]LogEntry, error) {
	revision, err := RevisionIdentityContext(ctx, dir)
	if err != nil {
		return nil, err
	}
	if revision == "" {
		return nil, nil
	}
	cmd := gitCommandContext(ctx, "-C", dir, "log", fmt.Sprintf("-%d", n), "--pretty=format:%H %h %s")
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	var entries []LogEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		ref, rest, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		hash, msg, _ := strings.Cut(rest, " ")
		entries = append(entries, LogEntry{Hash: hash, Ref: ref, Message: msg})
	}
	return entries, nil
}

func CommitAuthoredAt(dir, ref string) (time.Time, error) {
	return CommitAuthoredAtContext(context.Background(), dir, ref)
}

func CommitAuthoredAtContext(ctx context.Context, dir, ref string) (time.Time, error) {
	out, err := gitCommandContext(ctx, "-C", dir, "show", "-s", "--format=%aI", ref).Output()
	if err != nil {
		if ctx.Err() != nil {
			return time.Time{}, ctx.Err()
		}
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, strings.TrimSpace(string(out)))
}

func CommitFiles(dir, hash string) ([]FileStatus, error) {
	return CommitFilesContext(context.Background(), dir, hash)
}

func CommitFilesContext(ctx context.Context, dir, hash string) ([]FileStatus, error) {
	out, err := gitCommandContext(ctx, "-C", dir, "show", "--name-status",
		"--diff-merges=first-parent", "-M", "--format=", "-z", hash).Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	return parseNameStatusZ(out), nil
}

func parseNameStatusZ(out []byte) []FileStatus {
	fields := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	var files []FileStatus
	for i := 0; i < len(fields); {
		code := fields[i]
		if code == "" {
			i++
			continue
		}
		status := code[:1]
		if status == "R" || status == "C" {
			if i+2 >= len(fields) {
				break
			}
			files = append(files, FileStatus{Status: status, OldPath: fields[i+1], Path: fields[i+2]})
			i += 3
			continue
		}
		if i+1 >= len(fields) {
			break
		}
		files = append(files, FileStatus{Status: status, Path: fields[i+1]})
		i += 2
	}
	return files
}

func CommitFileDiff(dir, hash string, file FileStatus) (string, error) {
	return CommitFileDiffContext(context.Background(), dir, hash, file)
}

func CommitFileDiffContext(ctx context.Context, dir, hash string, file FileStatus) (string, error) {
	args := []string{"-C", dir, "--literal-pathspecs", "show", "--diff-merges=first-parent", "-M", "--format=", hash, "--"}
	if file.OldPath != "" {
		args = append(args, file.OldPath)
	}
	args = append(args, file.Path)
	out, err := gitCommandContext(ctx, args...).Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", err
	}
	return string(out), nil
}

func CommitMessage(dir, hash string) (string, error) {
	return CommitMessageContext(context.Background(), dir, hash)
}

func CommitMessageContext(ctx context.Context, dir, hash string) (string, error) {
	out, err := gitCommandContext(ctx, "-C", dir, "show", "-s", "--format=%B", hash).Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", err
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

type BlameInfo struct {
	Author string
	Time   time.Time
}

func BlameLine(dir, file string, line int) *BlameInfo {
	lineStr := fmt.Sprintf("%d,%d", line, line)
	cmd := exec.Command("git", "-C", dir, "blame", "-L", lineStr,
		"--porcelain", "--", file)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	info := &BlameInfo{}
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, "author ") {
			info.Author = strings.TrimPrefix(l, "author ")
		} else if strings.HasPrefix(l, "author-time ") {
			ts, err := strconv.ParseInt(strings.TrimPrefix(l, "author-time "), 10, 64)
			if err == nil {
				info.Time = time.Unix(ts, 0)
			}
		}
	}

	if info.Author == "" {
		return nil
	}
	// Uncommitted changes
	if strings.HasPrefix(info.Author, "Not Committed") {
		return nil
	}
	return info
}

func FormatRelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 30*24*time.Hour:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	case d < 365*24*time.Hour:
		months := int(d.Hours() / 24 / 30)
		if months <= 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := int(d.Hours() / 24 / 365)
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

func HeadSHA(dir string) string {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func RevisionIdentity(dir string) string {
	identity, _ := RevisionIdentityContext(context.Background(), dir)
	return identity
}

// RevisionIdentityContext returns empty success only when Git verifies that
// HEAD names an unborn branch whose ref does not yet exist.
func RevisionIdentityContext(ctx context.Context, dir string) (string, error) {
	out, err := gitCommandContext(ctx, "-C", dir, "rev-parse", "--verify", "HEAD").Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	unborn, verifyErr := verifyUnbornHEADContext(ctx, dir)
	if verifyErr != nil {
		return "", errors.Join(err, fmt.Errorf("verify unborn HEAD: %w", verifyErr))
	}
	if unborn {
		return "", nil
	}
	return "", err
}

func verifyUnbornHEADContext(ctx context.Context, dir string) (bool, error) {
	out, err := gitCommandContext(ctx, "-C", dir, "symbolic-ref", "-q", "HEAD").Output()
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		return false, err
	}
	ref := strings.TrimSpace(string(out))
	if ref == "" {
		return false, fmt.Errorf("symbolic HEAD has no ref")
	}
	err = gitCommandContext(ctx, "-C", dir, "show-ref", "--verify", "--quiet", ref).Run()
	if err == nil {
		return false, nil
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return true, nil
	}
	return false, err
}

func RemoteURL(dir string) string {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func Permalink(dir, filePath string, line int) string {
	remote := RemoteURL(dir)
	if remote == "" {
		return ""
	}
	sha := HeadSHA(dir)
	if sha == "" {
		return ""
	}

	baseURL := remoteToHTTPS(remote)
	if baseURL == "" {
		return ""
	}

	relPath, err := filepath.Rel(dir, filePath)
	if err != nil {
		return ""
	}
	relPath = filepath.ToSlash(relPath)

	return fmt.Sprintf("%s/blob/%s/%s#L%d", baseURL, sha, relPath, line+1)
}

func remoteToHTTPS(remote string) string {
	remote = strings.TrimSpace(remote)
	if strings.HasPrefix(remote, "git@") {
		remote = strings.TrimPrefix(remote, "git@")
		remote = strings.Replace(remote, ":", "/", 1)
		remote = "https://" + remote
	}
	remote = strings.TrimSuffix(remote, ".git")
	return remote
}

func IgnoredFiles(dir string, paths []string) map[string]bool {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"-C", dir, "check-ignore"}, paths...)
	cmd := exec.Command("git", args...)
	out, _ := cmd.Output()
	result := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			result[line] = true
		}
	}
	return result
}

func ShowFile(dir, path, ref string) (string, error) {
	return ShowFileContext(context.Background(), dir, path, ref)
}

func ShowFileContext(ctx context.Context, dir, path, ref string) (string, error) {
	spec := ref + ":" + path
	cmd := gitCommandContext(ctx, "-C", dir, "show", spec)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", err
	}
	return string(out), nil
}

func DiffFile(dir, path string) (string, error) {
	absPath := filepath.Join(dir, path)
	cmd := exec.Command("git", "-C", dir, "diff", "--", absPath)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) == 0 {
			return string(out), nil
		}
		return "", err
	}
	if len(out) == 0 {
		cmd = exec.Command("git", "-C", dir, "diff", "--cached", "--", absPath)
		out, err = cmd.Output()
		if err != nil {
			return "", err
		}
	}
	if len(out) == 0 {
		cmd = exec.Command("git", "-C", dir, "diff", "HEAD", "--", absPath)
		out, err = cmd.Output()
		if err != nil {
			return "", err
		}
	}
	return string(out), nil
}

func DiffRename(dir, oldPath, newPath string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "diff", "HEAD", "--", filepath.Join(dir, oldPath), filepath.Join(dir, newPath))
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) == 0 {
			return string(out), nil
		}
		return "", err
	}
	if len(out) == 0 {
		cmd = exec.Command("git", "-C", dir, "diff", "--cached", "--", filepath.Join(dir, oldPath), filepath.Join(dir, newPath))
		out, err = cmd.Output()
		if err != nil {
			return "", err
		}
	}
	return string(out), nil
}
