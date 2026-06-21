package app

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v2"
)

type PrFetchResult struct {
	URL      string
	Info     *github.PRInfo
	Diffs    map[string]string
	Comments []github.PRComment
	Err      error
}

type DiffContentResult struct {
	TabName  string
	OldLines []string
	NewLines []string
	Err      error
}

// PrCommentAddResult carries the result of adding a PR comment.
type PrCommentAddResult struct {
	GroupName string
	Owner     string
	Repo      string
	Number    int
	Err       error
}

// PrCommentsRefreshResult carries refreshed comments for a PR group.
type PrCommentsRefreshResult struct {
	GroupName string
	Comments  []github.PRComment
	Err       error
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

		// Also fetch PR comments (non-blocking - errors here are not fatal)
		comments, _ := github.FetchPRComments(owner, repo, number)

		a.Screen.PostEvent(tcell.NewEventInterrupt(&PrFetchResult{URL: url, Info: info, Diffs: diffs, Comments: comments}))
	}()
}

// AddPRComment adds a general comment to a PR and refreshes comments.
func (a *App) AddPRComment(group *ui.ChangesGroup, body string) {
	if group == nil || group.PROwner == "" || group.PRNumber == 0 {
		a.StatusError("Cannot add comment: PR info not available")
		return
	}
	owner := group.PROwner
	repo := group.PRRepo
	number := group.PRNumber
	groupName := group.Name

	a.StatusNotify("Adding comment...")
	go func() {
		err := github.AddPRComment(owner, repo, number, body)
		a.Screen.PostEvent(tcell.NewEventInterrupt(&PrCommentAddResult{
			GroupName: groupName,
			Owner:     owner,
			Repo:      repo,
			Number:    number,
			Err:       err,
		}))
	}()
}

// RefreshPRComments re-fetches comments for a PR group.
func (a *App) RefreshPRComments(group *ui.ChangesGroup) {
	if group == nil || group.PROwner == "" || group.PRNumber == 0 {
		return
	}
	owner := group.PROwner
	repo := group.PRRepo
	number := group.PRNumber
	groupName := group.Name

	go func() {
		comments, err := github.FetchPRComments(owner, repo, number)
		a.Screen.PostEvent(tcell.NewEventInterrupt(&PrCommentsRefreshResult{
			GroupName: groupName,
			Comments:  comments,
			Err:       err,
		}))
	}()
}
