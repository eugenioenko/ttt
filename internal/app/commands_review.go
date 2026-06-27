package app

import (
	"github.com/eugenioenko/ttt/internal/command"
)

func registerReviewCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID:       "sidebar.inbox",
		Title:    "Show Review Inbox",
		Keywords: []string{"view", "review", "pr", "comments", "inbox"},
		Handler: func() {
			app.ShowPanel("inbox", app.ReviewInbox)
		},
	})

	reg.Register(command.Command{
		ID:       "review.showInbox",
		Title:    "Review: Show Inbox",
		Keywords: []string{"review", "pr", "comments"},
		Handler: func() {
			app.ShowPanel("inbox", app.ReviewInbox)
		},
	})

	reg.Register(command.Command{
		ID:       "review.nextUnresolved",
		Title:    "Review: Next Unresolved Comment",
		Keywords: []string{"review", "pr", "comment", "next"},
		Handler:  app.ReviewNextUnresolved,
	})

	reg.Register(command.Command{
		ID:       "review.prevUnresolved",
		Title:    "Review: Previous Unresolved Comment",
		Keywords: []string{"review", "pr", "comment", "previous"},
		Handler:  app.ReviewPrevUnresolved,
	})

	reg.Register(command.Command{
		ID:       "review.markVerified",
		Title:    "Review: Mark Comment Verified",
		Keywords: []string{"review", "pr", "comment", "verify", "resolve"},
		Handler:  app.ReviewMarkVerified,
	})

	reg.Register(command.Command{
		ID:       "review.addInlineComment",
		Title:    "Review: Add Inline Comment",
		Keywords: []string{"review", "pr", "comment", "add"},
		Handler:  app.ReviewAddInlineComment,
	})

	reg.Register(command.Command{
		ID:       "review.refresh",
		Title:    "Review: Refresh Comments",
		Keywords: []string{"review", "pr", "comment", "refresh"},
		Handler:  app.ReviewRefreshComments,
	})
}
