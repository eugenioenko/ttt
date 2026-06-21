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

type DiffContentResult struct {
	TabName  string
	OldLines []string
	NewLines []string
	Err      error
}

// PRCommentsResult carries async comment fetch results back to the event loop.
type PRCommentsResult struct {
	Owner    string
	Repo     string
	Number   int
	Title    string
	Comments []github.PRComment
	Err      error
}

// PRCommentSubmitResult carries async comment submission results.
type PRCommentSubmitResult struct {
	Owner  string
	Repo   string
	Number int
	Title  string
	Err    error
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

// FetchPRComments fetches PR comments asynchronously and shows them in the panel.
func (a *App) FetchPRComments(owner, repo string, number int, title string) {
	// If a panel is already open, reuse it (just refresh comments)
	if a.CommentPanel != nil {
		a.CommentPanel.Loading = true
	} else {
		// Create and show the panel immediately with loading state
		panel := ui.NewCommentPanelWidget(fmt.Sprintf("PR #%d: %s", number, title))
		panel.Borders = a.Borders
		panel.Loading = true
		panel.OnClose = func() {
			a.DismissCommentPanel()
		}
		panel.OnOpenFile = func(path string, line int) {
			a.EditorGroup.OpenFile(path)
			if line > 0 {
				a.EditorGroup.GoToLine(line)
			}
			a.Root.SetFocus(a.EditorGroup)
		}
		panel.OnSubmit = func(body string) {
			a.submitPRComment(owner, repo, number, title, body)
		}
		a.CommentPanel = panel
		a.Root.PushOverlay(ui.Overlay{Widget: panel, Modal: false})
		a.Root.SetFocus(panel)
	}

	// Fetch comments in background
	go func() {
		comments, err := github.FetchPRComments(owner, repo, number)
		a.Screen.PostEvent(tcell.NewEventInterrupt(&PRCommentsResult{
			Owner:    owner,
			Repo:     repo,
			Number:   number,
			Title:    title,
			Comments: comments,
			Err:      err,
		}))
	}()
}

// submitPRComment sends a comment to a PR and refreshes the thread.
func (a *App) submitPRComment(owner, repo string, number int, title, body string) {
	a.StatusNotify("Submitting comment...")
	go func() {
		err := github.AddPRComment(owner, repo, number, body)
		a.Screen.PostEvent(tcell.NewEventInterrupt(&PRCommentSubmitResult{
			Owner:  owner,
			Repo:   repo,
			Number: number,
			Title:  title,
			Err:    err,
		}))
	}()
}

// DismissCommentPanel removes the comment panel overlay.
func (a *App) DismissCommentPanel() {
	if a.CommentPanel != nil {
		a.Root.PopOverlay()
		a.CommentPanel = nil
		a.FocusEditor()
	}
}

// ShowPRCommentsForGroup opens the comment panel for a PR group.
func (a *App) ShowPRCommentsForGroup() {
	// Find the first PR group in changes
	for _, g := range a.Changes.Groups {
		if g.IsPR {
			owner := g.PROwner
			repo := g.PRRepo
			// Parse number from group name
			var number int
			fmt.Sscanf(g.Name, "PR #%d:", &number)
			if number == 0 {
				continue
			}
			title := g.Name
			a.FetchPRComments(owner, repo, number, title)
			return
		}
	}
	a.StatusWarn("No PR open. Use 'Git: Review PR' first.")
}

// ShowPRCommentsDialog prompts for a PR URL and opens comments.
func (a *App) ShowPRCommentsDialog() {
	if a.Root.HasOverlay() {
		return
	}
	if !github.IsGHInstalled() {
		a.StatusError("GitHub CLI (gh) is required. Install from https://cli.github.com/")
		return
	}
	// If there's already a PR open, show its comments directly
	for _, g := range a.Changes.Groups {
		if g.IsPR {
			a.ShowPRCommentsForGroup()
			return
		}
	}
	// Otherwise prompt for URL
	dialog := ui.NewInputDialogWidget("PR Comments", "https://github.com/owner/repo/pull/123", "")
	dialog.ConfirmLabel = "Show Comments"
	dialog.Borders = a.Borders
	dialog.OnSubmit = func(url string) {
		a.DismissDialog()
		if url != "" {
			owner, repo, number, err := github.ParsePRURL(url)
			if err != nil {
				a.StatusError("Invalid PR URL: " + err.Error())
				return
			}
			a.FetchPRComments(owner, repo, number, fmt.Sprintf("PR #%d", number))
		}
	}
	dialog.OnDismiss = func() {
		a.DismissDialog()
	}
	a.ShowDialog(dialog)
}
