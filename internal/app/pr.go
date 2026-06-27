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

// PRCommentsResult is posted back to the event loop after fetching comments.
type PRCommentsResult struct {
	Comments []github.PRComment
	Owner    string
	Repo     string
	Number   int
	Err      error
}

// PRCommentAddResult is posted after adding a comment.
type PRCommentAddResult struct {
	Owner  string
	Repo   string
	Number int
	Err    error
}

// FetchPRComments fetches review comments asynchronously and posts the result.
func (a *App) FetchPRComments(owner, repo string, number int) {
	go func() {
		comments, err := github.FetchPRComments(owner, repo, number)
		a.Screen.PostEvent(tcell.NewEventInterrupt(&PRCommentsResult{
			Comments: comments,
			Owner:    owner,
			Repo:     repo,
			Number:   number,
			Err:      err,
		}))
	}()
}

// AddPRComment submits a general comment asynchronously and posts the result.
func (a *App) AddPRCommentAsync(owner, repo string, number int, body string) {
	a.StatusNotify("Posting comment...")
	go func() {
		err := github.AddPRComment(owner, repo, number, body)
		a.Screen.PostEvent(tcell.NewEventInterrupt(&PRCommentAddResult{
			Owner:  owner,
			Repo:   repo,
			Number: number,
			Err:    err,
		}))
	}()
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
