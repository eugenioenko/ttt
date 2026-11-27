package ui

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/i18n"
)

// applySaveCleanups puts the save-time trim and final-newline handling on the
// undo stack, so Ctrl+Z reaches them (#312).
func (g *EditorGroupWidget) applySaveCleanups(t *editorTab) {
	if t.Buf == nil || g.Editor == nil {
		return
	}
	cleaned := t.Buf.CleanedLines()
	if slices.Equal(cleaned, t.Buf.Lines) {
		return
	}

	g.Editor.ExecCommand(&undo.ReplaceLinesCommand{
		Start:    0,
		OldLines: append([]string(nil), t.Buf.Lines...),
		NewLines: cleaned,
	})

	// Trimming can shorten the line the cursor sits on.
	g.Editor.clampCursor()
	if n := len([]rune(t.Buf.Lines[g.Editor.Cursor.Line])); g.Editor.Cursor.Col > n {
		g.Editor.Cursor.Col = n
	}
}

func (g *EditorGroupWidget) Save() bool {
	t := g.activeTab()
	if t == nil || t.Content != nil {
		return false
	}
	if t.Virtual || t.ReadOnly {
		return false
	}
	g.applySaveCleanups(t)
	if err := t.Buf.SaveFile(t.FilePath); err != nil {
		g.reportError(i18n.Format("editorGroup.failedToSave", i18n.Args{"filePath": t.FilePath, "error": saveErrorText(err)}))
		return false
	}
	if t.Undo != nil {
		t.Undo.MarkSaved()
	}
	g.syncTabs()
	g.SyncDocumentViews()
	return true
}

// applySaveSettings sets buf's save-time cleanups for path from settings and .editorconfig.
func (g *EditorGroupWidget) applySaveSettings(buf *buffer.Buffer, path string) config.EditorConfigProps {
	buf.InsertFinalNewline = g.InsertFinalNewline
	buf.TrimTrailingWhitespace = g.TrimTrailingWhitespace
	ec := config.LoadEditorConfig(path)
	if ec.InsertFinalNLSet {
		buf.InsertFinalNewline = ec.InsertFinalNewline
	}
	if ec.TrimTrailingWSSet {
		buf.TrimTrailingWhitespace = ec.TrimTrailingWS
	}
	return ec
}

// saveErrorText is the user-facing text of a save error, with the buffer's
// recovery and replacement errors translated.
func saveErrorText(err error) string {
	var recovery *buffer.RecoveryError
	if errors.As(err, &recovery) {
		return i18n.Format("editorGroup.saveRecoveryKept", i18n.Args{"error": saveErrorText(recovery.Err), "path": recovery.Path})
	}
	var replaced *buffer.ReplacedError
	if errors.As(err, &replaced) {
		return i18n.Format("editorGroup.saveTargetReplaced", i18n.Args{"path": replaced.Path})
	}
	return err.Error()
}

func (g *EditorGroupWidget) SaveAs(path string) bool {
	t := g.activeTab()
	if t == nil || t.Content != nil {
		return false
	}
	replaceTabs, err := g.SaveAsTargets(path)
	if err != nil {
		g.reportError(err.Error())
		return false
	}
	g.applySaveSettings(t.Buf, path)
	g.applySaveCleanups(t)
	if err := t.Buf.SaveFile(path); err != nil {
		g.reportError(i18n.Format("editorGroup.savePathFailed", i18n.Args{"path": path, "error": saveErrorText(err)}))
		return false
	}
	if t.Undo != nil {
		t.Undo.MarkSaved()
	}
	// The target remains open through the source tab, so don't send OnFileClose.
	for _, replaceTab := range slices.Backward(replaceTabs) {
		g.releaseTabResources(g.tabs[replaceTab])
		g.tabs = append(g.tabs[:replaceTab], g.tabs[replaceTab+1:]...)
		if replaceTab < g.pinnedCount {
			g.pinnedCount--
		}
		if replaceTab < g.active {
			g.active--
		}
	}
	t = g.activeTab()
	g.movePathState(t.FilePath, path)
	t.FilePath = path
	t.IconPath = path
	t.Virtual = false
	t.Placeholder = false
	g.rebuildHighlighter(t)
	g.adoptSaveAsViews(t, path)
	g.syncTabs()
	g.SyncDocumentViews()
	return true
}

func (g *EditorGroupWidget) SaveAsTargets(path string) ([]int, error) {
	var targets []int
	for i := range g.tabs {
		other := &g.tabs[i]
		if i == g.active || other.Virtual || other.Buf == nil || other.Buf == g.activeTab().Buf {
			continue
		}
		same, err := sameSaveTarget(other.FilePath, path)
		if err != nil {
			return nil, errors.New(i18n.Format("editorGroup.saveDestinationFailed", i18n.Args{"filename": filepath.Base(path), "error": err.Error()}))
		}
		if !same {
			continue
		}
		if other.Buf.Dirty {
			return nil, errors.New(i18n.Format("editorGroup.saveDestinationDirty", i18n.Args{"filename": filepath.Base(path)}))
		}
		targets = append(targets, i)
	}
	for _, peer := range g.relatedGroups() {
		if peer == g {
			continue
		}
		for _, other := range peer.tabs {
			if other.Buf == nil || other.Virtual || other.Buf == g.activeTab().Buf {
				continue
			}
			same, err := sameSaveTarget(other.FilePath, path)
			if err != nil {
				return nil, errors.New(i18n.Format("editorGroup.saveDestinationFailed", i18n.Args{"filename": filepath.Base(path), "error": err.Error()}))
			}
			if same && other.Buf.Dirty {
				return nil, errors.New(i18n.Format("editorGroup.saveDestinationDirty", i18n.Args{"filename": filepath.Base(path)}))
			}
		}
	}
	return targets, nil
}

func sameSaveTarget(a, b string) (bool, error) {
	resolve := func(path string) (string, error) {
		// Cleaning link/.. before resolving symlinks can change the destination.
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			return filepath.Abs(resolved)
		}
		if errors.Is(err, syscall.ENOTDIR) {
			return "", nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if entry, statErr := os.Lstat(path); statErr == nil && entry.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("unresolved symlink %s: %w", path, err)
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		dir, name := filepath.Split(path)
		if dir == "" {
			dir = "."
		}
		parent, err := filepath.EvalSymlinks(dir)
		// SaveFile creates only the leaf, so a missing parent cannot become an alias.
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		return filepath.Abs(filepath.Join(parent, name))
	}
	a, errA := resolve(a)
	b, errB := resolve(b)
	if (errA == nil && a == "") || (errB == nil && b == "") {
		return false, nil
	}
	if err := errors.Join(errA, errB); err != nil {
		return false, err
	}
	if a == b {
		return true, nil
	}
	if !strings.EqualFold(a, b) {
		return false, nil
	}
	dirA, err := os.Stat(filepath.Dir(a))
	if err != nil {
		return false, err
	}
	dirB, err := os.Stat(filepath.Dir(b))
	if err != nil || !os.SameFile(dirA, dirB) {
		return false, err
	}
	if filepath.Base(a) == filepath.Base(b) {
		return true, nil
	}
	return caseInsensitiveDir(filepath.Dir(b))
}

func caseInsensitiveDir(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		name := strings.ToUpper(entry.Name())
		if name == entry.Name() {
			name = strings.ToLower(name)
		}
		if name == entry.Name() {
			continue
		}
		if slices.ContainsFunc(entries, func(e os.DirEntry) bool { return e.Name() == name }) {
			return false, nil
		}
		original, err := entry.Info()
		if err != nil {
			return false, err
		}
		variant, err := os.Lstat(filepath.Join(dir, name))
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return os.SameFile(original, variant), nil
	}
	return false, fmt.Errorf("no entry reveals case sensitivity in %s", dir)
}

// movePathState moves the bookmarks and decorations kept for oldPath, or for
// the files under it, to newPath.
func (g *EditorGroupWidget) movePathState(oldPath, newPath string) {
	prefix := oldPath + string(filepath.Separator)
	remap := func(p string) (string, bool) {
		switch {
		case p == oldPath:
			return newPath, true
		case oldPath != "" && strings.HasPrefix(p, prefix):
			return filepath.Join(newPath, strings.TrimPrefix(p, prefix)), true
		}
		return "", false
	}
	g.moveDecorations(remap)
	if p, ok := remap(g.bookmarkSyncPath); ok {
		g.bookmarkSyncPath = p
	}
	moved := map[string]map[int]Bookmark{}
	for p, b := range g.bookmarksByPath {
		if np, ok := remap(p); ok {
			delete(g.bookmarksByPath, p)
			moved[np] = b
		}
	}
	maps.Copy(g.bookmarksByPath, moved)
}

// RenamePath repoints open tabs after a path is renamed on disk. oldPath may be
// a file, matched exactly, or a folder, in which case every tab beneath it moves
// with it. Without this a renamed file's tab keeps pointing at the path it no
// longer occupies, so the next save writes the old name back out as a duplicate.
//
// Buffer, cursor, selection and undo history are deliberately preserved: the
// document did not change, only its name. Path-derived state (the syntax
// highlighter, and the language server's notion of which document this is) is
// rebuilt, since a rename can change the extension.
func (g *EditorGroupWidget) RenamePath(oldPath, newPath string) bool {
	if oldPath == "" || newPath == "" || oldPath == newPath {
		return false
	}
	g.movePathState(oldPath, newPath)
	prefix := oldPath + string(filepath.Separator)
	remapPath := func(path string) (string, bool) {
		switch {
		case path == oldPath:
			return newPath, true
		case strings.HasPrefix(path, prefix):
			return filepath.Join(newPath, strings.TrimPrefix(path, prefix)), true
		default:
			return "", false
		}
	}
	renamed := false
	for i := range g.tabs {
		t := &g.tabs[i]
		if t.Virtual {
			continue
		}
		if t.Content != nil {
			oldIconPath := t.IconPath
			updatedIconPath, ok := remapPath(oldIconPath)
			if !ok {
				continue
			}
			setter, ok := t.Content.(interface{ SetFilePath(string) })
			if !ok {
				continue
			}
			setter.SetFilePath(updatedIconPath)
			t.IconPath = updatedIconPath
			if updated, ok := remapPath(t.FilePath); ok {
				t.FilePath = updated
			} else if strings.HasPrefix(t.FilePath, oldIconPath+" (") {
				t.FilePath = updatedIconPath + strings.TrimPrefix(t.FilePath, oldIconPath)
			}
			renamed = true
			continue
		}
		if t.Buf == nil {
			continue
		}
		updated, ok := remapPath(t.FilePath)
		if !ok {
			continue
		}
		if g.OnFileClose != nil && t.DetectedLanguage != "" {
			g.OnFileClose(t.FilePath, t.DetectedLanguage)
		}
		t.FilePath = updated
		if t.Diff != nil {
			t.Diff.SetFilePath(updated)
		}
		if t.IconPath != "" {
			t.IconPath = updated
		}
		g.rebuildHighlighter(t)
		if g.OnFileOpen != nil && t.DetectedLanguage != "" {
			g.OnFileOpen(updated, t.DetectedLanguage, t.Buf.Lines)
		}
		g.syncDocumentViews(t)
		renamed = true
	}
	if renamed {
		g.syncTabs()
	}
	return renamed
}
