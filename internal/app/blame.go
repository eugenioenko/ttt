package app

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/view"
)

const blameDelay = 150 * time.Millisecond

type blameKey struct {
	Repo, File, EOL string
	Line            int
	Version         uint64
	Dirty           bool
	RepoState       [32]byte
}

type statusBlameState struct {
	debounce *asynctask.Task[blameKey]
	read     *asynctask.Task[*git.BlameInfo]
}

func (a *App) SyncBlame() {
	repoDir := a.SyncRepositoryBranch()
	buf := a.EditorGroup.ActiveBuffer()
	filePath := a.EditorGroup.ActiveFilePath()
	if repoDir == "" || buf == nil || a.EditorGroup.IsActiveVirtual() {
		a.stopBlame()
		return
	}
	if _, ok := a.Repository.gitRelativePath(repoDir, filePath); !ok {
		a.stopBlame()
		return
	}
	line, _ := a.EditorGroup.ActiveCursor()
	key := blameKey{
		Repo: repoDir, File: filePath, Line: line + 1,
		Version: buf.Version(), Dirty: buf.Dirty, EOL: buf.LineEnding,
		RepoState: a.Repository.currentInputs[repoDir].identity,
	}
	if a.blame.debounce == nil {
		a.blame.debounce = NewTask[blameKey](a, asynctask.TaskOptions{Delay: blameDelay})
	}
	a.blame.debounce.ScheduleKey(key, func(context.Context) (blameKey, error) {
		return key, nil
	}, func(key blameKey, err error) {
		if err == nil {
			a.requestBlame(key)
		}
	})
	// Until the changed key reaches UI delivery, the previous blame is stale.
	if a.blame.debounce.Pending() {
		if a.blame.read != nil {
			a.blame.read.Stop()
		}
		a.Status.RemoveSegment("blame")
	}
}

func (a *App) stopBlame() {
	if a.blame.debounce != nil {
		a.blame.debounce.Stop()
	}
	if a.blame.read != nil {
		a.blame.read.Stop()
	}
	a.Status.RemoveSegment("blame")
}

func (a *App) requestBlame(key blameKey) {
	buf := a.EditorGroup.ActiveBuffer()
	file, ok := a.Repository.gitRelativePath(key.Repo, key.File)
	if buf == nil || !ok {
		return
	}
	a.Status.RemoveSegment("blame")
	var lines []string
	if key.Dirty {
		// Snapshot on the UI goroutine before the worker can race with edits.
		lines = slices.Clone(buf.Lines)
	}
	eol := key.EOL
	if eol == "" {
		eol = "\n"
	}
	if a.blame.read == nil {
		a.blame.read = NewTask[*git.BlameInfo](a, asynctask.TaskOptions{})
	}
	a.blame.read.Schedule(func(ctx context.Context) (*git.BlameInfo, error) {
		if lines != nil {
			return git.BlameLineContents(ctx, key.Repo, file, key.Line, strings.Join(lines, eol)), ctx.Err()
		}
		return git.BlameLine(ctx, key.Repo, file, key.Line), ctx.Err()
	}, func(info *git.BlameInfo, err error) {
		if err == nil && info != nil {
			a.Status.SetSegment(view.StatusSegment{ID: "blame", Side: "left", Priority: 200,
				Text: fmt.Sprintf("%s, %s", info.Author, git.FormatRelativeTime(info.Time))})
		}
	})
}
