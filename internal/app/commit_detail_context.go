package app

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/scm"
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
	source := a.Repository.repositorySource(detail.Dir)
	detail.OnFetchContext = func(fileIndex int, file ui.CommitDetailFile) {
		ref := detail.Ref
		read := func() *CommitDetailContextResult {
			ctx, cancel := context.WithTimeout(request.Context, commitDetailContextTimeout)
			defer cancel()
			return readCommitDetailContext(ctx, request.Incarnation, tabID, source, ref, fileIndex, file)
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

func readCommitDetailContext(ctx context.Context, incarnation uint64, tabID string, source repositorySource, ref string, fileIndex int, file ui.CommitDetailFile) *CommitDetailContextResult {
	result := &CommitDetailContextResult{
		Incarnation: incarnation, TabID: tabID, Dir: source.root, Ref: ref, FileIndex: fileIndex,
		FileKey: ui.CommitDetailContextKey(file),
	}
	reader, err := source.revisionReader(ctx)
	var content scm.RevisionContent
	if err == nil {
		content, err = reader.ReadRevisionContent(ctx, ref, file.Path, file.OldPath)
	}
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		result.Canceled = errors.Is(err, context.Canceled)
		result.Err = i18n.Format("changes.couldNotLoadFullFileFor", i18n.Args{"path": commitContextDisplayPath(file.Path)})
		return result
	}
	oldContent, newContent := []byte(content.Before.Content), []byte(content.After.Content)

	if isBinaryCommitContent(oldContent) || isBinaryCommitContent(newContent) {
		result.ContentKind = ui.CommitDetailContentBinary
		return result
	}
	if len(oldContent) == 0 && len(newContent) == 0 {
		result.ContentKind = ui.CommitDetailContentEmpty
		return result
	}
	if content.Before.Exists {
		result.OldLines = splitCommitDetailLines(string(oldContent))
	}
	if content.After.Exists {
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
