package github

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
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

type PRComment struct {
	ID        int
	Body      string
	User      string
	CreatedAt string
	UpdatedAt string
	Path      string // empty for general comments
	Line      int    // 0 for general comments
	IsInline  bool
	InReplyTo int // 0 if not a reply
}

func parseReviewComments(data []byte) ([]PRComment, error) {
	var raw []struct {
		ID   int    `json:"id"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		CreatedAt    string `json:"created_at"`
		UpdatedAt    string `json:"updated_at"`
		Path         string `json:"path"`
		Line         *int   `json:"line"`
		OriginalLine *int   `json:"original_line"`
		InReplyToID  int    `json:"in_reply_to_id"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse review comments: %w", err)
	}
	var comments []PRComment
	for _, r := range raw {
		line := 0
		if r.Line != nil {
			line = *r.Line
		} else if r.OriginalLine != nil {
			line = *r.OriginalLine
		}
		comments = append(comments, PRComment{
			ID:        r.ID,
			Body:      r.Body,
			User:      r.User.Login,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
			Path:      r.Path,
			Line:      line,
			IsInline:  true,
			InReplyTo: r.InReplyToID,
		})
	}
	return comments, nil
}

func parseIssueComments(data []byte) ([]PRComment, error) {
	var raw []struct {
		ID   int    `json:"id"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse issue comments: %w", err)
	}
	var comments []PRComment
	for _, r := range raw {
		comments = append(comments, PRComment{
			ID:        r.ID,
			Body:      r.Body,
			User:      r.User.Login,
			CreatedAt: r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
			IsInline:  false,
		})
	}
	return comments, nil
}

func FetchPRComments(owner, repo string, number int) ([]PRComment, error) {
	// Fetch inline review comments
	reviewCmd := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/%s/pulls/%d/comments", owner, repo, number),
		"--paginate")
	reviewOut, err := reviewCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh api pull comments failed: %w", err)
	}
	reviewComments, err := parseReviewComments(reviewOut)
	if err != nil {
		return nil, err
	}

	// Fetch general issue comments
	issueCmd := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/%s/issues/%d/comments", owner, repo, number),
		"--paginate")
	issueOut, err := issueCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh api issue comments failed: %w", err)
	}
	issueComments, err := parseIssueComments(issueOut)
	if err != nil {
		return nil, err
	}

	// Combine and sort by ID
	all := append(reviewComments, issueComments...)
	sort.Slice(all, func(i, j int) bool {
		return all[i].ID < all[j].ID
	})
	return all, nil
}

func AddPRComment(owner, repo string, number int, body string) error {
	cmd := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/%s/issues/%d/comments", owner, repo, number),
		"-f", "body="+body)
	if _, err := cmd.Output(); err != nil {
		return fmt.Errorf("gh api add comment failed: %w", err)
	}
	return nil
}

func AddPRInlineComment(owner, repo string, number int, body, path string, line int, commitID string) error {
	cmd := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/%s/pulls/%d/comments", owner, repo, number),
		"-f", "body="+body,
		"-f", "path="+path,
		"-F", "line="+strconv.Itoa(line),
		"-f", "commit_id="+commitID)
	if _, err := cmd.Output(); err != nil {
		return fmt.Errorf("gh api add inline comment failed: %w", err)
	}
	return nil
}
