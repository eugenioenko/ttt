package app

import (
	"context"
	"fmt"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/jj"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/gdamore/tcell/v3"
)

// RepoOp is one source-control operation applied to a repository directory.
// Operations are values rather than hard-wired calls so another backend can
// supply its own without touching the runner.
type RepoOp struct {
	Name string
	Run  func(dir string) error
	// GitOnly ops run the git CLI directly. They skip jj workspaces, where a
	// colocated .git would otherwise let them mutate the repository behind jj.
	GitOnly bool
}

var (
	OpPull = RepoOp{Name: "pull", Run: git.Pull, GitOnly: true}
	OpPush = RepoOp{Name: "push", Run: git.Push, GitOnly: true}
)

func OpCommit(message string) RepoOp {
	return RepoOp{Name: "commit", Run: func(dir string) error { return git.Commit(dir, message) }, GitOnly: true}
}

func OpCreateBranch(name string) RepoOp {
	return RepoOp{Name: "branch", Run: func(dir string) error { return git.CheckoutNewBranch(dir, name) }, GitOnly: true}
}

// isJJDir discovers afresh rather than trusting the panel's cached repository,
// which predates a jj workspace initialized since the last refresh.
func (a *App) isJJDir(dir string) bool {
	if a.Changes.provider != nil {
		if repo, err := a.Changes.provider.Discover(context.Background(), dir); err == nil {
			return jj.IsRepository(repo)
		}
	}
	return jj.IsRepository(a.Changes.repository(dir))
}

func (a *App) withoutJJDirs(task RepoTask) RepoTask {
	gitOnly := false
	for _, op := range task.Ops {
		gitOnly = gitOnly || op.GitOnly
	}
	if !gitOnly {
		return task
	}
	dirs := make([]string, 0, len(task.Dirs))
	for _, dir := range task.Dirs {
		if !a.isJJDir(dir) {
			dirs = append(dirs, dir)
		}
	}
	task.Dirs = dirs
	return task
}

// RepoTask is a unit of background source-control work.
type RepoTask struct {
	Progress string // shown in the status bar while running, e.g. "Pulling"
	Done     string // shown on success, e.g. "Pulled successfully"
	Dirs     []string
	Ops      []RepoOp
	OnDone   func()
}

// RepoOpResult carries a finished task back to the event loop. Only the event
// loop touches widget state.
type RepoOpResult struct {
	Task RepoTask
	Err  error
}

// RunRepoTask runs the task's ops against every dir on a background goroutine,
// reporting progress in the status bar. Only one task runs at a time: git
// refuses concurrent writes to a repo with an index.lock error, which would
// surface as a spurious failure rather than the queueing a user expects.
func (a *App) RunRepoTask(task RepoTask) {
	if a.runningRepoOp != "" {
		a.StatusWarn(i18n.Format("status.isAlreadyRunning", i18n.Args{"runningRepoOp": a.runningRepoOp}))
		return
	}
	if len(task.Dirs) == 0 || len(task.Ops) == 0 {
		return
	}
	if task = a.withoutJJDirs(task); len(task.Dirs) == 0 {
		a.StatusWarn(i18n.Text("status.notAvailableInJJRepositories"))
		return
	}

	a.runningRepoOp = task.Progress
	a.setRepoOpSegment(task.Progress + "…")

	screen := a.Screen
	go func() {
		var failed error
		for _, dir := range task.Dirs {
			for _, op := range task.Ops {
				if err := op.Run(dir); err != nil {
					failed = fmt.Errorf("%s: %w", op.Name, err)
					break
				}
			}
			if failed != nil {
				break
			}
		}
		screen.PostEvent(tcell.NewEventInterrupt(&RepoOpResult{Task: task, Err: failed}))
	}()
}

// HandleRepoOpResult finishes a background task on the main thread.
func (a *App) HandleRepoOpResult(r *RepoOpResult) {
	a.runningRepoOp = ""
	a.setRepoOpSegment("")
	a.invalidateAllRepositories(repositoryResourcesForTask(r.Task))

	if r.Err != nil {
		a.StatusError(i18n.Format("status.failed", i18n.Args{"progress": r.Task.Progress, "err": r.Err}))
		return
	}
	if r.Task.OnDone != nil {
		r.Task.OnDone()
	}
	a.StatusNotify(r.Task.Done)
}

func repositoryResourcesForTask(task RepoTask) RepositoryResource {
	resources := RepositoryWorktree
	for _, op := range task.Ops {
		if op.Name == "commit" || op.Name == "pull" || op.Name == "branch" {
			resources |= RepositoryHistory
		}
	}
	return resources
}

func (a *App) setRepoOpSegment(text string) {
	a.Status.SetSegment(view.StatusSegment{
		ID: "repoop", Side: "left", Priority: 150, Text: text,
	})
}
