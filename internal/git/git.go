package git

import (
	"context"
	"fmt"
	"os"
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
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func IsRepo(dir string) bool {
	return IsRepoContext(context.Background(), dir)
}

func IsRepoContext(ctx context.Context, dir string) bool {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

func StatusFiles(dir string) ([]FileStatus, error) {
	return StatusFilesContext(context.Background(), dir)
}

func StatusFilesContext(ctx context.Context, dir string) ([]FileStatus, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "status", "--porcelain", "-u")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []FileStatus
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if len(line) < 4 {
			continue
		}
		x := line[0] // index (staged) status
		y := line[1] // worktree (unstaged) status
		path := strings.TrimSpace(line[3:])

		var oldPath string
		if parts := strings.SplitN(path, " -> ", 2); len(parts) == 2 {
			oldPath = parts[0]
			path = parts[1]
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
	return files, nil
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
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

type LogEntry struct {
	// Hash is the abbreviated hash, for display only. Its length depends on
	// core.abbrev and on how many objects the repo holds, so it is not stable
	// enough to key anything on — use Ref for that.
	Hash    string
	Ref     string
	Message string
}

func Log(dir string, n int) []LogEntry {
	entries, _ := LogWithError(dir, n)
	return entries
}

// LogWithError distinguishes a legitimate repository with no commits from a
// transient read failure, so callers can preserve the last good history.
func LogWithError(dir string, n int) ([]LogEntry, error) {
	return LogWithErrorContext(context.Background(), dir, n)
}

func LogWithErrorContext(ctx context.Context, dir string, n int) ([]LogEntry, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "log", fmt.Sprintf("-%d", n), "--pretty=format:%H %h %s")
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		head := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--verify", "HEAD")
		if head.Run() != nil && IsRepoContext(ctx, dir) {
			return nil, nil
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

// CommitFiles lists the files a commit touched.
//
// --diff-merges=first-parent is not cosmetic: `git show --name-status` prints
// nothing at all for a merge commit, so without it every merge in the log would
// expand to an empty file list.
//
// -M asks for rename detection only. Copies need -C, which finds only copies of
// files the same commit also modified, or --find-copies-harder, which compares
// against the whole tree and is far too slow to run from a panel.
// parseNameStatusZ still handles the copy record, because the format allows one
// and mis-reading it would shift every record after it.
func CommitFiles(dir, hash string) ([]FileStatus, error) {
	cmd := exec.Command("git", "-C", dir, "show", "--name-status",
		"--diff-merges=first-parent", "-M", "--format=", "-z", hash)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseNameStatusZ(out), nil
}

// parseNameStatusZ reads `--name-status -z` output. -z drops git's path
// quoting, so a record is exactly "status<NUL>path<NUL>" — or, for a rename or
// a copy, "Rxxx<NUL>old<NUL>new<NUL>", where xxx is a similarity score the rest
// of the app does not want.
func parseNameStatusZ(out []byte) []FileStatus {
	fields := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	var files []FileStatus
	for i := 0; i < len(fields); {
		code := fields[i]
		if code == "" {
			i++
			continue
		}
		st := code[:1]
		if st == "R" || st == "C" {
			// A truncated three-field record is dropped rather than read as a
			// two-field one, which would misalign every record after it.
			if i+2 >= len(fields) {
				break
			}
			files = append(files, FileStatus{Status: st, OldPath: fields[i+1], Path: fields[i+2]})
			i += 3
			continue
		}
		if i+1 >= len(fields) {
			break
		}
		files = append(files, FileStatus{Status: st, Path: fields[i+1]})
		i += 2
	}
	return files
}

// CommitFileDiff returns the diff a single commit applied to one file.
func CommitFileDiff(dir, hash string, f FileStatus) (string, error) {
	// -- ends option parsing, but pathspec magic still applies after it. Disable
	// that separately so legal names such as "*.txt" and ":(exclude)*" select
	// the one committed path the reader asked for.
	args := []string{"-C", dir, "--literal-pathspecs", "show", "--diff-merges=first-parent", "-M", "--format=", hash, "--"}
	if f.OldPath != "" {
		args = append(args, f.OldPath)
	}
	args = append(args, f.Path)
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// CommitMessage returns the complete message for one commit, including its
// subject and body. The trailing newline Git prints is presentation framing,
// not part of the message shown in the detail view.
func CommitMessage(dir, hash string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "show", "-s", "--format=%B", hash).Output()
	if err != nil {
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

// RevisionIdentity is a cheap fingerprint for the history surface. Including
// the branch name catches switching between two branches that currently point
// at the same commit, while the full object ID catches commits, pulls, resets,
// and detached-HEAD movement.
func RevisionIdentity(dir string) string {
	identity, _ := RevisionIdentityContext(context.Background(), dir)
	return identity
}

// RevisionIdentityContext returns a fingerprint that changes for either a new
// HEAD commit or a branch switch. An unborn repository is a known empty
// identity rather than a read failure.
func RevisionIdentityContext(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "HEAD", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		head := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--verify", "HEAD")
		if head.Run() != nil && IsRepoContext(ctx, dir) {
			return "", nil
		}
		return "", err
	}
	parts := strings.Fields(string(out))
	if len(parts) < 2 {
		return "", fmt.Errorf("unexpected revision identity output")
	}
	return parts[0] + "\x00" + parts[1], nil
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
	spec := ref + ":" + path
	cmd := exec.Command("git", "-C", dir, "show", spec)
	out, err := cmd.Output()
	if err != nil {
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

// DiffWorkingTreeFileContext returns the final working-tree state relative to HEAD,
// combining staged and unstaged edits into one document. Untracked files are
// compared with /dev/null so callers can render them as ordinary additions.
func DiffWorkingTreeFileContext(ctx context.Context, dir string, status FileStatus) (string, error) {
	path := filepath.Join(dir, status.Path)
	if status.Status == "?" {
		return diffFileFromEmptyContext(ctx, dir, path)
	}

	paths := []string{"--", path}
	if status.OldPath != "" && status.OldPath != status.Path {
		paths = []string{"--", filepath.Join(dir, status.OldPath), path}
	}
	args := append([]string{"-C", dir, "--literal-pathspecs", "diff", "HEAD"}, paths...)
	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err == nil {
		return string(out), nil
	}
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	// HEAD is absent in an unborn repository. Compare the final worktree file,
	// not merely its staged snapshot, with the empty tree.
	head := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--verify", "HEAD")
	if head.Run() != nil && IsRepoContext(ctx, dir) {
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			return "", nil
		} else if statErr != nil {
			return "", statErr
		}
		return diffFileFromEmptyContext(ctx, dir, path)
	}
	return "", err
}

func diffFileFromEmptyContext(ctx context.Context, dir, path string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "diff", "--no-index", "--", "/dev/null", path)
	out, err := cmd.Output()
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return string(out), nil
	}
	return string(out), err
}
