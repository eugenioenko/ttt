package ui

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/view"
)

func contentFileError(path string, content Widget) error {
	var message string
	switch w := content.(type) {
	case *ImageViewWidget:
		message = w.Err
	case *BinaryFileWidget:
		message = w.Err
	}
	if message != "" {
		return errors.New(i18n.Format("files.cannotOpen", i18n.Args{"path": path, "message": message}))
	}
	return nil
}

func (g *EditorGroupWidget) OpenFile(path string) error {
	if g.OpenFileElsewhere != nil && g.OpenFileElsewhere(path) {
		return nil
	}
	if isSpecialFile(path) {
		err := errors.New(i18n.Format("files.cannotOpenSpecialFile", i18n.Args{"path": path}))
		g.reportError(err.Error())
		return err
	}
	for i := range g.tabs {
		if g.tabs[i].FilePath == path {
			if g.tabs[i].Content != nil {
				g.tabs[i].Preview = false
				g.applyImagePrefs(g.tabs[i].Content)
				if refresher, ok := g.tabs[i].Content.(interface{ Refresh() }); ok {
					refresher.Refresh()
				}
				g.SwitchTab(i)
				return contentFileError(path, g.tabs[i].Content)
			}
			if g.tabs[i].Buf != nil && !g.tabs[i].Buf.Dirty {
				if err := g.tabs[i].Buf.LoadFile(path); err != nil && !errors.Is(err, os.ErrNotExist) {
					err = fmt.Errorf("%s: %w", i18n.Format("editorGroup.reloadFileFailed", i18n.Args{"path": path}), err)
					g.reportError(err.Error())
					g.tabs[i].Preview = false
					g.SwitchTab(i)
					return err
				}
			}
			g.tabs[i].Preview = false
			g.SwitchTab(i)
			return nil
		}
	}
	switch kind, head := sniffFileKind(path); kind {
	case fileKindImage:
		content := NewImageViewWidget(path)
		g.openContentFileTab(path, content)
		return contentFileError(path, content)
	case fileKindBinary:
		if g.FileHandler != nil {
			if content := g.FileHandler(path, head); content != nil {
				g.openContentFileTab(path, content)
				g.fileHandled(path)
				return nil
			}
		}
		content := NewBinaryFileWidget(path)
		g.openContentFileTab(path, content)
		return contentFileError(path, content)
	}
	newBuf := &buffer.Buffer{Lines: []string{""}, ShowTrailingNewline: g.ShowTrailingNewline}
	ec := g.applySaveSettings(newBuf, path)
	if err := newBuf.LoadFile(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			err = fmt.Errorf("%s: %w", i18n.Format("editorGroup.openFileFailed", i18n.Args{"path": path}), err)
			g.reportError(err.Error())
			return err
		}
		g.notify(i18n.Format("editorGroup.newFile", i18n.Args{"filename": filepath.Base(path)}))
	}
	tabSize := g.TabSize
	useTabs := !g.InsertSpaces
	detected := buffer.DetectIndent(newBuf.Lines)
	if ec.IndentStyle == "tab" {
		useTabs = true
	} else if ec.IndentStyle == "space" {
		useTabs = false
	} else if detected.UseTabs || detected.Size > 0 {
		useTabs = detected.UseTabs
	}
	if ec.IndentSize > 0 {
		tabSize = ec.IndentSize
	} else if detected.Size > 0 && !detected.UseTabs {
		tabSize = detected.Size
	}
	folds := fold.NewState()
	folds.SetPending(func() []fold.Range { return fold.ComputeIndentRanges(newBuf.Lines) })
	newTab := editorTab{
		FilePath: path,
		IconPath: path,
		Buf:      newBuf,
		Cur:      &cursor.Cursor{},
		Vp:       &view.Viewport{},
		Undo:     g.newUndoStack(),
		Sel:      &selection.Selection{},
		Folds:    folds,
		TabSize:  tabSize,
		UseTabs:  useTabs,
		Preview:  g.PreviewTabs,
	}
	g.rebuildHighlighter(&newTab)
	newTab.Annotations = g.mergedDecorations(path, newBuf, tabSize)
	if t := g.activeTab(); g.canReplaceTab(t) && (isFileViewerTab(t) || (t.Content == nil && t.Buf != nil && !t.Buf.Dirty)) {
		g.notifyContentTabClose(*t)
		g.tabs[g.active] = newTab
		g.syncTabs()
	} else {
		g.tabs = append(g.tabs, newTab)
		g.SwitchTab(len(g.tabs) - 1)
	}
	if g.OnFileOpen != nil && newTab.Highlighter != nil {
		g.OnFileOpen(path, newTab.Highlighter.Language(), newBuf.Lines)
	}
	return nil
}

func isFileViewerTab(t *editorTab) bool {
	switch t.Content.(type) {
	case *ImageViewWidget, *BinaryFileWidget:
		return true
	}
	return false
}

func (g *EditorGroupWidget) applyImagePrefs(content Widget) {
	if iv, ok := content.(*ImageViewWidget); ok {
		iv.Protocol = g.ImageProtocol
		iv.CellW, iv.CellH = g.ImageCellW, g.ImageCellH
		iv.Redraw = g.ImageRedraw
	}
}

const specialFileMask = os.ModeNamedPipe | os.ModeSocket | os.ModeDevice | os.ModeCharDevice | os.ModeIrregular

// Reading a pipe or device blocks until a peer shows up, which freezes the UI thread.
func isSpecialFile(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode()&specialFileMask != 0
}

type fileKind int

const (
	fileKindText fileKind = iota
	fileKindImage
	fileKindBinary
)

// The binary check mirrors isBinaryContent in internal/app/current_changes.go, which this package cannot import without a cycle.
func sniffFileKind(path string) (fileKind, []byte) {
	f, err := os.Open(path)
	if err != nil {
		return fileKindText, nil
	}
	defer func() { _ = f.Close() }()
	var head [512]byte
	n, _ := f.Read(head[:])
	sample := head[:n]
	ct := http.DetectContentType(sample)
	switch ct {
	case "image/png", "image/jpeg", "image/gif":
		return fileKindImage, head[:n]
	}
	if len(sample) == 0 {
		return fileKindText, nil
	}
	// A rune cut by the 512-byte window is not invalid UTF-8.
	if n == len(head) {
		for i := len(sample) - 1; i >= len(sample)-utf8.UTFMax && i >= 0; i-- {
			if utf8.RuneStart(sample[i]) {
				if !utf8.FullRune(sample[i:]) {
					sample = sample[:i]
				}
				break
			}
		}
	}
	if bytes.IndexByte(sample, 0) >= 0 || !utf8.Valid(sample) {
		return fileKindBinary, head[:n]
	}
	if !strings.HasPrefix(ct, "text/") {
		return fileKindBinary, head[:n]
	}
	return fileKindText, nil
}

// RetryFileHandlers offers binary-card tabs to FileHandler again: files opened from the command line load before plugins do.
func (g *EditorGroupWidget) RetryFileHandlers() {
	if g.FileHandler == nil {
		return
	}
	for i := range g.tabs {
		if _, ok := g.tabs[i].Content.(*BinaryFileWidget); !ok {
			continue
		}
		path := g.tabs[i].FilePath
		kind, head := sniffFileKind(path)
		if kind != fileKindBinary {
			continue
		}
		if content := g.FileHandler(path, head); content != nil {
			g.replaceContent(i, content)
			g.fileHandled(path)
		}
	}
	g.syncTabs()
}

func (g *EditorGroupWidget) ShowBinaryCard(path string) {
	for i := range g.tabs {
		if g.tabs[i].FilePath == path && g.tabs[i].Content != nil {
			g.replaceContent(i, NewBinaryFileWidget(path))
		}
	}
	g.syncTabs()
}

func (g *EditorGroupWidget) fileHandled(path string) {
	if g.OnFileHandled != nil {
		g.OnFileHandled(path)
	}
}

func (g *EditorGroupWidget) replaceContent(i int, content Widget) {
	old := g.tabs[i].Content
	if setter, ok := old.(interface{ SetFocused(bool) }); ok {
		setter.SetFocused(false)
	}
	if closer, ok := old.(interface{ Close() }); ok {
		closer.Close()
	}
	g.tabs[i].Content = content
	g.tabs[i].PluginKey = contentPluginKey(content)
	if i == g.active && g.focused {
		if setter, ok := content.(interface{ SetFocused(bool) }); ok {
			setter.SetFocused(true)
		}
	}
}

// Content tabs carry no buffer.
func (g *EditorGroupWidget) openContentFileTab(path string, content Widget) {
	g.applyImagePrefs(content)
	newTab := editorTab{
		FilePath:  path,
		PluginKey: contentPluginKey(content),
		IconPath:  path,
		Content:   content,
		Preview:   g.PreviewTabs,
	}
	if t := g.activeTab(); g.canReplaceTab(t) && (t.Content != nil || (t.Buf != nil && !t.Buf.Dirty)) {
		g.notifyContentTabClose(*t)
		g.tabs[g.active] = newTab
		g.syncTabs()
		return
	}
	g.tabs = append(g.tabs, newTab)
	g.SwitchTab(len(g.tabs) - 1)
}

func (g *EditorGroupWidget) NameUntitledPlaceholder() {
	if t := g.activeTab(); g.isUntouchedPlaceholder(t) {
		t.FilePath = g.nextUntitledName(t)
		g.syncTabs()
	}
}

func (g *EditorGroupWidget) NewFile() {
	name := g.nextUntitledName(nil)
	g.tabs = append(g.tabs, editorTab{
		FilePath: name,
		Buf:      &buffer.Buffer{Lines: []string{""}},
		Cur:      &cursor.Cursor{},
		Vp:       &view.Viewport{},
		Undo:     g.newUndoStack(),
		Sel:      &selection.Selection{},
		Virtual:  true,
		TabSize:  g.TabSize,
		UseTabs:  !g.InsertSpaces,
	})
	g.SwitchTab(len(g.tabs) - 1)
}

func (g *EditorGroupWidget) NewFileFromText(text string) {
	buf := &buffer.Buffer{ShowTrailingNewline: g.ShowTrailingNewline}
	buf.SetText(text)
	tab := editorTab{
		Buf:     buf,
		Cur:     &cursor.Cursor{},
		Vp:      &view.Viewport{},
		Undo:    g.newUndoStack(),
		Sel:     &selection.Selection{},
		Virtual: true,
		TabSize: g.TabSize,
		UseTabs: !g.InsertSpaces,
	}
	if t := g.activeTab(); len(g.tabs) == 1 && g.isUntouchedPlaceholder(t) {
		tab.FilePath = t.FilePath
		g.tabs[g.active] = tab
		g.syncTabs()
		return
	}
	tab.FilePath = g.nextUntitledName(nil)
	g.tabs = append(g.tabs, tab)
	g.SwitchTab(len(g.tabs) - 1)
}

func (g *EditorGroupWidget) nextUntitledName(exclude *editorTab) string {
	taken := make(map[string]bool)
	for _, group := range append([]*EditorGroupWidget{g}, g.relatedGroups()...) {
		for i := range group.tabs {
			t := &group.tabs[i]
			if t != exclude && t.Virtual {
				taken[t.FilePath] = true
			}
		}
	}
	if !taken["untitled"] {
		return "untitled"
	}
	for n := 2; ; n++ {
		name := fmt.Sprintf("untitled-%d", n)
		if !taken[name] {
			return name
		}
	}
}

func (g *EditorGroupWidget) OpenFileReadOnly(path, title string) error {
	for i := range g.tabs {
		if g.tabs[i].FilePath == path && g.tabs[i].ReadOnly {
			g.SwitchTab(i)
			return nil
		}
	}
	newBuf := &buffer.Buffer{Lines: []string{""}}
	if err := newBuf.LoadFile(path); err != nil {
		g.reportError(i18n.Format("editorGroup.failedToOpen", i18n.Args{"path": path, "error": err}))
		return err
	}
	tabSize := g.TabSize
	detected := buffer.DetectIndent(newBuf.Lines)
	if detected.Size > 0 && !detected.UseTabs {
		tabSize = detected.Size
	}
	folds := fold.NewState()
	folds.SetRanges(fold.ComputeIndentRanges(newBuf.Lines))
	tabTitle := title
	if tabTitle == "" {
		tabTitle = filepath.Base(path)
	}
	newTab := editorTab{
		FilePath: path,
		IconPath: path,
		Title:    tabTitle,
		Buf:      newBuf,
		Cur:      &cursor.Cursor{},
		Vp:       &view.Viewport{},
		Undo:     g.newUndoStack(),
		Sel:      &selection.Selection{},
		Folds:    folds,
		TabSize:  tabSize,
		UseTabs:  detected.UseTabs,
		ReadOnly: true,
	}
	g.rebuildHighlighter(&newTab)
	g.tabs = append(g.tabs, newTab)
	g.SwitchTab(len(g.tabs) - 1)
	return nil
}

func (g *EditorGroupWidget) OpenBufferReadOnly(title, filePath string, lines []string) {
	for i := range g.tabs {
		if g.tabs[i].Title == title && g.tabs[i].ReadOnly {
			g.tabs[i].Buf.SetLines(lines)
			g.SwitchTab(i)
			return
		}
	}
	newBuf := &buffer.Buffer{Lines: lines}
	if len(lines) == 0 {
		newBuf.SetLines([]string{""})
	}
	tabSize := g.TabSize
	detected := buffer.DetectIndent(newBuf.Lines)
	if detected.Size > 0 && !detected.UseTabs {
		tabSize = detected.Size
	}
	folds := fold.NewState()
	folds.SetRanges(fold.ComputeIndentRanges(newBuf.Lines))
	newTab := editorTab{
		FilePath: filePath,
		IconPath: filePath,
		Title:    title,
		Buf:      newBuf,
		Cur:      &cursor.Cursor{},
		Vp:       &view.Viewport{},
		Undo:     g.newUndoStack(),
		Sel:      &selection.Selection{},
		Folds:    folds,
		TabSize:  tabSize,
		UseTabs:  detected.UseTabs,
		ReadOnly: true,
	}
	g.rebuildHighlighter(&newTab)
	g.tabs = append(g.tabs, newTab)
	g.SwitchTab(len(g.tabs) - 1)
}

func (g *EditorGroupWidget) ReloadFile(path string) bool {
	for i := range g.tabs {
		if g.tabs[i].FilePath == path && g.tabs[i].Buf != nil {
			if err := g.tabs[i].Buf.LoadFile(path); err != nil {
				g.reportError(i18n.Format("editorGroup.failedToReload", i18n.Args{"path": path, "error": err}))
				return false
			}
			g.tabs[i].Buf.Dirty = false
			g.tabs[i].Undo = g.newUndoStack()
			if g.tabs[i].Folds != nil {
				g.tabs[i].Folds.SetRanges(fold.ComputeIndentRanges(g.tabs[i].Buf.Lines))
			}
			g.clampCursor(&g.tabs[i])
			if g.tabs[i].Highlighter != nil {
				g.tabs[i].Highlighter.SyncBuffer(g.tabs[i].Buf)
			}
			g.syncDocumentViews(&g.tabs[i])
			if i == g.active {
				g.syncTabs()
			}
			return true
		}
	}
	return false
}

// clampCursor keeps a tab's cursor within the bounds of its buffer, which may
// have shrunk after an external reload.
func (g *EditorGroupWidget) clampCursor(t *editorTab) {
	if t.Cur == nil || t.Buf == nil {
		return
	}
	n := len(t.Buf.Lines)
	if n == 0 {
		t.Cur.Line, t.Cur.Col = 0, 0
		return
	}
	if t.Cur.Line >= n {
		t.Cur.Line = n - 1
	}
	if t.Cur.Line < 0 {
		t.Cur.Line = 0
	}
	lineLen := len([]rune(t.Buf.Lines[t.Cur.Line]))
	if t.Cur.Col > lineLen {
		t.Cur.Col = lineLen
	}
}

// OpenFilePaths returns the paths of all tabs backed by a real file on disk
// (excluding untitled buffers and non-text content like diff views). The order
// is unspecified.
func (g *EditorGroupWidget) OpenFilePaths() []string {
	var paths []string
	for i := range g.tabs {
		t := &g.tabs[i]
		if t.Content != nil || t.Buf == nil {
			continue
		}
		if t.FilePath == "" || t.Virtual {
			continue
		}
		paths = append(paths, t.FilePath)
	}
	return paths
}

type OpenDocument struct {
	Path     string
	Language string
	Text     string
}

func (g *EditorGroupWidget) OpenDocuments() []OpenDocument {
	var docs []OpenDocument
	for i := range g.tabs {
		t := &g.tabs[i]
		if t.Content != nil || t.Buf == nil || t.DetectedLanguage == "" || t.Virtual || t.FilePath == "" {
			continue
		}
		docs = append(docs, OpenDocument{
			Path:     t.FilePath,
			Language: t.DetectedLanguage,
			Text:     strings.Join(t.Buf.Lines, "\n"),
		})
	}
	return docs
}

// OpenFiles is OpenDocuments without joining each buffer's text.
func (g *EditorGroupWidget) OpenFiles() []OpenDocument {
	var docs []OpenDocument
	for i := range g.tabs {
		t := &g.tabs[i]
		if t.Content != nil || t.Buf == nil || t.DetectedLanguage == "" || t.Virtual || t.FilePath == "" {
			continue
		}
		docs = append(docs, OpenDocument{Path: t.FilePath, Language: t.DetectedLanguage})
	}
	return docs
}

// BufferForPath returns the buffer of the tab with the given path, or nil.
func (g *EditorGroupWidget) BufferForPath(path string) *buffer.Buffer {
	for i := range g.tabs {
		if g.tabs[i].FilePath == path {
			return g.tabs[i].Buf
		}
	}
	return nil
}

// IsDirtyPath reports whether the tab with the given path has unsaved changes.
func (g *EditorGroupWidget) IsDirtyPath(path string) bool {
	b := g.BufferForPath(path)
	return b != nil && b.Dirty
}

func (g *EditorGroupWidget) ActiveFilePath() string {
	if t := g.activeTab(); t != nil {
		return t.FilePath
	}
	return ""
}

// RecentDocumentPath is the active tab's file or, when the active tab is not a
// file on disk (an untitled buffer, a plugin or terminal tab), the file tab
// that was active last, while it is still open.
func (g *EditorGroupWidget) RecentDocumentPath() string {
	if path := documentPath(g.activeTab()); path != "" {
		return path
	}
	for i := range g.tabs {
		if path := documentPath(&g.tabs[i]); path != "" && path == g.lastDocumentPath {
			return path
		}
	}
	return ""
}

func documentPath(t *editorTab) string {
	if t == nil || t.Content != nil || t.Buf == nil || t.Virtual {
		return ""
	}
	return t.FilePath
}

func (g *EditorGroupWidget) IsActiveVirtual() bool {
	if t := g.activeTab(); t != nil {
		return t.Virtual
	}
	return true
}

func (g *EditorGroupWidget) IsActiveReadOnly() bool {
	if t := g.activeTab(); t != nil {
		return t.ReadOnly
	}
	return false
}

func (g *EditorGroupWidget) ActiveFileName() string {
	t := g.activeTab()
	if t == nil {
		return "untitled"
	}
	return filepath.Base(t.FilePath)
}

func (g *EditorGroupWidget) IsActiveEditableFile(path string) bool {
	t := g.activeTab()
	return t != nil && t.FilePath == path && t.PluginKey == "" && t.Content == nil &&
		t.Buf != nil && !t.Virtual && !t.Placeholder && t.Diff == nil && !t.ReadOnly
}
