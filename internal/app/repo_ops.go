package app

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/gdamore/tcell/v3"
)

// RepoOp is one source-control operation applied to a repository directory.
// Operations are values rather than hard-wired calls so another backend can
// supply its own without touching the runner.
type RepoOp struct {
	Name string
	Run  func(dir string) error
}

var (
	OpPull = RepoOp{Name: "pull", Run: git.Pull}
	OpPush = RepoOp{Name: "push", Run: git.Push}
)

func OpCommit(message string) RepoOp {
	return RepoOp{Name: "commit", Run: func(dir string) error { return git.Commit(dir, message) }}
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
		a.StatusWarn(a.runningRepoOp + " is already running")
		return
	}
	if len(task.Dirs) == 0 || len(task.Ops) == 0 {
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
		a.StatusError(fmt.Sprintf("%s failed: %v", r.Task.Progress, r.Err))
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
		if op.Name == "commit" || op.Name == "pull" {
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
