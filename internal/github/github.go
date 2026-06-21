package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type PRFile struct {
	Path   string
	Status string // A, M, D, R
}

type PRComment struct {
	ID        int
	Body      string
	User      string
	CreatedAt string
	Path      string // empty for general comments
	Line      int    // 0 for general comments
	IsInline  bool
}

type PRInfo struct {
	Owner   string
	Repo    string
	Number  int
	Title   string
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

func FetchPRInfo(owner, repo string, number int) (*PRInfo, error) {
	repoArg := owner + "/" + repo
	numStr := strconv.Itoa(number)

	cmd := exec.Command("gh", "pr", "view", numStr,
		"--repo", repoArg,
		"--json", "title,number,files")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr view failed: %w", err)
	}
	var result struct {
		Title  string `json:"title"`
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

// FetchPRComments fetches both inline review comments and general issue comments
// for a pull request. Returns them as a unified slice sorted by creation time.
func FetchPRComments(owner, repo string, number int) ([]PRComment, error) {
	repoArg := owner + "/" + repo
	numStr := strconv.Itoa(number)

	// Fetch inline review comments (on specific lines of code)
	reviewCmd := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/pulls/%s/comments", repoArg, numStr),
		"--paginate")
	reviewOut, err := reviewCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh api pull comments failed: %w", err)
	}

	var reviewComments []struct {
		ID   int    `json:"id"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		CreatedAt string `json:"created_at"`
		Path      string `json:"path"`
		Line      *int   `json:"line"`
	}
	if len(reviewOut) > 0 {
		if err := json.Unmarshal(reviewOut, &reviewComments); err != nil {
			return nil, fmt.Errorf("parse review comments: %w", err)
		}
	}

	// Fetch general issue comments (not attached to specific lines)
	issueCmd := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/issues/%s/comments", repoArg, numStr),
		"--paginate")
	issueOut, err := issueCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh api issue comments failed: %w", err)
	}

	var issueComments []struct {
		ID   int    `json:"id"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		CreatedAt string `json:"created_at"`
	}
	if len(issueOut) > 0 {
		if err := json.Unmarshal(issueOut, &issueComments); err != nil {
			return nil, fmt.Errorf("parse issue comments: %w", err)
		}
	}

	var comments []PRComment

	for _, rc := range reviewComments {
		line := 0
		if rc.Line != nil {
			line = *rc.Line
		}
		comments = append(comments, PRComment{
			ID:        rc.ID,
			Body:      rc.Body,
			User:      rc.User.Login,
			CreatedAt: rc.CreatedAt,
			Path:      rc.Path,
			Line:      line,
			IsInline:  true,
		})
	}

	for _, ic := range issueComments {
		comments = append(comments, PRComment{
			ID:        ic.ID,
			Body:      ic.Body,
			User:      ic.User.Login,
			CreatedAt: ic.CreatedAt,
			IsInline:  false,
		})
	}

	return comments, nil
}

// AddPRComment adds a general comment to a pull request.
func AddPRComment(owner, repo string, number int, body string) error {
	repoArg := owner + "/" + repo
	numStr := strconv.Itoa(number)
	payload, _ := json.Marshal(map[string]string{"body": body})
	cmd := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/issues/%s/comments", repoArg, numStr),
		"-X", "POST",
		"--input", "-")
	cmd.Stdin = strings.NewReader(string(payload))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("add comment failed: %w: %s", err, string(out))
	}
	return nil
}

// AddPRInlineComment adds an inline review comment on a specific file and line.
func AddPRInlineComment(owner, repo string, number int, body, path string, line int) error {
	repoArg := owner + "/" + repo
	numStr := strconv.Itoa(number)

	// First get the HEAD commit SHA for the PR
	cmd := exec.Command("gh", "pr", "view", numStr,
		"--repo", repoArg,
		"--json", "headRefOid", "--jq", ".headRefOid")
	shaOut, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("get PR head SHA failed: %w", err)
	}
	commitSHA := strings.TrimSpace(string(shaOut))

	payload, _ := json.Marshal(map[string]interface{}{
		"body":      body,
		"commit_id": commitSHA,
		"path":      path,
		"line":      line,
	})
	postCmd := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/pulls/%s/comments", repoArg, numStr),
		"-X", "POST",
		"--input", "-")
	postCmd.Stdin = strings.NewReader(string(payload))
	if out, err := postCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("add inline comment failed: %w: %s", err, string(out))
	}
	return nil
}

// FormatCommentTime formats a GitHub API timestamp into a short relative or absolute form.
func FormatCommentTime(createdAt string) string {
	if len(createdAt) >= 10 {
		return createdAt[:10]
	}
	return createdAt
}

// CommentsForFile returns only the inline comments for a specific file path.
func CommentsForFile(comments []PRComment, path string) []PRComment {
	var result []PRComment
	for _, c := range comments {
		if c.IsInline && c.Path == path {
			result = append(result, c)
		}
	}
	return result
}

// GeneralComments returns only the non-inline (general) comments.
func GeneralComments(comments []PRComment) []PRComment {
	var result []PRComment
	for _, c := range comments {
		if !c.IsInline {
			result = append(result, c)
		}
	}
	return result
}

// FileCommentCounts returns a map of file path to number of inline comments.
func FileCommentCounts(comments []PRComment) map[string]int {
	counts := make(map[string]int)
	for _, c := range comments {
		if c.IsInline && c.Path != "" {
			counts[c.Path]++
		}
	}
	return counts
}

func SplitMultiFileDiff(unified string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(unified, "\n")
	var currentFile string
	var currentLines []string

	flush := func() {
		if currentFile != "" && len(currentLines) > 0 {
			result[currentFile] = strings.Join(currentLines, "\n")
		}
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				bPath := parts[len(parts)-1]
				if strings.HasPrefix(bPath, "b/") {
					currentFile = bPath[2:]
				} else {
					currentFile = bPath
				}
			}
			currentLines = []string{line}
		} else {
			currentLines = append(currentLines, line)
		}
	}
	flush()
	return result
}
