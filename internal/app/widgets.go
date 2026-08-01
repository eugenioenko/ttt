package app

import (
	"os"
	"path/filepath"
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

func resolveArgs() (ws *workspace.Workspace, openFiles []string, configFile string, prURLs []string) {
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
		if isPRURL(args[i]) {
			if _, _, _, err := github.ParsePRURL(args[i]); err == nil {
				prURLs = append(prURLs, args[i])
			}
			continue
		}
		absPath, err := filepath.Abs(workspace.ExpandPath(args[i]))
		if err != nil {
			openFiles = append(openFiles, args[i])
			continue
		}
		info, err := os.Stat(absPath)
		if err != nil {
			openFiles = append(openFiles, absPath)
			continue
		}
		if info.IsDir() {
			folders = append(folders, absPath)
		} else {
			openFiles = append(openFiles, absPath)
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
	if len(folders) == 0 && len(prURLs) == 0 && len(openFiles) == 0 {
		cwd, _ := os.Getwd()
		folders = append(folders, cwd)
	}
	ws = workspace.New(folders)
	return
}

func BuildApp(cfg *config.AppConfig, borders *term.BorderSet) (*App, []string) {
	ws, openFiles, _, prURLs := resolveArgs()
	return BuildAppFromConfig(cfg, borders, ws, openFiles), prURLs
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

func BuildAppFromConfig(cfg *config.AppConfig, borders *term.BorderSet, ws *workspace.Workspace, openFiles []string) *App {

	bracketStyles := ResolveBracketColorStyles(cfg.Theme.Editor.BracketColors)

	editorGroup := ui.NewEditorGroupWidget(borders, cfg.Settings.Editor.TabSize, cfg.Settings.Editor.LineNumbers, cfg.Settings.Editor.GutterStyle)
	editorGroup.InsertSpaces = cfg.Settings.Editor.InsertSpaces
	editorGroup.InsertFinalNewline = cfg.Settings.Editor.InsertFinalNewline
	editorGroup.ShowTrailingNewline = cfg.Settings.Editor.IsShowTrailingNewlineEnabled()
	editorGroup.TrimTrailingWhitespace = cfg.Settings.Editor.TrimTrailingWhitespace
	editorGroup.SyntaxHighlight = cfg.Settings.Editor.IsSyntaxHighlightEnabled()
	editorGroup.WordWrap = cfg.Settings.Editor.WordWrap
	editorGroup.Editor.WordWrap = cfg.Settings.Editor.WordWrap
	editorGroup.Editor.AutoDedent = cfg.Settings.Editor.IsAutoDedentEnabled()
	editorGroup.Editor.AutoIndent = cfg.Settings.Editor.IsAutoIndentEnabled()
	editorGroup.UndoDeleteCursorStart = cfg.Settings.Editor.UndoDeleteCursorStart
	editorGroup.BracketPairColorization = cfg.Settings.Editor.BracketPairColorization
	editorGroup.Editor.BracketPairColorization = cfg.Settings.Editor.BracketPairColorization
	editorGroup.BracketColorStyles = bracketStyles
	editorGroup.Editor.BracketColorStyles = bracketStyles
	for _, f := range openFiles {
		editorGroup.OpenFile(f)
		editorGroup.PinActiveTab()
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
	symbols := NewSymbolsPanel()

	explorer := NewNavigationPanel(cfg.Settings.Explorer, ws.Paths()...)

	sidebar := ui.NewSidebarWidget()
	sidebar.AddPanel("explorer", "Explore", explorer.Adapter)
	sidebar.AddPanel("search", "Find", search)
	sidebar.AddPanel("changes", "Changes", changes.Adapter)
	sidebar.AddPanel("outline", "Outline", symbols.Adapter)
	hasFolders := len(ws.Paths()) > 0
	sidebar.Visible = hasFolders
	sidebar.Borders = borders

	splitPanel := ui.NewSplitPanelWidget()
	splitPanel.Left = sidebar
	splitPanel.Right = contentSplit
	splitPanel.Borders = borders
	splitPanel.DividerPos = ui.DefaultSidebarWidth
	splitPanel.ShowLeft = sidebar.Visible
	splitPanel.RightBorderStartY = 2
	contentSplit.RightBorderStartY = &splitPanel.RightBorderStartY

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
	app.applyMenuBarVisibility(cfg.Settings.Editor.IsMenuBarVisible())
	// Rebuild the Diagnostics panel whenever any source (LSP or a plugin)
	// changes its diagnostics.
	app.EditorGroup.OnDiagnosticsChanged = app.refreshProblems
	return app
}
