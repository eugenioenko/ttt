package app

import (
	"context"
	"strings"
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
	FileIndex int
	FileKey   string
	OldLines  []string
	NewLines  []string
}

func (a *App) wireCommitDetailContext(tabID string, detail *ui.CommitDetailWidget) {
	detail.OnFetchContext = func(fileIndex int, file ui.CommitDetailFile) {
		dir, ref := detail.Dir, detail.Ref
		read := func() *CommitDetailContextResult {
			ctx, cancel := context.WithTimeout(context.Background(), commitDetailContextTimeout)
			defer cancel()
			return readCommitDetailContext(ctx, tabID, dir, ref, fileIndex, file)
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

func readCommitDetailContext(ctx context.Context, tabID, dir, ref string, fileIndex int, file ui.CommitDetailFile) *CommitDetailContextResult {
	result := &CommitDetailContextResult{
		TabID: tabID, Dir: dir, Ref: ref, FileIndex: fileIndex,
		FileKey: ui.CommitDetailContextKey(file),
	}
	oldPath := file.Path
	if file.OldPath != "" {
		oldPath = file.OldPath
	}
	if content, err := git.ShowFileContext(ctx, dir, oldPath, ref+"^"); err == nil {
		result.OldLines = splitCommitDetailLines(content)
	}
	if content, err := git.ShowFileContext(ctx, dir, file.Path, ref); err == nil {
		result.NewLines = splitCommitDetailLines(content)
	}
	return result
}

func splitCommitDetailLines(content string) []string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func (a *App) ApplyCommitDetailContext(result *CommitDetailContextResult) {
	detail := a.EditorGroup.CommitDetailWidgetByTab(result.TabID)
	if detail == nil || detail.Dir != result.Dir || detail.Ref != result.Ref {
		return
	}
	detail.ApplyFileContext(result.FileIndex, result.FileKey, result.OldLines, result.NewLines)
}
