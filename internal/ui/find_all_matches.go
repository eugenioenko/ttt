package ui

import (
	"bufio"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
)

// rgBaseArgs builds the argument list shared by the interactive search and the
// uncapped re-scan Replace All runs. capped keeps the result-list limits; the
// re-scan drops them so a replacement covers every match, not just the listed
// ones.
func rgBaseArgs(opts SearchOptions, include, exclude, query string, capped bool) []string {
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
	for _, g := range strings.Split(include, ",") {
		g = strings.TrimSpace(g)
		if g != "" {
			args = append(args, "--glob", g)
		}
	}
	for _, g := range strings.Split(exclude, ",") {
		g = strings.TrimSpace(g)
		if g != "" {
			args = append(args, "--glob", "!"+g)
		}
	}
	return append(args, "--", query)
}

// FindAllMatches runs ripgrep over paths with the result-list caps removed and
// returns every match, keyed by absolute path. Replace All uses it so a file
// with more than 100 matching lines, or a match past column 1000, is not left
// half-replaced. A missing rg, a run failure, or a path with no matches yields
// an empty map; callers fall back to the matches the panel already holds.
func FindAllMatches(ctx context.Context, opts SearchOptions, include, exclude, query string, paths []string) (map[string][]SearchMatch, error) {
	if query == "" || len(paths) == 0 {
		return nil, nil
	}
	if _, err := exec.LookPath("rg"); err != nil {
		return nil, err
	}

	// Absolute inputs make rg echo absolute paths, so the keys below do not
	// depend on the process working directory.
	args := rgBaseArgs(opts, include, exclude, query, false)
	for _, p := range paths {
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		args = append(args, p)
	}

	cmd := exec.CommandContext(ctx, "rg", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	matches := map[string][]SearchMatch{}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	for scanner.Scan() {
		var msg rgMessage
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
			matches[absPath] = append(matches[absPath], SearchMatch{
				FilePath: absPath,
				LineNum:  msg.Data.LineNumber,
				ColStart: sub.Start,
				ColEnd:   sub.End,
				LineText: lineText,
			})
		}
	}
	scanErr := scanner.Err()
	waitErr := cmd.Wait()
	if scanErr != nil {
		return nil, scanErr
	}
	return matches, waitErr
}
