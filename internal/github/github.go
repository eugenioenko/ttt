package github

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type PRFile struct {
	Path   string
	Status string // A, M, D, R
}

type PRInfo struct {
	Owner   string
	Repo    string
	Number  int
	Title   string
	Body    string
	BaseSHA string
	HeadSHA string
	Files   []PRFile
}

func IsGHInstalled() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

func ParsePRURL(url string) (owner, repo string, number int, err error) {
	url = strings.TrimRight(url, "/")
	url = strings.SplitN(url, "?", 2)[0]
	url = strings.SplitN(url, "#", 2)[0]
	parts := strings.Split(url, "/")
	for i, p := range parts {
		if p == "pull" && i >= 2 && i+1 < len(parts) {
			owner = parts[i-2]
			repo = parts[i-1]
			n, e := strconv.Atoi(parts[i+1])
			if e != nil {
				return "", "", 0, fmt.Errorf("invalid PR number: %s", parts[i+1])
			}
			return owner, repo, n, nil
		}
	}
	return "", "", 0, fmt.Errorf("could not parse PR URL: %s", url)
}

func CloneRepo(repo, dest string) error {
	cmd := exec.Command("gh", "repo", "clone", repo, dest)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func FetchPRInfo(owner, repo string, number int) (*PRInfo, error) {
	repoArg := owner + "/" + repo
	numStr := strconv.Itoa(number)

	cmd := exec.Command("gh", "pr", "view", numStr,
		"--repo", repoArg,
		"--json", "title,body,number,files")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr view failed: %w", err)
	}
	var result struct {
		Title  string `json:"title"`
		Body   string `json:"body"`
		Number int    `json:"number"`
		Files  []struct {
			Path   string `json:"path"`
			Status string `json:"status"`
		} `json:"files"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse gh output: %w", err)
	}

	query := fmt.Sprintf(`query { repository(owner: %q, name: %q) { pullRequest(number: %d) { baseRefOid headRefOid } } }`,
		owner, repo, number)
	gqlCmd := exec.Command("gh", "api", "graphql", "-f", "query="+query,
		"--jq", ".data.repository.pullRequest")
	gqlOut, err := gqlCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh graphql failed: %w", err)
	}
	var refs struct {
		BaseRefOid string `json:"baseRefOid"`
		HeadRefOid string `json:"headRefOid"`
	}
	if err := json.Unmarshal(gqlOut, &refs); err != nil {
		return nil, fmt.Errorf("parse graphql output: %w", err)
	}

	info := &PRInfo{
		Owner:   owner,
		Repo:    repo,
		Number:  result.Number,
		Title:   result.Title,
		Body:    result.Body,
		BaseSHA: refs.BaseRefOid,
		HeadSHA: refs.HeadRefOid,
	}
	for _, f := range result.Files {
		status := "M"
		switch f.Status {
		case "added":
			status = "A"
		case "deleted":
			status = "D"
		case "renamed":
			status = "R"
		case "copied":
			status = "A"
		}
		info.Files = append(info.Files, PRFile{Path: f.Path, Status: status})
	}
	return info, nil
}

func FetchPRDiff(owner, repo string, number int) (string, error) {
	repoArg := owner + "/" + repo
	cmd := exec.Command("gh", "pr", "diff", strconv.Itoa(number), "--repo", repoArg)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh pr diff failed: %w", err)
	}
	return string(out), nil
}

func FetchFileContent(owner, repo, path, ref string) (string, error) {
	repoArg := owner + "/" + repo
	cmd := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/contents/%s?ref=%s", repoArg, path, ref),
		"-H", "Accept: application/vnd.github.raw+json")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh api contents failed: %w", err)
	}
	return string(out), nil
}

func SplitMultiFileDiff(unified string) map[string]string {
	result := make(map[string]string)
	var currentFile string
	var currentLines []string
	inHeader := false
	hasStructure := false
	flush := func() {
		if currentFile != "" && hasStructure {
			result[currentFile] = strings.Join(currentLines, "\n")
		}
	}
	for _, line := range strings.Split(unified, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			currentFile = diffDestinationPath(strings.TrimPrefix(line, "diff --git "))
			currentLines = []string{line}
			inHeader = true
			hasStructure = false
			continue
		}
		currentLines = append(currentLines, line)
		if strings.HasPrefix(line, "@@ ") || line == "GIT binary patch" {
			hasStructure = true
			inHeader = false
		}
		if !inHeader {
			continue
		}
		for _, prefix := range []string{
			"old mode ", "new mode ", "new file mode ", "deleted file mode ",
			"rename from ", "rename to ",
			"copy from ", "copy to ", "Binary files ",
		} {
			if strings.HasPrefix(line, prefix) {
				hasStructure = true
				break
			}
		}
		for _, prefix := range []string{"rename to ", "copy to ", "+++ ", "--- "} {
			if !strings.HasPrefix(line, prefix) {
				continue
			}
			path := strings.TrimPrefix(line, prefix)
			if prefix == "+++ " || prefix == "--- " {
				path, _, _ = strings.Cut(path, "\t")
			}
			path = unquoteDiffPath(path)
			if path == "" || path == "/dev/null" {
				break
			}
			if prefix == "--- " {
				if currentFile == "" {
					currentFile = strings.TrimPrefix(path, "a/")
				}
			} else if prefix == "+++ " {
				currentFile = strings.TrimPrefix(path, "b/")
			} else {
				currentFile = path
			}
			break
		}
	}
	flush()
	return result
}

func unquoteDiffPath(path string) string {
	if strings.HasPrefix(path, "\"") {
		value, err := strconv.Unquote(path)
		if err != nil {
			return ""
		}
		return value
	}
	return path
}

func diffDestinationPath(paths string) string {
	if strings.HasPrefix(paths, "\"") {
		quoted, err := strconv.QuotedPrefix(paths)
		if err != nil {
			return ""
		}
		return strings.TrimPrefix(unquoteDiffPath(strings.TrimLeft(paths[len(quoted):], " ")), "b/")
	}
	if i := strings.Index(paths, " \"b/"); i >= 0 {
		return strings.TrimPrefix(unquoteDiffPath(paths[i+1:]), "b/")
	}
	// Git leaves spaces unquoted; equal old/new paths disambiguate embedded " b/".
	for offset := 0; offset < len(paths); {
		i := strings.Index(paths[offset:], " b/")
		if i < 0 {
			break
		}
		i += offset
		oldPath, newPath := strings.TrimPrefix(paths[:i], "a/"), paths[i+3:]
		if oldPath == newPath {
			return newPath
		}
		offset = i + 3
	}
	if i := strings.LastIndex(paths, " b/"); i >= 0 {
		return paths[i+3:]
	}
	if fields := strings.Fields(paths); len(fields) == 2 {
		return unquoteDiffPath(fields[1])
	}
	return ""
}
