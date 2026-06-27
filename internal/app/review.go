package app

import (
	"fmt"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v2"
)

// ReviewCommentsFetchResult is posted back to the event loop when comments are fetched.
type ReviewCommentsFetchResult struct {
	Owner    string
	Repo     string
	Number   int
	Comments []github.PRComment
	Err      error
}

// FetchReviewComments fetches PR review comments asynchronously.
func (a *App) FetchReviewComments(owner, repo string, number int) {
	a.ReviewInbox.Loading = true
	a.ReviewInbox.PROwner = owner
	a.ReviewInbox.PRRepo = repo
	a.ReviewInbox.PRNumber = number
	a.StatusNotify(fmt.Sprintf("Fetching review comments for PR #%d...", number))

	go func() {
		comments, err := github.FetchPRComments(owner, repo, number)
		a.Screen.PostEvent(tcell.NewEventInterrupt(&ReviewCommentsFetchResult{
			Owner:    owner,
			Repo:     repo,
			Number:   number,
			Comments: comments,
			Err:      err,
		}))
	}()
}

// HandleReviewCommentsFetched processes the result of a comment fetch.
func (a *App) HandleReviewCommentsFetched(result *ReviewCommentsFetchResult) {
	a.ReviewInbox.Loading = false

	if result.Err != nil {
		a.StatusError("Failed to fetch review comments: " + result.Err.Error())
		return
	}

	// Find the workspace dir for auto-detection
	dir := a.findRepoDir()

	// Load or create review state
	state, err := github.LoadReviewState(dir)
	if err != nil || state == nil || state.PRNumber != result.Number {
		state = github.NewReviewState(result.Owner, result.Repo, result.Number)
	}

	// Auto-detect addressed comments
	if dir != "" {
		addressed := github.DetectAddressed(dir, result.Comments)
		for id := range addressed {
			// Only upgrade open comments to addressed, don't downgrade verified/dismissed
			if state.GetState(id) == github.StateOpen {
				state.SetState(id, github.StateAddressed)
			}
		}
	}

	// Populate inbox
	a.ReviewInbox.SetComments(result.Comments, state)

	// Save state
	if dir != "" {
		state.Save(dir)
	}

	// Update gutter markers for the active file
	a.updateCommentMarkers()

	// Show inbox
	a.Sidebar.SetActivePanel("inbox")
	if !a.Sidebar.Visible {
		a.ShowSidebar()
	}
	a.Root.SetFocus(a.ReviewInbox)

	total := a.ReviewInbox.TotalComments()
	progress := a.ReviewInbox.ProgressText()
	a.StatusNotify(fmt.Sprintf("Loaded %d review comments (%s)", total, progress))
	a.Sidebar.SetPanelDirty("inbox", total > 0)
}

// ReviewNextUnresolved navigates to the next unresolved comment.
func (a *App) ReviewNextUnresolved() {
	unresolved := a.ReviewInbox.UnresolvedComments()
	if len(unresolved) == 0 {
		a.StatusNotify("No unresolved comments")
		return
	}

	// Find the current position
	currentFile := a.EditorGroup.ActiveFilePath()
	currentLine := 0
	if a.EditorGroup.IsEditorActive() {
		currentLine = a.EditorGroup.Editor.Cursor.Line + 1 // 1-based
	}

	// Find the next comment after the current position
	var target *ui.ReviewInboxItem
	var wrapTarget *ui.ReviewInboxItem

	for i := range unresolved {
		item := &unresolved[i]
		if !item.Comment.IsInline {
			continue
		}
		itemFile := a.resolveCommentPath(item.Comment.Path)

		if wrapTarget == nil {
			wrapTarget = item
		}

		if itemFile == currentFile && item.Comment.Line > currentLine {
			target = item
			break
		}
		if itemFile > currentFile {
			target = item
			break
		}
	}

	if target == nil {
		target = wrapTarget // wrap around
	}

	if target == nil {
		a.StatusNotify("No unresolved inline comments")
		return
	}

	a.navigateToComment(target.Comment)
}

// ReviewPrevUnresolved navigates to the previous unresolved comment.
func (a *App) ReviewPrevUnresolved() {
	unresolved := a.ReviewInbox.UnresolvedComments()
	if len(unresolved) == 0 {
		a.StatusNotify("No unresolved comments")
		return
	}

	currentFile := a.EditorGroup.ActiveFilePath()
	currentLine := 0
	if a.EditorGroup.IsEditorActive() {
		currentLine = a.EditorGroup.Editor.Cursor.Line + 1
	}

	var target *ui.ReviewInboxItem

	for i := len(unresolved) - 1; i >= 0; i-- {
		item := &unresolved[i]
		if !item.Comment.IsInline {
			continue
		}
		itemFile := a.resolveCommentPath(item.Comment.Path)

		if itemFile == currentFile && item.Comment.Line < currentLine {
			target = item
			break
		}
		if itemFile < currentFile {
			target = item
			break
		}
	}

	if target == nil {
		// Wrap around to last
		for i := len(unresolved) - 1; i >= 0; i-- {
			if unresolved[i].Comment.IsInline {
				target = &unresolved[i]
				break
			}
		}
	}

	if target == nil {
		a.StatusNotify("No unresolved inline comments")
		return
	}

	a.navigateToComment(target.Comment)
}

// ReviewMarkVerified marks the comment on the current line as verified.
func (a *App) ReviewMarkVerified() {
	if !a.ReviewInbox.HasData() {
		return
	}

	// Check if there's a selected comment in the inbox
	if sel := a.ReviewInbox.SelectedComment(); sel != nil {
		a.setCommentState(sel.Comment.ID, github.StateVerified)
		return
	}
}

// ReviewAddInlineComment opens a dialog to compose a comment for the current line.
func (a *App) ReviewAddInlineComment() {
	if !a.ReviewInbox.HasData() || a.ReviewInbox.PRNumber == 0 {
		a.StatusNotify("No active PR review")
		return
	}

	currentFile := a.EditorGroup.ActiveFilePath()
	if currentFile == "" {
		return
	}
	currentLine := 1
	if a.EditorGroup.IsEditorActive() {
		currentLine = a.EditorGroup.Editor.Cursor.Line + 1
	}

	// Find the relative path from the repo root
	dir := a.findRepoDir()
	relPath := currentFile
	if dir != "" {
		if rel, err := filepath.Rel(dir, currentFile); err == nil {
			relPath = rel
		}
	}

	a.ShowInputDialog(
		fmt.Sprintf("Comment on %s:%d", filepath.Base(relPath), currentLine),
		"Enter your comment...",
		"",
		func(body string) {
			if body == "" {
				return
			}
			// Find the head SHA from the Changes widget PR groups
			commitID := a.findPRHeadSHA()
			if commitID == "" {
				a.StatusError("Could not determine PR head commit")
				return
			}
			go func() {
				err := github.AddPRInlineComment(
					a.ReviewInbox.PROwner,
					a.ReviewInbox.PRRepo,
					a.ReviewInbox.PRNumber,
					body, relPath, currentLine, commitID,
				)
				a.Screen.PostEvent(tcell.NewEventInterrupt(&reviewCommentPostResult{err: err}))
			}()
		},
	)
}

type reviewCommentPostResult struct {
	err error
}

// ReviewRefreshComments re-fetches comments for the active PR.
func (a *App) ReviewRefreshComments() {
	if a.ReviewInbox.PRNumber == 0 {
		a.StatusNotify("No active PR review to refresh")
		return
	}
	a.FetchReviewComments(a.ReviewInbox.PROwner, a.ReviewInbox.PRRepo, a.ReviewInbox.PRNumber)
}

// HandleReviewCommentPosted handles the result of posting a comment.
func (a *App) HandleReviewCommentPosted(result *reviewCommentPostResult) {
	if result.err != nil {
		a.StatusError("Failed to post comment: " + result.err.Error())
		return
	}
	a.StatusNotify("Comment posted successfully")
	// Refresh to pick up the new comment
	a.ReviewRefreshComments()
}

// setCommentState updates a comment's state and persists it.
func (a *App) setCommentState(commentID int, state github.CommentState) {
	a.ReviewInbox.UpdateCommentState(commentID, state)
	a.updateCommentMarkers()

	dir := a.findRepoDir()
	if dir != "" && a.ReviewInbox.State != nil {
		a.ReviewInbox.State.Save(dir)
	}

	a.Sidebar.SetPanelDirty("inbox", a.ReviewInbox.TotalComments() > 0)
}

// navigateToComment opens a file and jumps to the comment's line.
func (a *App) navigateToComment(c github.PRComment) {
	path := a.resolveCommentPath(c.Path)
	a.EditorGroup.OpenFile(path)
	if c.Line > 0 {
		a.EditorGroup.GoToLine(c.Line)
	}
	a.FocusEditorIfEnabled()
}

// resolveCommentPath converts a relative PR path to an absolute path.
func (a *App) resolveCommentPath(relPath string) string {
	dir := a.findRepoDir()
	if dir == "" {
		return relPath
	}
	absPath := filepath.Join(dir, relPath)
	return absPath
}

// findRepoDir finds the git repo root for the workspace.
func (a *App) findRepoDir() string {
	paths := a.Workspace.Paths()
	for _, p := range paths {
		if root := git.RepoRoot(p); root != "" {
			return root
		}
	}
	if len(paths) > 0 {
		return paths[0]
	}
	return ""
}

// findPRHeadSHA finds the head SHA from PR groups in the Changes widget.
func (a *App) findPRHeadSHA() string {
	for _, g := range a.Changes.Groups {
		if g.IsPR {
			return g.PRHeadSHA
		}
	}
	return ""
}

// updateCommentMarkers updates the gutter markers for the active editor tab.
func (a *App) updateCommentMarkers() {
	if !a.ReviewInbox.HasData() {
		return
	}
	filePath := a.EditorGroup.ActiveFilePath()
	if filePath == "" {
		a.EditorGroup.Editor.CommentMarkers = nil
		return
	}

	// Try to match the file path against comment paths
	dir := a.findRepoDir()
	relPath := filePath
	if dir != "" {
		if rel, err := filepath.Rel(dir, filePath); err == nil {
			relPath = rel
		}
	}

	markers := a.ReviewInbox.CommentMarkersForFile(relPath)
	a.EditorGroup.Editor.CommentMarkers = markers
}

// ReviewStatusText returns the status bar text for the review state.
func (a *App) ReviewStatusText() string {
	if !a.ReviewInbox.HasData() {
		return ""
	}
	return a.ReviewInbox.ProgressText()
}

// LoadReviewCommentsForPR loads review comments when a PR is opened.
// Should be called after a PR is loaded in the Changes widget.
func (a *App) LoadReviewCommentsForPR(owner, repo string, number int) {
	a.FetchReviewComments(owner, repo, number)
}
