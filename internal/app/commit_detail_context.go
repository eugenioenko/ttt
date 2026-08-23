package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/gdamore/tcell/v3"
)

const commitDetailContextTimeout = 15 * time.Second

type CommitDetailContextResult struct {
	Incarnation uint64
	TabID       string
	Dir         string
	Ref         string
	FileIndex   int
	FileKey     string
	OldLines    []string
	NewLines    []string
	ContentKind ui.CommitDetailContentKind
	Err         string
	Canceled    bool
}

func (a *App) wireCommitDetailContext(tabID string, detail *ui.CommitDetailWidget, request commitDetailRequest) {
	detail.OnFetchContext = func(fileIndex int, file ui.CommitDetailFile) {
		dir, ref := detail.Dir, detail.Ref
		read := func() *CommitDetailContextResult {
			ctx, cancel := context.WithTimeout(request.Context, commitDetailContextTimeout)
			defer cancel()
			return readCommitDetailContext(ctx, request.Incarnation, tabID, dir, ref, fileIndex, file)
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

func readCommitDetailContext(ctx context.Context, incarnation uint64, tabID, dir, ref string, fileIndex int, file ui.CommitDetailFile) *CommitDetailContextResult {
	result := &CommitDetailContextResult{
		Incarnation: incarnation, TabID: tabID, Dir: dir, Ref: ref, FileIndex: fileIndex,
		FileKey: ui.CommitDetailContextKey(file),
	}
	oldPath := file.Path
	if file.OldPath != "" {
		oldPath = file.OldPath
	}
	var oldContent, newContent []byte
	if file.Status != "A" {
		content, err := git.ShowFileBytesContext(ctx, dir, oldPath, ref+"^")
		if err != nil {
			if errors.Is(err, context.Canceled) {
				result.Canceled = true
				return result
			}
			result.Err = fmt.Sprintf("Could not load full file for %s", commitContextDisplayPath(file.Path))
		} else {
			oldContent = content
		}
	}
	if file.Status != "D" {
		content, err := git.ShowFileBytesContext(ctx, dir, file.Path, ref)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				result.Canceled = true
				return result
			}
			result.Err = fmt.Sprintf("Could not load full file for %s", commitContextDisplayPath(file.Path))
		} else {
			newContent = content
		}
	}
	if result.Err != "" {
		return result
	}
	if isBinaryCommitContent(oldContent) || isBinaryCommitContent(newContent) {
		result.ContentKind = ui.CommitDetailContentBinary
		return result
	}
	if len(oldContent) == 0 && len(newContent) == 0 {
		result.ContentKind = ui.CommitDetailContentEmpty
		return result
	}
	if file.Status != "A" {
		result.OldLines = splitCommitDetailLines(string(oldContent))
	}
	if file.Status != "D" {
		result.NewLines = splitCommitDetailLines(string(newContent))
	}
	return result
}

func isBinaryCommitContent(content []byte) bool {
	return bytes.IndexByte(content, 0) >= 0 || !utf8.Valid(content)
}

func commitContextDisplayPath(path string) string {
	for _, r := range path {
		if !unicode.IsGraphic(r) {
			return strconv.QuoteToGraphic(path)
		}
	}
	return path
}

func splitCommitDetailLines(content string) []string {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func (a *App) ApplyCommitDetailContext(result *CommitDetailContextResult) {
	if result == nil || result.Canceled {
		return
	}
	detail := a.EditorGroup.CommitDetailWidgetByTab(result.TabID)
	if detail == nil || detail.Dir != result.Dir || detail.Ref != result.Ref || detail.Incarnation != result.Incarnation {
		return
	}
	applied := detail.ApplyFileContextContent(result.FileIndex, result.FileKey, result.OldLines, result.NewLines, result.ContentKind, result.Err)
	if applied && result.Err != "" && a.Status != nil && a.Screen != nil {
		a.StatusError(result.Err)
	}
}
