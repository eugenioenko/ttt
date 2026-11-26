package ui

import (
	"context"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/cursor"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/core/multicursor"
	"github.com/eugenioenko/ttt/internal/core/selection"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/widgets"
	"log/slog"
	"slices"

	"github.com/gdamore/tcell/v3"
)

type editorTab struct {
	ID       string
	FilePath string
	// PluginKey identifies a plugin's tab. It is kept apart from FilePath so a
	// file or buffer at the same path is never taken for the plugin's tab.
	PluginKey   string
	IconPath    string
	Title       string
	Buf         *buffer.Buffer
	Cur         *cursor.Cursor
	Vp          *view.Viewport
	Undo        *undo.UndoStack
	Sel         *selection.Selection
	autoClosing *autoClosingState
	Multi       *multicursor.MultiCursor
	Highlighter *highlight.Highlighter
	Diagnostics []Diagnostic
	Folds       *fold.State
	TabSize     int
	UseTabs     bool
	Content     Widget
	Preview     bool
	Virtual     bool
	Placeholder bool
	LineChanges []diff.LineChangeKind
	ReadOnly    bool

	LanguageSelection string
	// Keep document lifecycle and language servers independent of the highlighting selection.
	DetectedLanguage string
	// Diff shows the tab's own buffer as an editable diff against its base.
	Diff *DiffEditorWidget
	// Annotations shows the decorations for the tab's file when it is not
	// showing a diff.
	Annotations    *DiffOverlay
	viewVersion    uint64
	viewText       string
	viewAnchorText string
}

type EditorGroupWidget struct {
	BaseWidget
	TabBar            *TabBarWidget
	Editor            *EditorPaneWidget
	Autocomplete      *AutocompleteWidget
	Hover             *HoverWidget
	SignatureHelp     *SignatureHelpWidget
	tabs              []editorTab
	nextTabIdentity   uint64
	active            int
	lastDocumentPath  string
	pinnedCount       int
	closedTabs        *closedTabHistory
	TabSize           int
	InsertSpaces      bool
	LineNumbers       bool
	IndentGuides      bool
	FullLineSelection bool
	GutterStyle       string
	WordWrap          bool
	PreviewTabs       bool
	WordWrapColumn    int
	WordWrapBreak     string
	LanguageOverrides map[string]config.EditorLanguageOverride
	AutoSurround      config.AutoSurroundSettings
	WordWrapMaxBytes  int64
	// editorInDiff is set while the shared editor is bound to an editable
	// diff, which sets its word wrap from the diff's own wrap mode.
	editorInDiff            bool
	DiffMode                DiffMode
	DiffContext             DiffContextMode
	DiffWordWrap            bool
	MarkdownMaxWidth        int
	DiffHighContrast        bool
	DiffCollapsedEmphasis   bool
	DiffScrollbarGitMarks   bool
	LegacyScrollbarGlyphs   bool
	DiffSigns               bool
	DiffSignsColor          bool
	DiffGutterHighlight     bool
	DiffColorGutterNumbers  bool
	ChevronCollapsed        rune
	ChevronExpanded         rune
	ImageProtocol           string
	ImageCellW              int
	ImageCellH              int
	ImageRedraw             func()
	SyntaxHighlight         bool
	BracketPairColorization bool
	BracketColorStyles      []term.Style
	InsertFinalNewline      bool
	ShowTrailingNewline     bool
	TrimTrailingWhitespace  bool
	UndoDeleteCursorStart   bool
	HighlightActiveBorder   bool
	Borders                 *term.BorderSet
	OnFileOpen              func(path, lang string, lines []string)
	OnFileChange            func(path, lang, text string)
	OnFileClose             func(path, lang string)
	OnContentTabClose       func(id string)
	// FileHandler may claim a file that would open as the binary card; nil keeps the card.
	FileHandler   func(path string, head []byte) Widget
	OnFileHandled func(path string)
	// OnEmpty fires after the last tab closes and the untitled placeholder
	// takes its place.
	OnEmpty           func()
	OnActiveTabChange func()
	OnActivate        func()
	OpenFileElsewhere func(string) bool
	DropTarget        bool
	DropZone          EditorDropZone
	// RelatedGroups includes this group and all peers sharing its buffers.
	RelatedGroups   func() []*EditorGroupWidget
	lastActiveTabID string
	lastActiveBuf   *buffer.Buffer
	// EmptyStateID names a content tab that stands in for the placeholder:
	// like it, it has no close button while it is the only tab.
	EmptyStateID  string
	OnError       func(msg string)
	OnNotify      func(msg string)
	pendingNotify []string
	focused       bool
	find          findSession
	// diagSources holds diagnostics keyed by source ("lsp:<server>", "plugin:<name>")
	// then by file path. Merged per-path into each tab's Diagnostics.
	diagSources map[string]map[string][]Diagnostic
	// decoSources holds each source's decorations by file path.
	decoSources map[string]map[string]*decoSource
	// bookmarksByPath holds bookmarks keyed by file path so they survive a
	// preview tab being replaced (its editorTab struct is discarded entirely).
	bookmarksByPath   map[string]map[int]Bookmark
	bookmarkSyncPath  string
	OnBookmarkChanged func(path string, line int, action string, icon rune, style term.Style)
	OnGutterClick     func(path string, line int) bool
	LinkWorkDir       func() string
	ControlModifiers  *tcell.ModMask
	NativeLinks       bool
	OnOpenURL         func(string)
	OnOpenFileLink    func(string, int, int)
	OnLinkHover       func(*EditorPaneWidget)
	ResolveFileLinks  FileLinkResolver
	// OnDiagnosticsChanged fires whenever any source's diagnostics change, so
	// the Diagnostics panel can rebuild from DiagnosticsByPath().
	OnDiagnosticsChanged func()

	// FoldTask computes an opened file's folds after its first frame, and a
	// large file's folds after an edit once FoldDelay passes without another;
	// without them they are computed on the next frame.
	FoldTask  *asynctask.Task[[]fold.Range]
	FoldDelay *asynctask.Task[struct{}]
	foldBuf   *buffer.Buffer
	foldGen   int
}

// FoldRecomputeDelay lets a run of Enters in a large file recompute folds once.
const FoldRecomputeDelay = 150 * time.Millisecond

func NewEditorGroupWidget(borders *term.BorderSet, tabSize int, lineNumbers bool, gutterStyle string) *EditorGroupWidget {
	editor := NewEditorPaneWidget(
		&buffer.Buffer{Lines: []string{""}},
		&cursor.Cursor{},
		&view.Viewport{},
	)
	editor.TabSize = tabSize
	editor.LineNumbers = lineNumbers
	editor.GutterStyle = gutterStyle

	tabBar := NewTabBarWidget()
	tabBar.Borders = borders
	tabBar.MoreButton = NewMoreButtonWidget()

	g := &EditorGroupWidget{
		TabBar:      tabBar,
		Editor:      editor,
		TabSize:     tabSize,
		LineNumbers: lineNumbers,
		GutterStyle: gutterStyle,
		Borders:     borders,
		closedTabs:  &closedTabHistory{},

		DiffSignsColor:      true,
		DiffGutterHighlight: true,
		MarkdownMaxWidth:    80,
		PreviewTabs:         true,
	}
	tabBar.OnTabClick = func(index int) {
		g.SwitchTab(index)
	}
	tabBar.OnTabReorder = func(from, to int) {
		g.MoveTab(from, to)
	}
	tabBar.NormalizeDropTarget = g.NormalizeTabMoveTarget
	tabBar.OnNextTab = func() { g.NextTab() }
	tabBar.OnPrevTab = func() { g.PrevTab() }
	tabBar.OnDoubleClick = func() { g.NewFile() }
	editor.OnBookmarkChange = func(line int, action string, b Bookmark) {
		if g.OnBookmarkChanged != nil {
			g.OnBookmarkChanged(g.ActiveFilePath(), line, action, b.Icon, b.Style)
		}
	}
	editor.OnGutterClick = func(line int) bool {
		return g.OnGutterClick != nil && g.OnGutterClick(g.ActiveFilePath(), line)
	}
	undoStack := g.newUndoStack()
	sel := &selection.Selection{}
	editor.Undo = undoStack
	editor.Selection = sel
	g.tabs = []editorTab{{
		FilePath:    "untitled",
		Buf:         editor.Buf,
		Cur:         editor.Cursor,
		Vp:          editor.Viewport,
		Undo:        undoStack,
		Sel:         sel,
		Virtual:     true,
		Placeholder: true,
	}}
	g.syncTabs()
	return g
}

func (g *EditorGroupWidget) Focusable() bool { return true }

func (g *EditorGroupWidget) SetFocused(focused bool) {
	g.focused = focused
	if focused && g.OnActivate != nil {
		g.OnActivate()
	}
	if t := g.activeTab(); t != nil && t.Content != nil {
		if setter, ok := t.Content.(interface{ SetFocused(bool) }); ok {
			setter.SetFocused(focused)
		}
	}
}

func (g *EditorGroupWidget) reportError(msg string) {
	if g.OnError != nil {
		g.OnError(msg)
	} else {
		slog.Error(msg)
	}
}

func (g *EditorGroupWidget) notify(msg string) {
	if g.OnNotify != nil {
		g.OnNotify(msg)
	} else {
		g.pendingNotify = append(g.pendingNotify, msg)
	}
}

func (g *EditorGroupWidget) FlushNotifications() {
	if g.OnNotify == nil {
		return
	}
	for _, msg := range g.pendingNotify {
		g.OnNotify(msg)
	}
	g.pendingNotify = nil
}

func (g *EditorGroupWidget) IsEditorActive() bool {
	t := g.activeTab()
	return t != nil && t.Content == nil
}

func editorCursorVisible(e *EditorPaneWidget) bool {
	x, y := e.CursorX, e.CursorY
	r := e.GetRect()
	if !r.Contains(x, y) || y >= r.Y+e.Viewport.Height {
		return false
	}
	if e.WordWrap {
		return true
	}
	gutter := e.GutterWidth()
	return x >= r.X+gutter && x < r.X+gutter+e.Viewport.Width
}

func (g *EditorGroupWidget) WantsRawKeys() bool {
	rk, ok := g.activeContent().(RawKeyConsumer)
	return ok && rk.WantsRawKeys()
}

func (g *EditorGroupWidget) ActiveTerminalTab() *TerminalTabWidget {
	tt, _ := g.activeContent().(*TerminalTabWidget)
	return tt
}

func (g *EditorGroupWidget) CursorPosition() (int, int, bool) {
	// Content tabs (settings, plugin panels) own their own cursor.
	if t := g.activeTab(); t != nil && t.Content != nil {
		if cp, ok := t.Content.(widgets.CursorPositioner); ok {
			return cp.CursorPosition()
		}
	}
	if g.IsEditorActive() {
		if d := g.activeInlineDiff(); d != nil && d.headFocused() {
			return d.left.CursorX, d.left.CursorY, editorCursorVisible(d.left)
		}
		if g.Editor.isMultiActive() {
			return 0, 0, false
		}
		return g.Editor.CursorX, g.Editor.CursorY, editorCursorVisible(g.Editor)
	}
	return 0, 0, false
}

// ActiveBuffer returns the buffer backing the active tab, or nil if the active
// tab is not a text buffer (e.g. a diff view).
func (g *EditorGroupWidget) ActiveBuffer() *buffer.Buffer {
	t := g.activeTab()
	if t == nil || t.Content != nil {
		return nil
	}
	return t.Buf
}

func (g *EditorGroupWidget) IsDirty() bool {
	t := g.activeTab()
	if t == nil || t.Content != nil {
		return false
	}
	return t.Buf.Dirty
}

func (g *EditorGroupWidget) AnyDirty() bool {
	for _, t := range g.tabs {
		if t.Content != nil {
			continue
		}
		if t.Buf.Dirty {
			return true
		}
	}
	return false
}

// PositionAt maps screen coordinates to a 0-based buffer line/col and the word
// under that position in the active editor. ok is false when the editor is not
// active or the coordinates fall outside the editor content area.
func (g *EditorGroupWidget) PositionAt(mx, my int) (line, col int, word string, ok bool) {
	if !g.IsEditorActive() || g.Editor == nil || g.Editor.Buf == nil {
		return 0, 0, "", false
	}
	r := g.Editor.GetRect()
	if mx < r.X || mx >= r.X+r.W || my < r.Y || my >= r.Y+r.H {
		return 0, 0, "", false
	}
	line, col = g.Editor.mouseToPos(r, mx, my)
	if line < 0 || line >= len(g.Editor.Buf.Lines) {
		return line, col, "", true
	}
	return line, col, wordAt(g.Editor.Buf.Lines[line], col), true
}

func (g *EditorGroupWidget) scheduleFolds() {
	t := g.activeTab()
	// Wrapped panes do not fold, so folds wait until wrap is turned off.
	if t == nil || t.Folds == nil || !t.Folds.Pending() || g.Editor.WordWrap {
		return
	}
	if g.FoldTask == nil || (t.Folds.Stale() && g.FoldDelay == nil) {
		t.Folds.Resolve()
		return
	}
	gen := t.Folds.Generation()
	if g.foldBuf == t.Buf && g.foldGen == gen {
		return
	}
	buf, folds := t.Buf, t.Folds
	g.foldBuf, g.foldGen = buf, gen
	compute := func() {
		lines := slices.Clone(buf.Lines)
		g.FoldTask.Schedule(func(context.Context) ([]fold.Range, error) {
			return fold.ComputeIndentRanges(lines), nil
		}, func(ranges []fold.Range, _ error) {
			g.foldBuf = nil
			// Only edits that add or remove lines start a new generation, so
			// ranges for the same generation and line count are as current as
			// an edit would leave them. Rejecting them on every keystroke would
			// keep folds from landing while the user types.
			if folds.Pending() && folds.Generation() == gen && len(buf.Lines) == len(lines) {
				folds.SetRanges(ranges)
			}
		})
	}
	if !folds.Stale() {
		compute()
		return
	}
	g.FoldDelay.Schedule(func(context.Context) (struct{}, error) {
		return struct{}{}, nil
	}, func(_ struct{}, err error) {
		if err == nil && folds.Pending() && folds.Generation() == gen {
			compute()
		}
	})
}

// ContentRect is the screen rect below the tab bar that tab content fills.
func (g *EditorGroupWidget) ContentRect() Rect {
	r := g.GetRect()
	height := g.TabBar.Height()
	return Rect{X: r.X, Y: r.Y + height, W: r.W, H: max(r.H-height, 0)}
}

func (g *EditorGroupWidget) Render(surface Surface) {
	g.scheduleFolds()
	g.syncTabs()
	w, h := surface.Size()
	r := g.GetRect()
	tabBarH := g.TabBar.Height()

	if w <= 0 || h <= tabBarH {
		g.TabBar.InvalidatePointerInteraction()
		return
	}

	g.TabBar.SetRect(Rect{X: r.X, Y: r.Y, W: r.W, H: tabBarH})
	tabSurface := surface.Sub(Rect{X: 0, Y: 0, W: w, H: tabBarH})
	split := len(g.relatedGroups()) > 1
	g.TabBar.ShowGroupFocus = split && g.HighlightActiveBorder
	g.TabBar.GroupActive = g.focused
	g.Editor.Passive = split && !g.focused
	g.TabBar.Render(tabSurface)
	if g.effectiveDropZone() == EditorDropCenter {
		borders := term.SingleBorderSet()
		if g.Borders != nil {
			borders = *g.Borders
		}
		for x := 0; x < w; x++ {
			tabSurface.SetCell(x, 0, term.Cell{Ch: borders.Horizontal, Style: term.StyleBorderActive})
		}
	}

	contentH := h - tabBarH
	contentRect := g.ContentRect()
	contentSurface := surface.Sub(Rect{X: 0, Y: tabBarH, W: w, H: contentH})

	t := g.activeTab()
	if t == nil {
		return
	}
	if d, ok := t.Content.(diffSignsSurface); ok {
		d.setGutterStyle(g.GutterStyle)
	}
	if d := t.diffWidget(); d != nil {
		for _, p := range d.viewPanes() {
			g.configurePaneLinks(p, d.FilePath)
		}
	}
	if detail, ok := t.Content.(*CommitDetailWidget); ok {
		detail.configurePaneLinks = g.configurePaneLinks
	}
	if t.Content != nil {
		t.Content.SetRect(contentRect)
		t.Content.Render(contentSurface)
	} else if t.Diff != nil {
		t.Diff.setGutterStyle(g.GutterStyle)
		t.Diff.SetRect(contentRect)
		t.Diff.Render(contentSurface)
	} else {
		path := t.FilePath
		if t.Virtual {
			path = ""
		}
		g.configurePaneLinks(g.Editor, path)
		g.Editor.SetRect(contentRect)
		g.Editor.Render(contentSurface)
	}

	if g.SignatureHelp != nil && g.SignatureHelp.Label != "" {
		g.SignatureHelp.AnchorX = g.Editor.CursorX - r.X
		g.SignatureHelp.AnchorY = g.Editor.CursorY - r.Y
		g.SignatureHelp.Borders = g.Borders
		g.SignatureHelp.Render(surface)
	}

	if g.Autocomplete != nil && len(g.Autocomplete.Items) > 0 {
		g.Autocomplete.AnchorX = g.Editor.CursorX - r.X
		g.Autocomplete.AnchorY = g.Editor.CursorY - r.Y
		g.Autocomplete.Borders = g.Borders
		g.Autocomplete.Render(surface)
	}

	if g.Hover != nil && g.Hover.HasContent() {
		g.Hover.OffsetX = r.X
		g.Hover.OffsetY = r.Y
		g.Hover.Borders = g.Borders
		g.Hover.Render(surface)
	}

	g.renderDropPreview(contentSurface)
}

func (g *EditorGroupWidget) CancelPointerCapture() bool {
	canceled := g.TabBar.CancelPointerCapture()
	t := g.activeTab()
	if t != nil {
		if t.Content != nil {
			canceled = widgets.CancelPointerCapture(t.Content) || canceled
		} else if t.Diff != nil {
			canceled = widgets.CancelPointerCapture(t.Diff) || canceled
		}
	}
	if g.Editor != nil {
		canceled = widgets.CancelPointerCapture(g.Editor) || canceled
	}
	return canceled
}

func (g *EditorGroupWidget) OwnsPointerCapture() bool {
	if g.TabBar.OwnsPointerCapture() {
		return true
	}
	return g.contentOwnsPointerCapture()
}

func (g *EditorGroupWidget) PointerGestureActive() bool {
	return g.TabBar.PointerGestureActive()
}

func (g *EditorGroupWidget) contentOwnsPointerCapture() bool {
	t := g.activeTab()
	switch {
	case t == nil:
		return false
	case t.Content != nil:
		owner, ok := t.Content.(widgets.PointerCaptureOwner)
		return ok && owner.OwnsPointerCapture()
	case t.Diff != nil:
		return t.Diff.OwnsPointerCapture()
	}
	return g.Editor != nil && g.Editor.OwnsPointerCapture()
}

func (g *EditorGroupWidget) InvalidatePointerInteraction() bool {
	return g.TabBar.InvalidatePointerInteraction()
}

func (g *EditorGroupWidget) SetPointerCaptureInvalidated(invalidated func()) {
	g.TabBar.SetPointerCaptureInvalidated(invalidated)
}

func (g *EditorGroupWidget) HandleEvent(ev tcell.Event) EventResult {
	if mouse, ok := ev.(*tcell.EventMouse); ok && mouse.Buttons()&(tcell.Button1|tcell.Button2|tcell.Button3) != 0 {
		x, y := mouse.Position()
		if g.GetRect().Contains(x, y) && g.OnActivate != nil {
			g.OnActivate()
		}
	}
	if _, ok := ev.(*tcell.EventMouse); ok && g.TabBar.PointerGestureActive() {
		return g.TabBar.HandleEvent(ev)
	}
	if g.Hover != nil {
		result := g.Hover.HandleEvent(ev)
		if result == EventDismissed {
			g.Hover = nil
		}
		if result == EventConsumed {
			return EventConsumed
		}
	}
	if g.SignatureHelp != nil {
		if kev, ok := ev.(*tcell.EventKey); ok && kev.Key() == tcell.KeyEscape && g.Autocomplete == nil {
			g.SignatureHelp = nil
			return EventConsumed
		}
	}
	if g.Autocomplete != nil {
		result := g.Autocomplete.HandleEvent(ev)
		if result == EventConsumed {
			return EventConsumed
		}
	}
	if _, isMouse := ev.(*tcell.EventMouse); !isMouse || !g.contentOwnsPointerCapture() {
		result := g.TabBar.HandleEvent(ev)
		slog.Debug("editorGroup", "tabBarResult", result)
		if result != EventIgnored {
			return result
		}
	}
	t := g.activeTab()
	if t == nil {
		return EventIgnored
	}
	if t.Content != nil {
		if result := t.Content.HandleEvent(ev); result != EventIgnored {
			return result
		}
		if _, ok := ev.(*tcell.EventMouse); ok {
			return EventConsumed
		}
		return EventIgnored
	}
	var result EventResult
	if t.Diff != nil {
		result = t.Diff.HandleEvent(ev)
	} else {
		result = g.Editor.HandleEvent(ev)
	}
	g.saveMultiState()
	return result
}
