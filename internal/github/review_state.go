package github

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CommentState represents the review state of a comment thread
type CommentState int

const (
	StateOpen      CommentState = iota // Unresolved
	StateAddressed                     // Code changed since comment
	StateVerified                      // Reviewer confirmed fix
	StateDismissed                     // Won't fix
)

func (s CommentState) String() string {
	switch s {
	case StateOpen:
		return "open"
	case StateAddressed:
		return "addressed"
	case StateVerified:
		return "verified"
	case StateDismissed:
		return "dismissed"
	default:
		return "open"
	}
}

// ReviewState holds persistent state for a PR review session
type ReviewState struct {
	PRNumber int                  `json:"pr_number"`
	Owner    string               `json:"owner"`
	Repo     string               `json:"repo"`
	Comments map[int]CommentState `json:"comments"` // comment ID -> state
}

const reviewStateFile = ".ttt-review-state.json"

// NewReviewState creates an empty review state for a PR.
func NewReviewState(owner, repo string, number int) *ReviewState {
	return &ReviewState{
		PRNumber: number,
		Owner:    owner,
		Repo:     repo,
		Comments: make(map[int]CommentState),
	}
}

// SetState sets the review state for a comment.
func (rs *ReviewState) SetState(commentID int, state CommentState) {
	rs.Comments[commentID] = state
}

// GetState returns the review state for a comment, defaulting to StateOpen.
func (rs *ReviewState) GetState(commentID int) CommentState {
	if s, ok := rs.Comments[commentID]; ok {
		return s
	}
	return StateOpen
}

// Save writes the review state to {dir}/.ttt-review-state.json.
func (rs *ReviewState) Save(dir string) error {
	data, err := json.MarshalIndent(rs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, reviewStateFile), data, 0644)
}

// LoadReviewState reads review state from {dir}/.ttt-review-state.json.
func LoadReviewState(dir string) (*ReviewState, error) {
	data, err := os.ReadFile(filepath.Join(dir, reviewStateFile))
	if err != nil {
		return nil, err
	}
	var rs ReviewState
	if err := json.Unmarshal(data, &rs); err != nil {
		return nil, err
	}
	if rs.Comments == nil {
		rs.Comments = make(map[int]CommentState)
	}
	return &rs, nil
}

// CountByState returns the number of comments in each state.
func (rs *ReviewState) CountByState() (open, addressed, verified, dismissed int) {
	for _, s := range rs.Comments {
		switch s {
		case StateOpen:
			open++
		case StateAddressed:
			addressed++
		case StateVerified:
			verified++
		case StateDismissed:
			dismissed++
		}
	}
	return
}

// DetectAddressed checks whether the file referenced by each inline comment
// has been modified after the comment was created. It runs
// git log --since="<createdAt>" --oneline -- <path> for each comment and
// returns a map of comment IDs that are addressed (file was changed).
func DetectAddressed(dir string, comments []PRComment) map[int]bool {
	result := make(map[int]bool)
	for _, c := range comments {
		if !c.IsInline || c.Path == "" {
			continue
		}
		cmd := exec.Command("git", "-C", dir,
			"log", "--since="+c.CreatedAt, "--oneline", "--", c.Path)
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(out)) != "" {
			result[c.ID] = true
		}
	}
	return result
}
