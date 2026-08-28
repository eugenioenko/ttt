package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
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

func resolveArgs() (ws *workspace.Workspace, openFiles []FileTarget, configFile string, prURLs []string) {
	var folders []string
	var wsFile string

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		if args[i] == "--workspace" && i+1 < len(args) {
			wsFile = args[i+1]
			i++
			continue
		}
		if args[i] == "--config" && i+1 < len(args) {
			configFile = args[i+1]
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
		if args[i] == "--debug" {
			continue
		}
		if args[i] == "--listen" {
			continue
		}
		if isPRURL(args[i]) {
			if _, _, _, err := github.ParsePRURL(args[i]); err == nil {
				prURLs = append(prURLs, args[i])
			}
			continue
		}
		absPath, err := filepath.Abs(workspace.ExpandPath(args[i]))
		if err != nil {
			openFiles = append(openFiles, FileTarget{Path: args[i]})
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
				openFiles = append(openFiles, target)
				continue
			}
			openFiles = append(openFiles, FileTarget{Path: absPath})
			continue
		}
		if info.IsDir() {
			folders = append(folders, absPath)
		} else {
			openFiles = append(openFiles, FileTarget{Path: absPath})
		}
	}

	if wsFile != "" {
		loaded, err := workspace.LoadFile(wsFile)
		if err == nil {
			ws = loaded
			for _, f := range folders {
				ws.AddFolder(f)
			}
			return
		}
	}

	// Opening only files intentionally creates no workspace — a folder must be passed explicitly.
	if len(folders) == 0 && len(prURLs) == 0 {
		cwd, _ := os.Getwd()
		folders = append(folders, cwd)
	}
	ws = workspace.New(folders)
	return
}

func BuildApp(cfg *config.AppConfig, borders *term.BorderSet) (*App, []string, []FileTarget) {
	ws, openFiles, _, prURLs := resolveArgs()
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
	n := len(colors)
	if n > len(bracketColorSlots) {
		n = len(bracketColorSlots)
	}
	return bracketColorSlots[:n]
}

func BuildAppFromConfig(cfg *config.AppConfig, borders *term.BorderSet, ws *workspace.Workspace, openFiles []FileTarget) *App {

	bracketStyles := ResolveBracketColorStyles(cfg.Theme.Editor.BracketColors)

	editorGroup := ui.NewEditorGroupWidget(borders, cfg.Settings.Editor.TabSize, cfg.Settings.Editor.LineNumbers, cfg.Settings.Editor.GutterStyle)
	editorGroup.InsertSpaces = cfg.Settings.Editor.InsertSpaces
	editorGroup.InsertFinalNewline = cfg.Settings.Editor.InsertFinalNewline
	editorGroup.ShowTrailingNewline = cfg.Settings.Editor.IsShowTrailingNewlineEnabled()
	editorGroup.TrimTrailingWhitespace = cfg.Settings.Editor.TrimTrailingWhitespace
	editorGroup.SyntaxHighlight = cfg.Settings.Editor.IsSyntaxHighlightEnabled()
	editorGroup.WordWrap = cfg.Settings.Editor.WordWrap
	editorGroup.DiffMode = configuredDiffMode(cfg.Settings.Editor.DiffMode)
	editorGroup.DiffContext = configuredDiffContext(cfg.Settings.Editor.DiffContext)
	editorGroup.DiffWordWrap = cfg.Settings.Editor.DiffWordWrap
	editorGroup.DiffHighContrast = cfg.Settings.Editor.DiffHighContrast
	editorGroup.DiffCollapsedEmphasis = cfg.Settings.Editor.DiffCollapsedEmphasis
	editorGroup.Editor.WordWrap = cfg.Settings.Editor.WordWrap
	editorGroup.Editor.AutoDedent = cfg.Settings.Editor.IsAutoDedentEnabled()
	editorGroup.Editor.AutoIndent = cfg.Settings.Editor.IsAutoIndentEnabled()
	editorGroup.UndoDeleteCursorStart = cfg.Settings.Editor.UndoDeleteCursorStart
	editorGroup.BracketPairColorization = cfg.Settings.Editor.BracketPairColorization
	editorGroup.Editor.BracketPairColorization = cfg.Settings.Editor.BracketPairColorization
	editorGroup.BracketColorStyles = bracketStyles
	editorGroup.Editor.BracketColorStyles = bracketStyles
	for _, f := range openFiles {
		editorGroup.OpenFile(f.Path)
		editorGroup.CommitActiveTab()
		// The tab just opened is the active one, so this lands on the right
		// buffer without switching tabs. PlaceCursor rather than GoToLineCol:
		// the viewport has no height yet, and ApplyFileTargets frames the file
		// left active once the first render has run.
		if f.Line > 0 {
			editorGroup.PlaceCursor(f.Line, f.Col)
		}
	}

	terminalPanel := ui.NewTerminalPanelWidget()
	terminalPanel.Borders = borders
	problems := ui.NewProblemsWidget()
	references := ui.NewReferencesWidget()
	output := ui.NewOutputWidget()
	bottomPanel := ui.NewBottomPanelWidget(borders)
	bottomPanel.AddPanel("terminal", "Terminal", terminalPanel)
	bottomPanel.AddPanel("problems", "Diagnostics", problems)
	bottomPanel.AddPanel("references", "References", references)
	bottomPanel.AddPanel("output", "Output", output)

	contentSplit := ui.NewContentSplitWidget()
	contentSplit.Top = editorGroup
	contentSplit.Bottom = bottomPanel
	contentSplit.Borders = borders
	contentSplit.ShowBottom = false

	status := view.NewStatusBar()
	statusBar := ui.NewStatusBarWidget(status)

	menuBar := ui.NewMenuBarWidget([]ui.MenuItem{
		{Name: "File"},
		{Name: "Edit"},
		{Name: "Selection"},
		{Name: "View"},
		{Name: "Options"},
		{Name: "Help"},
	})

	search := ui.NewSearchWidget()
	search.SetWorkDirs(ws.Paths())
	search.Debounce.DelayMs = cfg.Settings.Search.Debounce
	changes := NewChangesPanel(ws.Paths()...)
	changes.SetFileView(cfg.Settings.Git.FileView)
	if cfg.Settings.Sidebar.CommitHistoryHeight > 0 {
		changes.Split.BottomH = cfg.Settings.Sidebar.CommitHistoryHeight
		changes.Split.BottomRatio = 0
	}
	symbols := NewSymbolsPanel()

	explorer := NewNavigationPanel(cfg.Settings.Explorer, ws.Paths()...)

	sidebar := ui.NewSidebarWidget()
	sidebar.AddPanel("explorer", "Explore", explorer.Adapter)
	sidebar.AddPanel("search", "Find", search)
	sidebar.AddPanel("changes", "Changes", changes.Adapter)
	sidebar.AddPanel("outline", "Outline", symbols.Adapter)
	sidebar.SetPanelOrder(cfg.Settings.Sidebar.PanelOrder)
	sidebar.Tabs.Config.Reorderable = true
	hasFolders := len(ws.Paths()) > 0
	sidebar.Visible = hasFolders
	sidebar.Borders = borders

	splitPanel := ui.NewSplitPanelWidget()
	splitPanel.Left = sidebar
	splitPanel.Right = contentSplit
	splitPanel.Borders = borders
	splitPanel.DividerPos = ui.DefaultSidebarWidth
	if cfg.Settings.Sidebar.Width > 0 {
		splitPanel.DividerPos = cfg.Settings.Sidebar.Width
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

	rootBox := widgets.NewVStackWidget(menuBar, splitPanel, statusBar)

	root := ui.NewRoot(rootBox)
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
		Workspace:           ws,
		Palette:             BuildTerminalPalettePtr(cfg.Theme),
		TerminalPanel:       terminalPanel,
		Problems:            problems,
		References:          references,
		Output:              output,
		DocVersions:         make(map[string]int),
		LspNotified:         make(map[string]bool),
		pluginDetailWidgets: make(map[string]*pluginDetailState),
	}
	app.Repository = NewRepositoryState(changes, ws.Paths())
	app.Repository.SetCurrentChangesHandler(app.ApplyCurrentChanges)
	app.applyMenuBarVisibility(cfg.Settings.Editor.IsMenuBarVisible())
	// Rebuild the Diagnostics panel whenever any source (LSP or a plugin)
	// changes its diagnostics.
	app.EditorGroup.OnDiagnosticsChanged = app.refreshProblems
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
	for i := len(targets) - 1; i >= 0; i-- {
		if targets[i].Line > 0 && targets[i].Path == active {
			a.EditorGroup.GoToLineCol(targets[i].Line, targets[i].Col)
			return
		}
	}
}
