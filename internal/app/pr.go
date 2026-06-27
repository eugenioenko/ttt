package app

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v2"
)

type PrFetchResult struct {
	URL   string
	Info  *github.PRInfo
	Diffs map[string]string
	Err   error
}

// PrCommentsFetchResult carries PR comments fetched asynchronously.
type PrCommentsFetchResult struct {
	Owner    string
	Repo     string
	Number   int
	Title    string
	HeadSHA  string
	Comments []github.PRComment
	Err      error
}

// PrAddCommentResult carries the result of adding a comment.
type PrAddCommentResult struct {
	Err error
	// Refetch triggers
	Owner  string
	Repo   string
	Number int
	Title  string
}

type DiffContentResult struct {
	TabName  string
	OldLines []string
	NewLines []string
	Err      error
}

// ActivePR tracks the currently loaded PR for comment operations.
type ActivePR struct {
	Owner   string
	Repo    string
	Number  int
	Title   string
	HeadSHA string
}

// FetchPRComments fetches comments for a PR asynchronously.
func (a *App) FetchPRComments(owner, repo string, number int, title, headSHA string) {
	a.StatusNotify(fmt.Sprintf("Fetching PR #%d comments...", number))
	go func() {
		comments, err := github.FetchPRComments(owner, repo, number)
		a.Screen.PostEvent(tcell.NewEventInterrupt(&PrCommentsFetchResult{
			Owner:    owner,
			Repo:     repo,
			Number:   number,
			Title:    title,
			HeadSHA:  headSHA,
			Comments: comments,
			Err:      err,
		}))
	}()
}

// AddPRComment adds a general comment to the active PR.
func (a *App) AddPRGeneralComment(pr *ActivePR, body string) {
	a.StatusNotify("Posting comment...")
	go func() {
		err := github.AddPRComment(pr.Owner, pr.Repo, pr.Number, body)
		a.Screen.PostEvent(tcell.NewEventInterrupt(&PrAddCommentResult{
			Err:    err,
			Owner:  pr.Owner,
			Repo:   pr.Repo,
			Number: pr.Number,
			Title:  pr.Title,
		}))
	}()
}

// AddPRInlineComment adds an inline comment on a specific file/line.
func (a *App) AddPRInlineComment(pr *ActivePR, body, path string, line int) {
	a.StatusNotify("Posting inline comment...")
	go func() {
		err := github.AddPRInlineComment(pr.Owner, pr.Repo, pr.Number, body, path, line, pr.HeadSHA)
		a.Screen.PostEvent(tcell.NewEventInterrupt(&PrAddCommentResult{
			Err:    err,
			Owner:  pr.Owner,
			Repo:   pr.Repo,
			Number: pr.Number,
			Title:  pr.Title,
		}))
	}()
}

// ShowReviewHub displays the review hub overlay with all PR comments.
func (a *App) ShowReviewHub(comments []github.PRComment, title string, number int) {
	// Convert github.PRComment to ui.ReviewComment
	var uiComments []ui.ReviewComment
	for _, c := range comments {
		uiComments = append(uiComments, ui.ReviewComment{
			ID:        c.ID,
			Body:      c.Body,
			User:      c.User,
			CreatedAt: c.CreatedAt,
			Path:      c.Path,
			Line:      c.Line,
			IsInline:  c.IsInline,
		})
	}

	hub := ui.NewReviewHubWidget(uiComments, title, number)
	hub.Borders = a.Borders
	hub.OnDismiss = func() {
		a.DismissDialog()
	}
	hub.OnNavigate = func(path string, line int) {
		a.DismissDialog()
		// Try to find file in the workspace and open it
		if path != "" {
			a.EditorGroup.OpenFile(path)
			if line > 0 {
				a.EditorGroup.GoToLine(line)
			}
			a.Root.SetFocus(a.EditorGroup)
		}
	}
	hub.OnAddComment = func() {
		if a.CurrentPR == nil {
			a.StatusWarn("No active PR")
			return
		}
		a.DismissDialog()
		a.ShowInputDialog("Add PR Comment", "Type your comment...", "", func(body string) {
			if body != "" {
				a.AddPRGeneralComment(a.CurrentPR, body)
			}
		})
	}
	hub.OnAddInlineComment = func(path string, line int) {
		if a.CurrentPR == nil {
			a.StatusWarn("No active PR")
			return
		}
		a.DismissDialog()
		placeholder := fmt.Sprintf("Reply to %s:%d", path, line)
		a.ShowInputDialog("Add Inline Comment", placeholder, "", func(body string) {
			if body != "" {
				a.AddPRInlineComment(a.CurrentPR, body, path, line)
			}
		})
	}
	a.ShowDialog(hub)
}

func (a *App) FetchAndOpenPR(url string) {
	owner, repo, number, err := github.ParsePRURL(url)
	if err != nil {
		a.StatusError("Invalid PR URL: " + err.Error())
		return
	}

	a.Changes.Loading = true
	a.StatusNotify(fmt.Sprintf("Fetching PR #%d...", number))

	go func() {
		info, err := github.FetchPRInfo(owner, repo, number)
		if err != nil {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&PrFetchResult{URL: url, Err: err}))
			return
		}

		diffText, err := github.FetchPRDiff(owner, repo, number)
		if err != nil {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&PrFetchResult{URL: url, Err: err}))
			return
		}

		diffs := github.SplitMultiFileDiff(diffText)
		a.Screen.PostEvent(tcell.NewEventInterrupt(&PrFetchResult{URL: url, Info: info, Diffs: diffs}))
	}()
}
