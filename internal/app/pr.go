package app

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/github"

	"github.com/gdamore/tcell/v2"
)

type PrFetchResult struct {
	URL   string
	Info  *github.PRInfo
	Diffs map[string]string
	Err   error
}

type DiffContentResult struct {
	TabName  string
	OldLines []string
	NewLines []string
	Err      error
}

// PRCommentsFetchResult delivers fetched PR comments to the event loop.
type PRCommentsFetchResult struct {
	Owner    string
	Repo     string
	Number   int
	Comments []github.PRComment
	Err      error
}

// PRCommentAddResult delivers the result of posting a new comment.
type PRCommentAddResult struct {
	Owner  string
	Repo   string
	Number int
	Err    error
}

// FetchPRComments fetches comments for a PR asynchronously and posts the
// result back to the event loop.
func (a *App) FetchPRComments(owner, repo string, number int) {
	a.Reviews.Loading = true
	go func() {
		comments, err := github.FetchPRComments(owner, repo, number)
		a.Screen.PostEvent(tcell.NewEventInterrupt(&PRCommentsFetchResult{
			Owner:    owner,
			Repo:     repo,
			Number:   number,
			Comments: comments,
			Err:      err,
		}))
	}()
}

// RefreshPRComments re-fetches comments for the currently loaded PR.
func (a *App) RefreshPRComments() {
	if a.Reviews.Owner == "" || a.Reviews.Number == 0 {
		return
	}
	a.FetchPRComments(a.Reviews.Owner, a.Reviews.Repo, a.Reviews.Number)
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
