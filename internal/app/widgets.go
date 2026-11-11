package app

import (
	"errors"
	"github.com/eugenioenko/ttt/internal/highlight"
	"github.com/eugenioenko/ttt/internal/i18n"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/fold"
	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"
	"github.com/eugenioenko/ttt/internal/widgets"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func isPRURL(arg string) bool {
	return strings.Contains(arg, "github.com/") && strings.Contains(arg, "/pull/")
}

// FileTarget is a file named on the command line, with the optional 1-based
// cursor position parsed from a `path:line[:col]` argument. Line and Col are 0
// when the argument carried no position.
type FileTarget struct {
	Path string
	Line int
	Col  int
}

// splitLineCol splits a `path:line[:col]` argument into its path and 1-based
// line and column. Trailing colons are tolerated so text pasted straight from
// tools like `grep -n` ("main.go:42:") works. At most two trailing numeric
// fields are consumed, which leaves Windows paths such as `C:\src\main.go`
// alone because the drive letter is not followed by a number. ok is false when
// the argument carries no positional suffix.
func splitLineCol(arg string) (path string, line, col int, ok bool) {
	parts := strings.Split(strings.TrimRight(arg, ":"), ":")
	if len(parts) < 2 {
		return arg, 0, 0, false
	}
	var nums []int
	end := len(parts)
	for end > 1 && len(nums) < 2 {
		n, err := strconv.Atoi(parts[end-1])
		if err != nil || n < 1 {
			break
		}
		nums = append([]int{n}, nums...)
		end--
	}
	if len(nums) == 0 {
		return arg, 0, 0, false
	}
	path = strings.Join(parts[:end], ":")
	if path == "" {
		return arg, 0, 0, false
	}
	line = nums[0]
	if len(nums) > 1 {
		col = nums[1]
	}
	return path, line, col, true
}

// resolveLineColArg interprets arg as `path:line[:col]` and reports the target
// only when the stripped path actually names an existing file. Requiring the
// file to exist keeps ambiguous arguments — a new file whose name contains a
// colon and a number — behaving as they did before.
func resolveLineColArg(arg string) (FileTarget, bool) {
	path, line, col, ok := splitLineCol(arg)
	if !ok {
		return FileTarget{}, false
	}
	abs, err := filepath.Abs(workspace.ExpandPath(path))
	if err != nil {
		return FileTarget{}, false
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return FileTarget{}, false
	}
	return FileTarget{Path: abs, Line: line, Col: col}, true
}

type openArgs struct {
	folders       []string
	files         []FileTarget
	workspaceFile string
	configFile    string
	prURLs        []string
	welcome       bool
}

func parseOpenArgs() openArgs {
	var parsed openArgs

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		if args[i] == "--workspace" && i+1 < len(args) {
			parsed.workspaceFile = args[i+1]
			i++
			continue
		}
		if args[i] == "--config" && i+1 < len(args) {
			parsed.configFile = args[i+1]
			i++
			continue
		}
		if args[i] == "--exec" && i+1 < len(args) {
			i++
			continue
		}
		if args[i] == "--exec-split-on" && i+1 < len(args) {
			i++
			continue
		}
		if args[i] == "--plugin" && i+1 < len(args) {
			i++
			continue
		}
		if args[i] == "--size" && i+1 < len(args) {
			i++
			continue
		}
		if args[i] == "--new-window" {
			continue
		}
		if args[i] == "--debug" {
			continue
		}
		if args[i] == "--listen" {
			continue
		}
		if args[i] == "-" {
			continue
		}
		if args[i] == "--welcome" {
			parsed.welcome = true
			continue
		}
		if isPRURL(args[i]) {
			if _, _, _, err := github.ParsePRURL(args[i]); err == nil {
				parsed.prURLs = append(parsed.prURLs, args[i])
			}
			continue
		}
		absPath, err := filepath.Abs(workspace.ExpandPath(args[i]))
		if err != nil {
			parsed.files = append(parsed.files, FileTarget{Path: args[i]})
			continue
		}
		info, err := os.Stat(absPath)
		if err != nil {
			// Nothing exists at the path as written. Try it as `path:line[:col]`
			// before falling back to creating a new file, so `ttt main.go:42`
			// opens main.go at line 42 instead of a file named "main.go:42".
			// A file that genuinely contains a colon still wins, because the
			// stat above is attempted first.
			if target, ok := resolveLineColArg(args[i]); ok {
				parsed.files = append(parsed.files, target)
				continue
			}
			parsed.files = append(parsed.files, FileTarget{Path: absPath})
			continue
		}
		if info.IsDir() {
			parsed.folders = append(parsed.folders, absPath)
		} else {
			parsed.files = append(parsed.files, FileTarget{Path: absPath})
		}
	}
	return parsed
}

func resolveArgs(welcomeOnHome bool) (ws *workspace.Workspace, openFiles []FileTarget, configFile string, prURLs []string) {
	parsed := parseOpenArgs()
	folders := parsed.folders
	openFiles, configFile, prURLs = parsed.files, parsed.configFile, parsed.prURLs
	if parsed.workspaceFile != "" {
		loaded, err := workspace.LoadFile(parsed.workspaceFile)
		if err == nil {
			ws = loaded
			for _, f := range folders {
				ws.AddFolder(f)
			}
			return
		}
	}

	// Opening only files intentionally creates no workspace — a folder must be passed explicitly.
	// --welcome, or starting in $HOME with welcome.showOnHome, opens no folder
	// so the welcome page shows instead.
	if len(folders) == 0 && len(prURLs) == 0 && len(openFiles) == 0 && !parsed.welcome {
		cwd, _ := os.Getwd()
		if home, err := os.UserHomeDir(); !welcomeOnHome || err != nil || filepath.Clean(cwd) != filepath.Clean(home) {
			folders = append(folders, cwd)
		}
	}
	ws = workspace.New(folders)
	return
}

func BuildApp(cfg *config.AppConfig, borders *term.BorderSet) (*App, []string, []FileTarget) {
	ws, openFiles, _, prURLs := resolveArgs(cfg.Settings.Welcome.ShowOnHome)
	return BuildAppFromConfig(cfg, borders, ws, openFiles), prURLs, openFiles
}

var bracketColorSlots = []term.Style{
	term.StyleBracketColor1,
	term.StyleBracketColor2,
	term.StyleBracketColor3,
	term.StyleBracketColor4,
	term.StyleBracketColor5,
	term.StyleBracketColor6,
}

func ResolveBracketColorStyles(colors []string) []term.Style {
	if len(colors) == 0 {
		colors = []string{"yellow", "magenta", "blue"}
	}
	n := min(len(colors), len(bracketColorSlots))
	return bracketColorSlots[:n]
}

func BuildAppFromConfig(cfg *config.AppConfig, borders *term.BorderSet, ws *workspace.Workspace, openFiles []FileTarget) *App {
	locale, languageErr := loadDisplayLanguage(cfg.Settings.Language)
	initText()

	bracketStyles := ResolveBracketColorStyles(cfg.Theme.Editor.BracketColors)

	highlight.SetTimeLimit(time.Duration(cfg.Settings.Editor.HighlightTimeLimit) * time.Millisecond)
	editorGroup := ui.NewEditorGroupWidget(borders, cfg.Settings.Editor.TabSize, cfg.Settings.Editor.LineNumbers, cfg.Settings.Editor.GutterStyle)
	editorGroup.SetFileIconMode(cfg.Settings.Appearance.Icons)
	editorGroup.InsertSpaces = cfg.Settings.Editor.InsertSpaces
	editorGroup.SetIndentGuides(cfg.Settings.Editor.IndentGuides)
	editorGroup.InsertFinalNewline = cfg.Settings.Editor.InsertFinalNewline
	editorGroup.PreviewTabs = cfg.Settings.Editor.PreviewTabsEnabled()
	editorGroup.ShowTrailingNewline = cfg.Settings.Editor.IsShowTrailingNewlineEnabled()
	editorGroup.TrimTrailingWhitespace = cfg.Settings.Editor.TrimTrailingWhitespace
	editorGroup.SyntaxHighlight = cfg.Settings.Editor.IsSyntaxHighlightEnabled()
	editorGroup.WordWrap = cfg.Settings.Editor.WordWrap
	editorGroup.WordWrapColumn = cfg.Settings.Editor.WordWrapColumn
	editorGroup.WordWrapBreak = cfg.Settings.Editor.WordWrapBreak
	editorGroup.LanguageOverrides = cfg.Settings.Editor.LanguageOverrides
	editorGroup.WordWrapMaxBytes = cfg.Settings.Editor.WordWrapMaxBytes()
	editorGroup.DiffMode = configuredDiffMode(cfg.Settings.Editor.DiffMode)
	editorGroup.DiffContext = configuredDiffContext(cfg.Settings.Editor.DiffContext)
	editorGroup.DiffWordWrap = cfg.Settings.Editor.DiffWordWrap
	editorGroup.SetMarkdownMaxWidth(cfg.Settings.Markdown.WrapWidth)
	editorGroup.DiffHighContrast = cfg.Settings.Editor.DiffHighContrast
	editorGroup.DiffCollapsedEmphasis = cfg.Settings.Editor.DiffCollapsedEmphasis
	editorGroup.DiffSigns = cfg.Settings.Editor.DiffSigns
	editorGroup.DiffSignsColor = cfg.Settings.Editor.DiffSignsColor
	editorGroup.DiffGutterHighlight = cfg.Settings.Editor.DiffGutterHighlight
	editorGroup.DiffColorGutterNumbers = cfg.Settings.Editor.DiffColorGutterNumbers
	editorGroup.DiffScrollbarGitMarks = cfg.Settings.Editor.DiffScrollbarGitMarks
	editorGroup.LegacyScrollbarGlyphs = cfg.Settings.Appearance.ScrollbarGlyphs == config.ScrollbarGlyphsLegacy
	editorGroup.ApplyFileWordWrap()
	editorGroup.Editor.AutoDedent = cfg.Settings.Editor.IsAutoDedentEnabled()
	editorGroup.Editor.AutoIndent = cfg.Settings.Editor.IsAutoIndentEnabled()
	editorGroup.Editor.AutoClosing = cfg.Settings.Editor.AutoClosing
	editorGroup.Editor.LegacyScrollbarGlyphs = cfg.Settings.Appearance.ScrollbarGlyphs == config.ScrollbarGlyphsLegacy
	editorGroup.Editor.ScrollbarGitMarks = cfg.Settings.Editor.IsScrollbarGitMarksEnabled()
	editorGroup.UndoDeleteCursorStart = cfg.Settings.Editor.UndoDeleteCursorStart
	editorGroup.BracketPairColorization = cfg.Settings.Editor.BracketPairColorization
	editorGroup.Editor.BracketPairColorization = cfg.Settings.Editor.BracketPairColorization
	editorGroup.SetImageProtocol(cfg.Settings.Image.Protocol)
	editorGroup.BracketColorStyles = bracketStyles
	editorGroup.Editor.BracketColorStyles = bracketStyles
	grammarErrs := loadGrammarSettings(cfg.Settings.Editor.Grammars)
	_, _ = openFileTargets(editorGroup, openFiles)

	terminalPanel := ui.NewTerminalPanelWidget()
	terminalPanel.Borders = borders
	terminalPanel.PaddingX = cfg.Settings.Terminal.PaddingX
	problems := ui.NewProblemsWidget()
	references := ui.NewReferencesWidget()
	output := ui.NewOutputWidget()
	bottomPanel := ui.NewBottomPanelWidget(borders)
	bottomPanel.AddPanel("terminal", i18n.Text("panel.terminal"), terminalPanel)
	bottomPanel.AddPanel("problems", i18n.Text("panel.diagnostics"), problems)
	bottomPanel.AddPanel("references", i18n.Text("panel.references"), references)
	bottomPanel.AddPanel("output", i18n.Text("panel.output"), output)

	contentSplit := ui.NewContentSplitWidget()
	contentSplit.Top = editorGroup
	contentSplit.Bottom = bottomPanel
	contentSplit.Borders = borders
	contentSplit.ShowBottom = false

	status := view.NewStatusBar()
	statusBar := ui.NewStatusBarWidget(status)

	menuBar := ui.NewMenuBarWidget([]ui.MenuItem{
		{Name: i18n.Text("menu.file")},
		{Name: i18n.Text("menu.edit")},
		{Name: i18n.Text("menu.selection")},
		{Name: i18n.Text("menu.view")},
		{Name: i18n.Text("menu.options")},
		{Name: i18n.Text("general.help")},
	})

	search := ui.NewSearchWidget()
	search.Icons = cfg.Settings.Appearance.Icons
	search.SetWorkDirs(ws.Paths())
	search.Debounce.DelayMs = cfg.Settings.Search.Debounce
	changes := NewChangesPanel(ws.Paths()...)
	changes.SetFileView(cfg.Settings.Git.FileView)
	changes.SetIcons(cfg.Settings.Appearance.Icons)
	state := config.LoadState()
	if h := state.CommitHistoryHeight; h > 0 {
		changes.Split.BottomH = h
		changes.Split.BottomRatio = 0
	} else if h := cfg.Settings.Sidebar.CommitHistoryHeight; h > 0 {
		changes.Split.BottomH = h
		changes.Split.BottomRatio = 0
	}
	symbols := NewSymbolsPanel()
	symbols.SetIcons(cfg.Settings.Appearance.Icons)

	explorer := NewNavigationPanel(cfg.Settings.Explorer, cfg.Settings.Appearance.Icons, ws.Paths()...)

	sidebar := ui.NewSidebarWidget()
	sidebar.AddPanel("explorer", i18n.Text("panel.explore"), explorer.Adapter)
	sidebar.AddPanel("search", i18n.Text("general.find"), search)
	sidebar.AddPanel("changes", i18n.Text("panel.changes"), changes.Adapter)
	sidebar.AddPanel("outline", i18n.Text("panel.outline"), symbols.Adapter)
	if len(state.SidebarPanelOrder) > 0 {
		sidebar.SetPanelOrder(state.SidebarPanelOrder)
	} else {
		sidebar.SetPanelOrder(cfg.Settings.Sidebar.PanelOrder)
	}
	sidebar.Tabs.Config.Reorderable = true
	hasFolders := len(ws.Paths()) > 0
	sidebar.Visible = hasFolders && !state.SidebarHidden
	sidebar.Borders = borders

	splitPanel := ui.NewSplitPanelWidget()
	splitPanel.Left = sidebar
	splitPanel.Right = contentSplit
	splitPanel.Borders = borders
	splitPanel.DividerPos = ui.DefaultSidebarWidth
	if state.SidebarWidth >= ui.MinSidebarWidth {
		splitPanel.DividerPos = state.SidebarWidth
	} else if cfg.Settings.Sidebar.Width >= ui.MinSidebarWidth {
		splitPanel.DividerPos = cfg.Settings.Sidebar.Width
	}
	panelPos := state.PanelPosition
	if panelPos == "" {
		panelPos = cfg.Settings.Panel.Position
	}
	if panelPos == "right" {
		contentSplit.Position = ui.SplitRight
	}
	if state.PanelHeight >= ui.MinPanelHeight {
		contentSplit.BottomH = state.PanelHeight
	}
	if state.PanelWidth >= ui.MinPanelWidth {
		contentSplit.RightW = state.PanelWidth
	}
	splitPanel.ShowLeft = sidebar.Visible
	splitPanel.RightBorderStartY = 2
	contentSplit.RightBorderStartY = &splitPanel.RightBorderStartY
	editorGroup.TabBar.PointerInteractionValid = func() bool {
		return contentSplit.TopContentHeight() > 3
	}
	sidebar.Tabs.Config.PointerInteractionValid = func() bool {
		r := sidebar.GetRect()
		return sidebar.Visible && splitPanel.ShowLeft && r.W > 0 && r.H > 2
	}
	bottomPanel.Tabs.Config.PointerInteractionValid = func() bool {
		r := bottomPanel.GetRect()
		return contentSplit.ShowBottom && bottomPanel.Visible && r.W > 0 && r.H > 2
	}

	rootBox := widgets.NewVStackWidget(menuBar, splitPanel, statusBar)

	root := ui.NewRoot(rootBox)
	root.SetControlModifiers(controlModifiers(cfg.Settings.Keyboard.ModifierKey))
	root.SetForwardShortcuts(cfg.Settings.Terminal.ForwardShortcuts)
	editorGroup.ControlModifiers = &root.ControlModifiers
	output.ControlModifiers = &root.ControlModifiers
	for _, input := range []*widgets.InputWidget{search.Input, search.Include, search.Exclude, search.ReplaceInput, changes.Input} {
		input.ControlModifiers = &root.ControlModifiers
	}
	root.SetFocus(editorGroup)

	app := &App{
		Root:                root,
		RootBox:             rootBox,
		EditorGroup:         editorGroup,
		Sidebar:             sidebar,
		SplitPanel:          splitPanel,
		ContentSplit:        contentSplit,
		BottomPanel:         bottomPanel,
		Explorer:            explorer,
		Search:              search,
		Changes:             changes,
		Symbols:             symbols,
		MenuBar:             menuBar,
		StatusBar:           statusBar,
		Status:              status,
		Borders:             borders,
		Settings:            &cfg.Settings,
		State:               state,
		Workspace:           ws,
		Palette:             BuildTerminalPalettePtr(cfg.Theme, WithTransparentBackground(cfg.Settings.Editor.TransparentBackground), WithTerminalSelectionStyle(cfg.Settings.Terminal.SelectionStyle)),
		TerminalPanel:       terminalPanel,
		Problems:            problems,
		References:          references,
		Output:              output,
		DocVersions:         make(map[string]int),
		lspSent:             make(map[string]<-chan struct{}),
		lspEpochs:           make(map[string]int),
		lastPulled:          make(map[string]pullKey),
		LspNotified:         make(map[string]bool),
		pluginDetailWidgets: make(map[string]*pluginDetailState),
	}
	app.setExplorerSearchDebounce(cfg.Settings.Search.Debounce)
	app.diagPull = NewTask[pulledDiagnostics](app, asynctask.TaskOptions{Delay: pullDiagnosticsDelay})
	app.diagBackground = make(map[string]*asynctask.Task[[]backgroundPull])
	app.Repository = NewRepositoryState(changes, ws.Paths())
	app.Repository.SetExplorer(explorer)
	app.Repository.SetCurrentChangesHandler(app.ApplyCurrentChanges)
	app.applyMenuBarVisibility(cfg.Settings.Editor.IsMenuBarVisible())
	// Rebuild the Diagnostics panel whenever any source (LSP or a plugin)
	// changes its diagnostics.
	app.EditorGroup.OnDiagnosticsChanged = app.refreshProblems
	app.EditorGroup.FileHandler = app.openWithFileHandler
	app.EditorGroup.OnFileHandled = app.fileHandlerTabOpened
	app.EditorGroup.LinkWorkDir = app.Workspace.Primary
	app.EditorGroup.OnOpenURL = OpenURL
	app.EditorGroup.OnOpenFileLink = app.openFileLink
	app.EditorGroup.OnLinkHover = app.Root.TrackEditorLinkHover
	app.EditorGroup.ResolveFileLinks = app.resolveEditorFileLinks
	app.applyIcons(cfg.Settings.Appearance)
	app.applySidebarTabStyle(cfg.Settings.Sidebar.TabStyle)
	app.loadedGrammars = slices.Clone(cfg.Settings.Editor.Grammars)
	app.reportGrammarErrors(grammarErrs)
	app.language.locale = locale
	app.language.requested = cfg.Settings.Language.Locale
	if languageErr != nil {
		app.LogOutput("error", "languages", languageErr.Error())
	}
	editorGroup.FoldTask = NewTask[[]fold.Range](app, asynctask.TaskOptions{})
	editorGroup.FoldDelay = NewTask[struct{}](app, asynctask.TaskOptions{Delay: ui.FoldRecomputeDelay})
	app.appliedSettings.Terminal.ForwardShortcuts = cfg.Settings.Terminal.ForwardShortcuts
	if cfg.SettingsError != nil {
		app.StatusWarn(i18n.Format("settings.loadFailed", i18n.Args{"error": cfg.SettingsError.Error()}))
	}
	return app
}

// ApplyFileTargets frames the file left active at startup. Every file's cursor
// is already placed as it opens; the scroll has to wait until here because
// GoToLineCol centres the line against the viewport height, which is zero until
// the first render.
//
// Only the active file is framed. Scrolling a background tab means switching to
// it and back, which leaves that tab's viewport shifted, so the other files
// keep a correct cursor and an unscrolled view instead.
func (a *App) ApplyFileTargets(targets []FileTarget) {
	active := a.EditorGroup.ActiveFilePath()
	for _, target := range slices.Backward(targets) {
		if target.Line > 0 && target.Path == active {
			a.EditorGroup.GoToLineCol(target.Line, target.Col)
			return
		}
	}
}

func openFileTargets(g *ui.EditorGroupWidget, targets []FileTarget) ([]FileTarget, error) {
	var opened []FileTarget
	var failures []error
	for _, f := range targets {
		err := g.OpenFile(f.Path)
		if g.ActiveFilePath() == f.Path {
			g.CommitActiveTab()
		}
		if err != nil {
			failures = append(failures, err)
			continue
		}
		opened = append(opened, f)
		// The tab just opened is the active one, so this lands on the right
		// buffer without switching tabs. PlaceCursor rather than GoToLineCol:
		// the viewport has no height yet, and ApplyFileTargets frames the file
		// left active once the first render has run.
		if f.Line > 0 {
			g.PlaceCursor(f.Line, f.Col)
		}
	}
	return opened, errors.Join(failures...)
}
