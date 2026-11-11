package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/ui"
)

const maxResponseBody = 10 * 1024 * 1024 // 10 MB

// PluginEditorAPI implements plugin.EditorAPI using the App's editor state.
type PluginEditorAPI struct {
	eg *ui.EditorGroupWidget
}

func NewPluginEditorAPI(app *App) *PluginEditorAPI {
	return &PluginEditorAPI{eg: app.EditorGroup}
}

func (e *PluginEditorAPI) BufferText() string {
	buf := e.eg.ActiveBuffer()
	if buf == nil {
		return ""
	}
	return strings.Join(buf.Lines, "\n")
}

func (e *PluginEditorAPI) BufferLines() []string {
	buf := e.eg.ActiveBuffer()
	if buf == nil {
		return nil
	}
	result := make([]string, len(buf.Lines))
	copy(result, buf.Lines)
	return result
}

func (e *PluginEditorAPI) CurrentLine() string {
	buf := e.eg.ActiveBuffer()
	if buf == nil {
		return ""
	}
	line, _ := e.eg.ActiveCursor()
	if line >= 0 && line < len(buf.Lines) {
		return buf.Lines[line]
	}
	return ""
}

func (e *PluginEditorAPI) GetLine(n int) string {
	buf := e.eg.ActiveBuffer()
	if buf == nil || n < 0 || n >= len(buf.Lines) {
		return ""
	}
	return buf.Lines[n]
}

func (e *PluginEditorAPI) LineCount() int {
	buf := e.eg.ActiveBuffer()
	if buf == nil {
		return 0
	}
	return len(buf.Lines)
}

func (e *PluginEditorAPI) CursorPos() (int, int) {
	return e.eg.ActiveCursor()
}

func (e *PluginEditorAPI) Selection() (bool, int, int, int, int) {
	ed := e.eg.Editor
	if ed == nil || ed.Selection == nil || !ed.Selection.Active {
		return false, 0, 0, 0, 0
	}
	line, col := e.eg.ActiveCursor()
	start, end := ed.Selection.Range(line, col)
	return true, start.Line, start.Col, end.Line, end.Col
}

func (e *PluginEditorAPI) SelectionText() string {
	ed := e.eg.Editor
	if ed == nil || ed.Selection == nil || !ed.Selection.Active || ed.Buf == nil {
		return ""
	}
	line, col := e.eg.ActiveCursor()
	return ed.Selection.Text(ed.Buf.Lines, line, col)
}

func (e *PluginEditorAPI) FilePath() string {
	return e.eg.ActiveFilePath()
}

func (e *PluginEditorAPI) FileName() string {
	return e.eg.ActiveFileName()
}

func (e *PluginEditorAPI) Language() string {
	ed := e.eg.Editor
	if ed == nil || ed.Highlighter == nil {
		return ""
	}
	return ed.Highlighter.Language()
}

func (e *PluginEditorAPI) Viewport() (int, int, int) {
	ed := e.eg.Editor
	if ed == nil || ed.Viewport == nil {
		return 0, 0, 0
	}
	top := ed.Viewport.TopLine
	height := ed.Viewport.Height
	bottom := top + height - 1
	buf := e.eg.ActiveBuffer()
	if buf != nil && bottom >= len(buf.Lines) {
		bottom = len(buf.Lines) - 1
	}
	return top, bottom, height
}

func (e *PluginEditorAPI) ScrollTo(line int) {
	ed := e.eg.Editor
	if ed == nil || ed.Viewport == nil {
		return
	}
	if line < 0 {
		line = 0
	}
	ed.Viewport.TopLine = line
}

func (e *PluginEditorAPI) ScrollBy(delta int) {
	ed := e.eg.Editor
	if ed == nil || ed.Viewport == nil {
		return
	}
	newTop := ed.Viewport.TopLine + delta
	if newTop < 0 {
		newTop = 0
	}
	ed.Viewport.TopLine = newTop
}

func (e *PluginEditorAPI) SetLine(line int, text string) {
	ed := e.eg.Editor
	if ed == nil || ed.Buf == nil || ed.Undo == nil {
		return
	}
	if line < 0 || line >= len(ed.Buf.Lines) {
		return
	}
	runes := []rune(ed.Buf.Lines[line])
	e.Replace(line, 0, line, len(runes), text)
}

func (e *PluginEditorAPI) Insert(line, col int, text string) {
	ed := e.eg.Editor
	if ed == nil || ed.Buf == nil || ed.Undo == nil {
		return
	}
	if line < 0 || line >= len(ed.Buf.Lines) || col < 0 {
		return
	}
	runes := []rune(ed.Buf.Lines[line])
	if col > len(runes) {
		col = len(runes)
	}

	if !strings.Contains(text, "\n") {
		cmd := &undo.InsertStringCommand{Line: line, Col: col, Text: text}
		cmd.Apply(ed.Buf)
		ed.Undo.Push(cmd)
	} else {
		cmd := &undo.PasteCommand{Line: line, Col: col, Text: text, Suffix: string(runes[col:])}
		cmd.Apply(ed.Buf)
		ed.Undo.Push(cmd)
	}
	ed.FlushOnChange()
}

func (e *PluginEditorAPI) Replace(startLine, startCol, endLine, endCol int, text string) {
	ed := e.eg.Editor
	if ed == nil || ed.Buf == nil || ed.Undo == nil {
		return
	}
	if startLine < 0 || startLine >= len(ed.Buf.Lines) || endLine < startLine || startCol < 0 || endCol < 0 {
		return
	}
	if startLine == endLine && endCol < startCol {
		return
	}

	delCmd := &undo.DeleteSelectionCommand{
		StartLine: startLine, StartCol: startCol,
		EndLine: endLine, EndCol: endCol,
	}
	if delCmd.ComputeDeleted(ed.Buf) == text {
		return
	}
	delCmd.Apply(ed.Buf)

	if text == "" {
		ed.Undo.Push(delCmd)
		ed.FlushOnChange()
		return
	}

	col := delCmd.StartCol
	var insertCmd undo.EditCommand
	if !strings.Contains(text, "\n") {
		insertCmd = &undo.InsertStringCommand{Line: startLine, Col: col, Text: text}
	} else {
		suffix := string([]rune(ed.Buf.Lines[startLine])[col:])
		insertCmd = &undo.PasteCommand{Line: startLine, Col: col, Text: text, Suffix: suffix}
	}
	insertCmd.Apply(ed.Buf)

	batch := &undo.BatchCommand{Commands: []undo.EditCommand{delCmd, insertCmd}}
	ed.Undo.Push(batch)
	ed.FlushOnChange()
}

func (e *PluginEditorAPI) SetCursor(line, col int) {
	ed := e.eg.Editor
	if ed == nil || ed.Cursor == nil {
		return
	}
	ed.Cursor.Line = line
	ed.Cursor.Col = col
	ed.EnsureCursorVisible()
}

func (e *PluginEditorAPI) SetSelection(startLine, startCol, endLine, endCol int) {
	ed := e.eg.Editor
	if ed == nil || ed.Selection == nil || ed.Cursor == nil {
		return
	}
	ed.Selection.Start(startLine, startCol)
	ed.Cursor.Line = endLine
	ed.Cursor.Col = endCol
}

func (e *PluginEditorAPI) ClearSelection() {
	ed := e.eg.Editor
	if ed == nil || ed.Selection == nil {
		return
	}
	ed.Selection.Clear()
}

func (e *PluginEditorAPI) BeginUndoGroup() {
	ed := e.eg.Editor
	if ed == nil || ed.Undo == nil {
		return
	}
	ed.Undo.BeginTransaction()
}

func (e *PluginEditorAPI) EndUndoGroup() {
	ed := e.eg.Editor
	if ed == nil || ed.Undo == nil {
		return
	}
	ed.Undo.EndTransaction()
}

func (e *PluginEditorAPI) AddCursor(line, col int) {
	e.eg.AddCursor(line, col)
}

func (e *PluginEditorAPI) GetCursors() []plugin.CursorPosition {
	states := e.eg.GetCursors()
	if states == nil {
		line, col := e.eg.ActiveCursor()
		return []plugin.CursorPosition{{Line: line, Col: col}}
	}
	result := make([]plugin.CursorPosition, len(states))
	for i, cs := range states {
		result[i] = plugin.CursorPosition{Line: cs.Line, Col: cs.Col}
	}
	return result
}

func (e *PluginEditorAPI) ClearCursors() {
	e.eg.CollapseMultiCursor()
}

func (e *PluginEditorAPI) SetSearch(pattern string, useRegex bool) {
	if !e.eg.IsEditorActive() {
		return
	}
	buf := e.eg.ActiveBuffer()
	if buf == nil {
		return
	}
	opts := ui.SearchOptions{UseRegex: useRegex, CaseSensitive: true}
	matches, _ := ui.FindInLines(buf.Lines, pattern, opts)
	e.eg.SetSearch(pattern, opts, matches)
}

func (e *PluginEditorAPI) ClearSearch() {
	e.eg.ClearSearch()
}

func (e *PluginEditorAPI) DiffHunks() []plugin.DiffHunkTarget {
	hunks, _, goTo := activeDiffHunks(e.eg)
	if goTo == nil {
		return nil
	}
	// An active diff with no change blocks returns an empty slice, not nil,
	// so a plugin can tell "no diff here" from "a diff with no changes".
	out := make([]plugin.DiffHunkTarget, len(hunks))
	for i, h := range hunks {
		out[i] = plugin.DiffHunkTarget{FilePath: h.FilePath, Kind: string(h.Kind)}
	}
	return out
}

func (e *PluginEditorAPI) DiffHunkAt() int {
	_, at, _ := activeDiffHunks(e.eg)
	return at
}

func (e *PluginEditorAPI) GoToDiffHunk(index int) error {
	hunks, _, goTo := activeDiffHunks(e.eg)
	if goTo == nil {
		return fmt.Errorf("the active tab is not a diff")
	}
	if index < 1 || index > len(hunks) {
		return fmt.Errorf("hunk %d out of range (1-%d)", index, len(hunks))
	}
	return goTo(index)
}

// PluginFilesystemAPI implements plugin.FilesystemAPI with path restrictions.
type PluginFilesystemAPI struct {
	read      fsRoots
	write     fsRoots
	vars      plugin.FsVars
	onWrite   func(path string)
	mu        sync.RWMutex
	closed    bool
	closeOnce sync.Once
	closeErr  error
}

type fsGrantKind uint8

const (
	fsDirectoryGrant fsGrantKind = iota
	fsExactFileGrant
	fsPendingGrant
)

type fsRoot struct {
	path            string
	canonical       string
	fileLeaf        string
	pendingRelative string
	kind            fsGrantKind
	root            *os.Root
	err             error
}

type fsRoots struct {
	roots []fsRoot
	all   bool
}

type fsRootAnchor struct {
	root *os.Root
	info os.FileInfo
	err  error
}

type fsRootCache struct {
	anchorsByPath map[string]fsRootAnchor
}

func (cache *fsRootCache) rootForGrant(path string) fsRoot {
	grant, err := describeGrantedPath(path)
	root := fsRoot{
		canonical:       grant.canonical,
		fileLeaf:        grant.fileLeaf,
		pendingRelative: grant.pendingRelative,
		kind:            grant.kind,
		err:             err,
	}
	if err == nil {
		anchor := cache.rootForAnchor(path, grant)
		root.root = anchor.root
		root.err = anchor.err
		if root.err == nil && grant.kind == fsExactFileGrant && grant.fileInfo != nil {
			root.err = verifyExactFileGrant(root.root, path, grant.fileLeaf, grant.fileInfo)
		}
	}
	return root
}

func (cache *fsRootCache) rootForAnchor(path string, grant fsGrantPath) fsRootAnchor {
	anchor, ok := cache.anchorsByPath[grant.anchor]
	if !ok {
		anchor.root, anchor.err = openGrantedRoot(path, grant.anchor, grant.anchorInfo)
		if anchor.err == nil {
			anchor.info = grant.anchorInfo
		}
		cache.anchorsByPath[grant.anchor] = anchor
		return anchor
	}
	if anchor.err == nil && !os.SameFile(grant.anchorInfo, anchor.info) {
		anchor.err = fmt.Errorf("granted root %q changed while sharing", path)
	}
	return anchor
}

const pluginRPCDialTimeout = 10 * time.Second

func dialPluginRPC(vars plugin.FsVars) func(context.Context, string) (io.ReadWriteCloser, error) {
	return func(ctx context.Context, endpoint string) (io.ReadWriteCloser, error) {
		ctx, cancel := context.WithTimeout(ctx, pluginRPCDialTimeout)
		defer cancel()
		var d net.Dialer
		if path, ok := strings.CutPrefix(endpoint, plugin.UnixEndpointPrefix); ok {
			return d.DialContext(ctx, "unix", vars.Expand(path))
		}
		return d.DialContext(ctx, "tcp", endpoint)
	}
}

func NewPluginFilesystemAPI(allowedRoots ...string) *PluginFilesystemAPI {
	roots := newFsRootsCached(allowedRoots, false, &fsRootCache{})
	return &PluginFilesystemAPI{read: roots, write: roots}
}

func NewPluginFilesystemAPIFor(p *plugin.Plugin, workspace []string) *PluginFilesystemAPI {
	vars := plugin.FsVars{Workspace: workspace, Config: config.UserConfigDir()}
	if home, err := os.UserHomeDir(); err == nil {
		vars.Home = home
	}
	cache := &fsRootCache{}
	roots := func(access plugin.FsAccess) fsRoots {
		dirs, all := access.Resolve(vars)
		if p.Dir != "" {
			dirs = append(dirs, p.Dir)
		}
		return newFsRootsCached(dirs, all, cache)
	}
	return &PluginFilesystemAPI{read: roots(p.Granted.FsRead), write: roots(p.Granted.FsWrite), vars: vars}
}

func newFsRootsCached(dirs []string, all bool, cache *fsRootCache) fsRoots {
	if all {
		return fsRoots{all: true}
	}
	if cache.anchorsByPath == nil {
		cache.anchorsByPath = make(map[string]fsRootAnchor)
	}
	roots := make([]fsRoot, 0, len(dirs)*2)
	for _, path := range dirs {
		abs, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		abs = filepath.Clean(abs)
		root := cache.rootForGrant(abs)
		root.path = abs
		roots = append(roots, root)
		// openRoot resolves symlinks in requested paths, so keep both the granted path and
		// resolved path mapped to same handle (macOS /tmp and /var are symlinks).
		if root.canonical != abs {
			root.path = root.canonical
			roots = append(roots, root)
		}
	}
	return fsRoots{roots: roots, all: all}
}

type fsGrantPath struct {
	canonical       string
	anchor          string
	fileLeaf        string
	fileInfo        os.FileInfo
	pendingRelative string
	kind            fsGrantKind
	anchorInfo      os.FileInfo
}

func describeGrantedPath(path string) (fsGrantPath, error) {
	canonical, info, err := canonicalGrantedRoot(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fsGrantPath{canonical: canonical}, err
		}
		if _, lstatErr := os.Lstat(path); lstatErr == nil {
			return fsGrantPath{canonical: canonical}, err
		}
		parent := filepath.Dir(path)
		if resolvedParent, resolveErr := filepath.EvalSymlinks(parent); resolveErr == nil && resolvedParent != parent {
			canonical = filepath.Join(resolvedParent, filepath.Base(path))
			anchorInfo, statErr := os.Stat(resolvedParent)
			if statErr != nil {
				return fsGrantPath{canonical: canonical}, statErr
			}
			if !anchorInfo.IsDir() {
				return fsGrantPath{canonical: canonical}, fmt.Errorf("granted file parent %q is not a directory", resolvedParent)
			}
			return fsGrantPath{
				canonical:  canonical,
				anchor:     resolvedParent,
				fileLeaf:   filepath.Base(canonical),
				kind:       fsExactFileGrant,
				anchorInfo: anchorInfo,
			}, nil
		}
		anchor, pendingRelative, anchorInfo, anchorErr := nearestGrantAnchor(path)
		if anchorErr != nil {
			return fsGrantPath{canonical: canonical}, anchorErr
		}
		if filepath.IsAbs(pendingRelative) || pendingRelative == "." || pendingRelative == ".." || strings.HasPrefix(pendingRelative, ".."+string(filepath.Separator)) {
			return fsGrantPath{canonical: canonical}, fmt.Errorf("invalid pending grant path %q", path)
		}
		return fsGrantPath{
			canonical:       path,
			anchor:          anchor,
			pendingRelative: pendingRelative,
			kind:            fsPendingGrant,
			anchorInfo:      anchorInfo,
		}, nil
	}
	if info != nil && info.IsDir() {
		return fsGrantPath{canonical: canonical, anchor: canonical, kind: fsDirectoryGrant, anchorInfo: info}, nil
	}
	anchor := filepath.Dir(canonical)
	anchorInfo, err := os.Stat(anchor)
	if err != nil {
		return fsGrantPath{canonical: canonical}, err
	}
	if !anchorInfo.IsDir() {
		return fsGrantPath{canonical: canonical}, fmt.Errorf("granted file parent %q is not a directory", anchor)
	}
	return fsGrantPath{
		canonical:  canonical,
		anchor:     anchor,
		fileLeaf:   filepath.Base(canonical),
		fileInfo:   info,
		kind:       fsExactFileGrant,
		anchorInfo: anchorInfo,
	}, nil
}

func nearestGrantAnchor(path string) (string, string, os.FileInfo, error) {
	for candidate := filepath.Dir(path); ; candidate = filepath.Dir(candidate) {
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			canonical, resolveErr := filepath.EvalSymlinks(candidate)
			if resolveErr != nil {
				return "", "", nil, resolveErr
			}
			canonicalInfo, statErr := os.Stat(canonical)
			if statErr != nil {
				return "", "", nil, statErr
			}
			if !canonicalInfo.IsDir() || !os.SameFile(info, canonicalInfo) {
				return "", "", nil, fmt.Errorf("granted directory %q changed while resolving", candidate)
			}
			pendingRelative, relErr := filepath.Rel(candidate, path)
			if relErr != nil {
				return "", "", nil, relErr
			}
			return canonical, pendingRelative, canonicalInfo, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", "", nil, err
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return "", "", nil, fmt.Errorf("no existing directory anchor for grant %q", path)
		}
	}
}

func verifyExactFileGrant(root *os.Root, path, leaf string, expected os.FileInfo) error {
	if expected == nil {
		return fmt.Errorf("missing observed file identity for grant %q", path)
	}
	actual, err := root.Lstat(leaf)
	if err != nil {
		return err
	}
	if actual.Mode()&os.ModeSymlink != 0 || !os.SameFile(expected, actual) {
		return fmt.Errorf("granted file %q changed while opening", path)
	}
	return nil
}

func canonicalGrantedRoot(path string) (string, os.FileInfo, error) {
	original, err := os.Stat(path)
	if err != nil {
		return path, nil, err
	}
	canonical := path
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		canonical = resolved
	}
	return canonical, original, nil
}

func openGrantedRoot(path, canonical string, original os.FileInfo) (*os.Root, error) {
	root, err := os.OpenRoot(canonical)
	if err != nil {
		return nil, err
	}
	opened, err := root.Stat(".")
	if err == nil && !os.SameFile(original, opened) {
		err = fmt.Errorf("granted root %q changed while opening", path)
	}
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	return root, nil
}

func openGrantedChildRoot(parent *os.Root, component string, observed os.FileInfo) (*os.Root, error) {
	root, err := parent.OpenRoot(component)
	if err != nil {
		return nil, err
	}
	opened, err := root.Stat(".")
	if err == nil && !os.SameFile(observed, opened) {
		err = fmt.Errorf("granted path component %q changed while opening", component)
	}
	if err != nil {
		_ = root.Close()
		return nil, err
	}
	return root, nil
}

func resolveDanglingSymlink(path string) (string, error) {
	seen := make(map[string]struct{})
	for {
		if resolved, err := filepath.EvalSymlinks(path); err == nil {
			return resolved, nil
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			parent, err := filepath.EvalSymlinks(filepath.Dir(path))
			if err != nil {
				return "", err
			}
			return filepath.Join(parent, filepath.Base(path)), nil
		}
		if _, ok := seen[path]; ok {
			return "", fmt.Errorf("symlink loop at %q", path)
		}
		seen[path] = struct{}{}
		target, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		path = filepath.Clean(target)
	}
}

type resolvedFSPath struct {
	root      *os.Root
	relative  string
	path      string
	kind      fsGrantKind
	temporary bool
}

func (grant fsRoot) openPendingRoot(relative, path string) (resolvedFSPath, error) {
	components := strings.Split(filepath.ToSlash(grant.pendingRelative), "/")
	current := grant.root
	owned := false
	cleanup := true
	defer func() {
		if cleanup && owned {
			_ = current.Close()
		}
	}()

	for i, component := range components {
		if component == "" || component == "." || component == ".." {
			return resolvedFSPath{}, fmt.Errorf("invalid pending grant component %q", component)
		}
		observed, err := current.Lstat(component)
		if err != nil {
			if i == len(components)-1 && errors.Is(err, os.ErrNotExist) {
				if relative != "." {
					return resolvedFSPath{}, fmt.Errorf("access denied: pending grant %q is not a directory", grant.path)
				}
				cleanup = false
				return resolvedFSPath{root: current, relative: component, path: path, kind: fsExactFileGrant, temporary: owned}, nil
			}
			return resolvedFSPath{}, err
		}
		if observed.Mode()&os.ModeSymlink != 0 {
			return resolvedFSPath{}, fmt.Errorf("access denied: pending grant component %q is a symlink", component)
		}
		if i != len(components)-1 && !observed.IsDir() {
			return resolvedFSPath{}, fmt.Errorf("pending grant component %q is not a directory", component)
		}
		if i == len(components)-1 && !observed.IsDir() {
			if relative != "." {
				return resolvedFSPath{}, fmt.Errorf("access denied: pending grant %q is not a directory", grant.path)
			}
			cleanup = false
			return resolvedFSPath{root: current, relative: component, path: path, kind: fsExactFileGrant, temporary: owned}, nil
		}

		next, err := openGrantedChildRoot(current, component, observed)
		if err != nil {
			return resolvedFSPath{}, err
		}
		if owned {
			if err := current.Close(); err != nil {
				_ = next.Close()
				return resolvedFSPath{}, err
			}
		}
		current, owned = next, true
		if i == len(components)-1 {
			cleanup = false
			return resolvedFSPath{root: current, relative: relative, path: path, kind: fsDirectoryGrant, temporary: true}, nil
		}
	}
	return resolvedFSPath{}, fmt.Errorf("invalid pending grant %q", grant.path)
}

func (r fsRoots) openRoot(path string) (resolvedFSPath, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return resolvedFSPath{}, fmt.Errorf("invalid path: %w", err)
	}
	original := filepath.Clean(abs)
	clean := original
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		clean = resolved
	} else if info, err := os.Lstat(clean); err == nil && info.Mode()&os.ModeSymlink != 0 {
		if !r.all {
			// A dangling symlink: writing through it would create its target, wherever that is.
			return resolvedFSPath{}, fmt.Errorf("access denied: path %q is a dangling symlink", path)
		}
		clean, err = resolveDanglingSymlink(clean)
		if err != nil {
			return resolvedFSPath{}, fmt.Errorf("invalid path: %w", err)
		}
	} else if resolved, err := filepath.EvalSymlinks(filepath.Dir(clean)); err == nil {
		clean = filepath.Join(resolved, filepath.Base(clean))
	}

	if r.all {
		volumeRoot := string(os.PathSeparator)
		if volume := filepath.VolumeName(clean); volume != "" {
			volumeRoot = volume + string(os.PathSeparator)
		}
		relative, err := filepath.Rel(volumeRoot, clean)
		if err != nil || filepath.IsAbs(relative) {
			return resolvedFSPath{}, fmt.Errorf("invalid path: %q", path)
		}
		root, err := os.OpenRoot(volumeRoot)
		if err != nil {
			return resolvedFSPath{}, err
		}
		return resolvedFSPath{root: root, relative: relative, path: clean, temporary: true}, nil
	}

	findRoot := func(candidate string) (*fsRoot, string) {
		var selected *fsRoot
		selectedRelative := ""
		for i := range r.roots {
			root := &r.roots[i]
			relative, err := filepath.Rel(root.path, candidate)
			if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				continue
			}
			if root.kind == fsExactFileGrant {
				if relative != "." {
					continue
				}
				relative = root.fileLeaf
			}
			if selected == nil || len(root.path) < len(selected.path) {
				selected, selectedRelative = root, relative
			}
		}
		return selected, selectedRelative
	}

	selectedRoot, relative := findRoot(clean)
	resolved := clean
	if selectedRoot == nil {
		selectedRoot, relative = findRoot(original)
		resolved = original
	}
	if selectedRoot == nil {
		return resolvedFSPath{}, fmt.Errorf("access denied: path %q is outside allowed directories", path)
	}
	if selectedRoot.err != nil {
		return resolvedFSPath{}, selectedRoot.err
	}
	if selectedRoot.kind == fsPendingGrant {
		return selectedRoot.openPendingRoot(relative, resolved)
	}
	return resolvedFSPath{root: selectedRoot.root, relative: relative, path: resolved, kind: selectedRoot.kind}, nil
}

func (f *PluginFilesystemAPI) Close() error {
	f.closeOnce.Do(func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.closed = true
		seen := make(map[*os.Root]struct{})
		var errs []error
		for _, roots := range []fsRoots{f.read, f.write} {
			for _, root := range roots.roots {
				if root.root == nil {
					continue
				}
				if _, ok := seen[root.root]; ok {
					continue
				}
				seen[root.root] = struct{}{}
				if err := root.root.Close(); err != nil {
					errs = append(errs, err)
				}
			}
		}
		f.closeErr = errors.Join(errs...)
	})
	return f.closeErr
}

func (p resolvedFSPath) openExactFile(flag int) (*os.File, error) {
	if flag != os.O_RDONLY && flag != os.O_WRONLY {
		return nil, fmt.Errorf("unsupported exact file open flags %d", flag)
	}
	info, err := p.root.Lstat(p.relative)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("access denied: exact file %q is a symlink", p.path)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("access denied: exact file %q is a directory", p.path)
	}
	file, err := p.root.OpenFile(p.relative, flag, 0)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		_ = file.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("exact file %q changed while opening", p.path)
	}
	return file, nil
}

func (p resolvedFSPath) openExactFileForWrite() (*os.File, error) {
	file, err := p.openExactFile(os.O_WRONLY)
	if !errors.Is(err, os.ErrNotExist) {
		return file, err
	}
	file, err = p.root.OpenFile(p.relative, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return p.openExactFile(os.O_WRONLY)
	}
	return file, err
}

func (f *PluginFilesystemAPI) ReadFile(path string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.closed {
		return "", os.ErrClosed
	}
	path = f.vars.Expand(path)
	resolved, err := f.read.openRoot(path)
	if err != nil {
		return "", err
	}
	if resolved.temporary {
		defer resolved.root.Close()
	}
	if resolved.kind == fsExactFileGrant {
		file, err := resolved.openExactFile(os.O_RDONLY)
		if err != nil {
			return "", err
		}
		data, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil {
			return "", readErr
		}
		if closeErr != nil {
			return "", closeErr
		}
		return string(data), nil
	}
	data, err := resolved.root.ReadFile(resolved.relative)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *PluginFilesystemAPI) WriteFile(path, content string) error {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.closed {
		return os.ErrClosed
	}
	path = f.vars.Expand(path)
	resolved, err := f.write.openRoot(path)
	if err != nil {
		return err
	}
	if resolved.kind == fsExactFileGrant {
		file, openErr := resolved.openExactFileForWrite()
		if openErr != nil {
			err = openErr
		} else {
			err = file.Truncate(0)
			if err == nil {
				_, err = file.Seek(0, io.SeekStart)
			}
			if err == nil {
				var written int
				written, err = io.WriteString(file, content)
				if err == nil && written != len(content) {
					err = io.ErrShortWrite
				}
			}
			if closeErr := file.Close(); err == nil {
				err = closeErr
			}
		}
	} else {
		err = resolved.root.WriteFile(resolved.relative, []byte(content), 0644)
	}
	var closeErr error
	if resolved.temporary {
		closeErr = resolved.root.Close()
	}
	if err != nil {
		return err
	}
	if f.onWrite != nil {
		f.onWrite(resolved.path)
	}
	return closeErr
}

func (f *PluginFilesystemAPI) FileExists(path string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.closed {
		return false
	}
	path = f.vars.Expand(path)
	resolved, err := f.read.openRoot(path)
	if err != nil {
		return false
	}
	if resolved.temporary {
		defer resolved.root.Close()
	}
	if resolved.kind == fsExactFileGrant {
		info, err := resolved.root.Lstat(resolved.relative)
		return err == nil && info.Mode()&os.ModeSymlink == 0
	}
	_, err = resolved.root.Stat(resolved.relative)
	return err == nil
}

func (f *PluginFilesystemAPI) ListDir(path string) ([]plugin.FileEntry, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.closed {
		return nil, os.ErrClosed
	}
	path = f.vars.Expand(path)
	resolved, err := f.read.openRoot(path)
	if err != nil {
		return nil, err
	}
	if resolved.temporary {
		defer resolved.root.Close()
	}
	var directory *os.File
	if resolved.kind == fsExactFileGrant {
		directory, err = resolved.openExactFile(os.O_RDONLY)
	} else {
		directory, err = resolved.root.Open(resolved.relative)
	}
	if err != nil {
		return nil, err
	}
	entries, err := directory.ReadDir(-1)
	closeErr := directory.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	result := make([]plugin.FileEntry, len(entries))
	for i, entry := range entries {
		result[i] = plugin.FileEntry{Name: entry.Name(), IsDir: entry.IsDir()}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// PluginSystemAPI implements plugin.SystemAPI.
type PluginSystemAPI struct {
	onExec func()
}

// PluginTerminalAPI adapts the App's terminal tabs to the plugin
// `ttt.terminal` module (terminal.control / terminal.read).
type PluginTerminalAPI struct {
	app *App
}

func NewPluginTerminalAPI(a *App) *PluginTerminalAPI {
	return &PluginTerminalAPI{app: a}
}

func (t *PluginTerminalAPI) TerminalPalette() (string, map[string]string) {
	if t.app.TerminalPalette == nil {
		return "unavailable", map[string]string{}
	}
	return t.app.TerminalPalette.Snapshot()
}

func (t *PluginTerminalAPI) RequestTerminalPalette() {
	if t.app.TerminalPalette != nil {
		t.app.TerminalPalette.Request()
	}
}

func (t *PluginTerminalAPI) TerminalSpawn(spec plugin.TerminalSpawnSpec) (string, error) {
	return t.app.SpawnTerminalSpec(TerminalSpec{
		Shell:    spec.Shell,
		Args:     spec.Args,
		Env:      spec.Env,
		Cwd:      spec.Cwd,
		Label:    spec.Label,
		InEditor: spec.Location == plugin.TerminalLocationEditor,
	})
}

func (t *PluginTerminalAPI) TerminalSend(id, text string) error {
	return t.app.SendToTerminal(id, text)
}

func (t *PluginTerminalAPI) TerminalTail(id string, maxBytes int) (string, error) {
	return t.app.TerminalTail(id, maxBytes)
}

func (t *PluginTerminalAPI) TerminalScreen(id string) (string, error) {
	return t.app.TerminalScreen(id)
}

func (t *PluginTerminalAPI) TerminalFocus(id string) error {
	return t.app.FocusTerminal(id)
}

func (t *PluginTerminalAPI) TerminalClose(id string) error {
	for _, info := range t.app.ListTerminals() {
		if info.ID == id {
			t.app.CloseTerminal(id)
			return nil
		}
	}
	return fmt.Errorf("terminal %q not found", id)
}

func (t *PluginTerminalAPI) TerminalList() []plugin.TerminalInfo {
	return t.app.ListTerminals()
}

func NewPluginSystemAPI() *PluginSystemAPI {
	return &PluginSystemAPI{}
}

var dangerousGitArgPatterns = []string{
	"--upload-pack", "--receive-pack",
	"--exec=", "--config=", "--config-env",
	"core.fsmonitor", "core.sshCommand", "core.pager",
	"diff.external", "merge.tool",
}

// validateArgs guards git only: git runs a config value starting with "!"
// (an alias or helper set through -c) as a shell command. Other binaries get
// their arguments as plain argv with no shell, so "=!" is ordinary text there.
func (s *PluginSystemAPI) validateArgs(binary string, args []string) error {
	if !plugin.IsGitBinary(binary) {
		return nil
	}
	for i, arg := range args {
		if strings.Contains(arg, "=!") {
			return fmt.Errorf("argument %d contains command injection pattern", i)
		}
		for _, pattern := range dangerousGitArgPatterns {
			if strings.Contains(arg, pattern) {
				return fmt.Errorf("argument %d contains blocked pattern %q", i, pattern)
			}
		}
	}
	return nil
}

// pluginExecWaitDelay bounds how long a grandchild can hold the output pipes
// open after the child exits or is killed, and so delay the result.
const pluginExecWaitDelay = time.Second

func (s *PluginSystemAPI) Exec(ctx context.Context, binary string, args []string, opts plugin.ExecOptions) (string, string, int, error) {
	if err := s.validateArgs(binary, args); err != nil {
		return "", "", -1, err
	}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.WaitDelay = pluginExecWaitDelay
	var stdout, stderr bytes.Buffer
	outW, errW := io.Writer(&stdout), io.Writer(&stderr)
	if opts.Stdout != nil {
		outW = opts.Stdout
	}
	if opts.Stderr != nil {
		errW = opts.Stderr
	}
	if opts.Stdin != "" {
		cmd.Stdin = strings.NewReader(opts.Stdin)
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}
	err := runPiped(cmd, outW, errW)
	if s.onExec != nil {
		s.onExec()
	}
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			err = nil
		}
	}
	return stdout.String(), stderr.String(), exitCode, err
}

func (s *PluginSystemAPI) Env(name string) string {
	return os.Getenv(name)
}

// pluginSpawnedProcess adapts *exec.Cmd to plugin.SpawnedProcess.
type pluginSpawnedProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func (p *pluginSpawnedProcess) Write(data []byte) (int, error) { return p.stdin.Write(data) }
func (p *pluginSpawnedProcess) CloseStdin() error              { return p.stdin.Close() }
func (p *pluginSpawnedProcess) Kill() error                    { return p.cmd.Process.Kill() }
func (p *pluginSpawnedProcess) PID() int                       { return p.cmd.Process.Pid }
func (p *pluginSpawnedProcess) Stdout() io.Reader              { return p.stdout }
func (p *pluginSpawnedProcess) Stderr() io.Reader              { return p.stderr }

func (p *pluginSpawnedProcess) Wait() (int, error) {
	err := p.cmd.Wait()
	if err == nil {
		return 0, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), nil
	}
	return -1, err
}

// Spawn starts binary with raw piped stdio for interactive use (agents
// speaking JSON-RPC over stdio, REPLs, streaming tools). Same argument
// validation as Exec applies.
func (s *PluginSystemAPI) Spawn(binary string, args []string, opts plugin.SpawnOptions) (plugin.SpawnedProcess, error) {
	if err := s.validateArgs(binary, args); err != nil {
		return nil, err
	}
	cmd := exec.Command(binary, args...)
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, err
	}
	return &pluginSpawnedProcess{cmd: cmd, stdin: stdin, stdout: stdout, stderr: stderr}, nil
}

// PluginNetworkAPI implements plugin.NetworkAPI. Where a plugin may connect
// is its network.http grant's business; this layer only enforces the scheme
// and holds redirects to the grant.
type PluginNetworkAPI struct {
	client *http.Client
	// allow is the plugin's network.http check. The plugin host applies it
	// to the URL a plugin passes; redirects are checked here.
	allow func(*url.URL) error
}

func NewPluginNetworkAPI() *PluginNetworkAPI {
	n := &PluginNetworkAPI{}
	n.client = &http.Client{Timeout: 30 * time.Second, CheckRedirect: n.checkRedirect}
	return n
}

// ForPlugin returns a copy whose redirects are also held to allow.
func (n *PluginNetworkAPI) ForPlugin(allow func(*url.URL) error) plugin.NetworkAPI {
	c := &PluginNetworkAPI{allow: allow}
	c.client = &http.Client{Timeout: n.client.Timeout, Transport: n.client.Transport, CheckRedirect: c.checkRedirect}
	return c
}

// checkRedirect holds every redirect to the checks the first URL passed;
// without it a listed host could send a plugin anywhere.
func (n *PluginNetworkAPI) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	if err := n.validateURL(req.URL.String()); err != nil {
		return err
	}
	if n.allow != nil {
		return n.allow(req.URL)
	}
	return nil
}

func (n *PluginNetworkAPI) validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL scheme %q not allowed, only http and https", u.Scheme)
	}
	return nil
}

func (n *PluginNetworkAPI) Get(url string, headers map[string]string) (int, string, map[string]string, error) {
	if err := n.validateURL(url); err != nil {
		return 0, "", nil, err
	}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, "", nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := n.client.Do(req)
	if err != nil {
		return 0, "", nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return resp.StatusCode, "", nil, err
	}
	respHeaders := make(map[string]string)
	for k := range resp.Header {
		respHeaders[k] = resp.Header.Get(k)
	}
	return resp.StatusCode, string(body), respHeaders, nil
}

func (n *PluginNetworkAPI) Post(url string, headers map[string]string, body string) (int, string, map[string]string, error) {
	if err := n.validateURL(url); err != nil {
		return 0, "", nil, err
	}
	req, err := http.NewRequest("POST", url, strings.NewReader(body))
	if err != nil {
		return 0, "", nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := n.client.Do(req)
	if err != nil {
		return 0, "", nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return resp.StatusCode, "", nil, err
	}
	respHeaders := make(map[string]string)
	for k := range resp.Header {
		respHeaders[k] = resp.Header.Get(k)
	}
	return resp.StatusCode, string(respBody), respHeaders, nil
}

type PluginSettingsAPI struct {
	app *App
}

func NewPluginSettingsAPI(app *App) *PluginSettingsAPI {
	return &PluginSettingsAPI{app: app}
}

func (s *PluginSettingsAPI) Get(key string) (any, bool) {
	m := s.settingsMap()
	if m == nil {
		return nil, false
	}
	return getPath(m, strings.Split(key, "."))
}

func (s *PluginSettingsAPI) Set(key string, value any) error {
	m := s.settingsMap()
	if m == nil {
		m = make(map[string]any)
	}
	setPath(m, strings.Split(key, "."), value)

	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, s.app.Settings); err != nil {
		return err
	}
	return config.SaveSettings(*s.app.Settings)
}

func (s *PluginSettingsAPI) settingsMap() map[string]any {
	data, err := json.Marshal(s.app.Settings)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	return m
}

func getPath(m map[string]any, parts []string) (any, bool) {
	for i, part := range parts {
		val, ok := m[part]
		if !ok {
			return nil, false
		}
		if i == len(parts)-1 {
			return val, true
		}
		child, ok := val.(map[string]any)
		if !ok {
			return nil, false
		}
		m = child
	}
	return nil, false
}

func setPath(m map[string]any, parts []string, value any) {
	for i, part := range parts {
		if i == len(parts)-1 {
			if value == nil {
				delete(m, part)
			} else {
				m[part] = value
			}
			return
		}
		child, ok := m[part].(map[string]any)
		if !ok {
			child = make(map[string]any)
			m[part] = child
		}
		m = child
	}
}
