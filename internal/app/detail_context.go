package app

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

const commitDetailContextTimeout = 15 * time.Second

type CommitDetailContextResult struct {
	TabID     string
	Dir       string
	Ref       string
	Gen       uint64
	FileIndex int
	FileKey   string
	OldLines  []string
	NewLines  []string
}

func (a *App) wireCommitDetailContext(tabID string, detail *ui.CommitDetailWidget) {
	if detail == nil {
		return
	}
	detail.OnFetchContext = func(fileIndex int, file ui.CommitDetailFile) {
		ctx, cancel := context.WithTimeout(context.Background(), commitDetailContextTimeout)
		dir, ref, gen, current := detail.Dir, detail.Ref, detail.LoadGen, detail.CurrentChanges
		read := func() *CommitDetailContextResult {
			defer cancel()
			return readCommitDetailContext(ctx, tabID, dir, ref, gen, current, fileIndex, file)
		}
		if a.Screen == nil {
			a.ApplyCommitDetailContext(read())
			return
		}
		screen := a.Screen
		go func() {
			screen.PostEvent(tcell.NewEventInterrupt(read()))
		}()
	}
}

func readCommitDetailContext(ctx context.Context, tabID, dir, ref string, gen uint64, current bool, fileIndex int, file ui.CommitDetailFile) *CommitDetailContextResult {
	result := &CommitDetailContextResult{
		TabID: tabID, Dir: dir, Ref: ref, Gen: gen,
		FileIndex: fileIndex, FileKey: ui.CommitDetailContextKey(file),
	}
	oldPath := file.Path
	if file.OldPath != "" {
		oldPath = file.OldPath
	}
	if current {
		if content, err := git.ShowFileContext(ctx, dir, oldPath, "HEAD"); err == nil {
			result.OldLines = splitFileLines(content)
		}
		if content, err := os.ReadFile(filepath.Join(dir, file.Path)); err == nil {
			result.NewLines = splitFileLines(string(content))
		}
		return result
	}
	if content, err := git.ShowFileContext(ctx, dir, oldPath, ref+"^"); err == nil {
		result.OldLines = splitFileLines(content)
	}
	if content, err := git.ShowFileContext(ctx, dir, file.Path, ref); err == nil {
		result.NewLines = splitFileLines(content)
	}
	return result
}

func (a *App) ApplyCommitDetailContext(result *CommitDetailContextResult) {
	if result == nil {
		return
	}
	detail := a.EditorGroup.CommitDetailWidgetByTab(result.TabID)
	if detail == nil || detail.Dir != result.Dir || detail.Ref != result.Ref || detail.LoadGen != result.Gen {
		return
	}
	detail.ApplyFileContext(result.FileIndex, result.FileKey, result.OldLines, result.NewLines)
}
