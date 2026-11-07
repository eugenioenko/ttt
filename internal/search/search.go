// Package search owns the ripgrep scans behind the editor's search and
// replace surfaces. The UI receives typed results and does not own the
// subprocess lifecycle itself.
package search

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
)

// Options configures a ripgrep scan.
type Options struct {
	CaseSensitive bool
	UseRegex      bool
	Include       string
	Exclude       string
	Query         string
}

// Match is one ripgrep match.
type Match struct {
	FilePath string
	LineNum  int
	ColStart int
	ColEnd   int
	LineText string
}

// RgMessage is one line of ripgrep's --json output.
type RgMessage struct {
	Type string      `json:"type"`
	Data RgMatchData `json:"data"`
}

// RgMatchData is the payload of an RgMessage.
type RgMatchData struct {
	Path       RgText       `json:"path"`
	Lines      RgText       `json:"lines"`
	LineNumber int          `json:"line_number"`
	Submatches []RgSubmatch `json:"submatches"`
}

// RgText is a text field of ripgrep's --json output.
type RgText struct {
	Text string `json:"text"`
}

// RgSubmatch is one match inside a line of ripgrep's --json output.
type RgSubmatch struct {
	Match RgText `json:"match"`
	Start int    `json:"start"`
	End   int    `json:"end"`
}

// maxPathBytes bounds the path text handed to one ripgrep invocation. Windows
// caps the whole command line at about 32K characters, so a long result list
// has to be scanned in batches.
const maxPathBytes = 8000

// BaseArgs builds the argument list shared by the interactive search and the
// uncapped re-scan Replace All runs. capped keeps the result-list limits; the
// re-scan drops them so a replacement covers every match, not just the listed
// ones.
func BaseArgs(opts Options, capped bool) []string {
	args := []string{"--json"}
	if capped {
		args = append(args, "--max-count=100", "--max-columns=1000")
	}
	if opts.CaseSensitive {
		args = append(args, "--case-sensitive")
	} else {
		args = append(args, "--smart-case")
	}
	if !opts.UseRegex {
		args = append(args, "--fixed-strings")
	}
	for _, g := range strings.Split(opts.Include, ",") {
		g = strings.TrimSpace(g)
		if g != "" {
			args = append(args, "--glob", g)
		}
	}
	for _, g := range strings.Split(opts.Exclude, ",") {
		g = strings.TrimSpace(g)
		if g != "" {
			args = append(args, "--glob", "!"+g)
		}
	}
	return append(args, "--", opts.Query)
}

// FindAllMatches runs ripgrep over paths with the result-list caps removed and
// returns every match, keyed by absolute path. Replace All uses it so a file
// with more than 100 matching lines, or a match past column 1000, is not left
// half-replaced. A successful scan omits paths that no longer match.
func FindAllMatches(ctx context.Context, opts Options, paths []string) (map[string][]Match, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if opts.Query == "" || len(paths) == 0 {
		return nil, nil
	}
	if _, err := exec.LookPath("rg"); err != nil {
		return nil, err
	}

	matches := map[string][]Match{}
	for _, batch := range batchPaths(paths) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		batchMatches, err := scanBatch(ctx, opts, batch)
		if err != nil {
			return nil, err
		}
		for path, ms := range batchMatches {
			matches[path] = append(matches[path], ms...)
		}
	}
	return matches, nil
}

// batchPaths splits paths into groups whose text stays inside maxPathBytes, so
// no single ripgrep invocation exceeds the platform command-line limit.
func batchPaths(paths []string) [][]string {
	var batches [][]string
	current := []string{}
	size := 0
	for _, p := range paths {
		if size+len(p) > maxPathBytes && len(current) > 0 {
			batches = append(batches, current)
			current = []string{}
			size = 0
		}
		current = append(current, p)
		size += len(p)
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

// scanBatch runs one ripgrep invocation over a batch of paths.
func scanBatch(ctx context.Context, opts Options, paths []string) (map[string][]Match, error) {
	// Absolute inputs make rg echo absolute paths, so the keys below do not
	// depend on the process working directory.
	args := BaseArgs(opts, false)
	for _, p := range paths {
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		args = append(args, p)
	}

	cmd := exec.CommandContext(ctx, "rg", args...)
	return scanCommand(ctx, cmd)
}

func scanCommand(ctx context.Context, cmd *exec.Cmd) (map[string][]Match, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	matches := map[string][]Match{}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	for scanner.Scan() {
		var msg RgMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		if msg.Type != "match" {
			continue
		}

		absPath, err := filepath.Abs(msg.Data.Path.Text)
		if err != nil {
			continue
		}
		absPath = filepath.Clean(absPath)
		lineText := strings.TrimRight(msg.Data.Lines.Text, "\n\r")
		for _, sub := range msg.Data.Submatches {
			matches[absPath] = append(matches[absPath], Match{
				FilePath: absPath,
				LineNum:  msg.Data.LineNumber,
				ColStart: sub.Start,
				ColEnd:   sub.End,
				LineText: lineText,
			})
		}
	}
	scanErr := scanner.Err()
	if scanErr != nil {
		// rg can still be blocked writing to the pipe, so stop it before
		// waiting; otherwise cmd.Wait blocks forever.
		cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if scanErr != nil {
		return nil, scanErr
	}
	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) && exitErr.ExitCode() == 1 {
		return matches, nil
	}
	return matches, waitErr
}
