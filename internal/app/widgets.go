package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/core/navhistory"
	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/view"
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
		if isPRURL(args[i]) {
			if _, _, _, err := github.ParsePRURL(args[i]); err == nil {
				prURLs = append(prURLs, args[i])
			}
			continue
		}
		absPath, err := filepath.Abs(args[i])
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

func BuildAppFromConfig(cfg *config.AppConfig, borders *term.BorderSet, ws *workspace.Workspace, openFiles []string) *App {

	editorGroup := ui.NewEditorGroupWidget(borders, cfg.Settings.Editor.TabSize, cfg.Settings.Editor.LineNumbers, cfg.Settings.Editor.GutterStyle)
	editorGroup.InsertFinalNewline = cfg.Settings.Editor.InsertFinalNewline
	editorGroup.TrimTrailingWhitespace = cfg.Settings.Editor.TrimTrailingWhitespace
	for _, f := range openFiles {
		editorGroup.OpenFile(f)
		editorGroup.PinActiveTab()
	}

	terminalPanel := ui.NewTerminalPanelWidget()
	problems := ui.NewProblemsWidget()
	references := ui.NewReferencesWidget()
	bottomPanel := ui.NewBottomPanelWidget(borders)
	bottomPanel.AddPanel("terminal", "TERMINAL", terminalPanel)
	bottomPanel.AddPanel("problems", "PROBLEMS", problems)
	bottomPanel.AddPanel("references", "REFERENCES", references)

	contentSplit := ui.NewContentSplitWidget()
	contentSplit.Top = editorGroup
	contentSplit.Bottom = bottomPanel
	contentSplit.Borders = borders
	contentSplit.ShowBottom = false

	status := &view.StatusBar{FileName: editorGroup.ActiveFilePath()}
	statusBar := ui.NewStatusBarWidget(status)

	menuBar := ui.NewMenuBarWidget([]ui.MenuItem{
		{Name: "File"},
		{Name: "Edit"},
		{Name: "Selection"},
		{Name: "View"},
		{Name: "Help"},
	})

	explorer := ui.NewExplorerWidget(cfg.Settings.Explorer, ws.Paths()...)
	search := ui.NewSearchWidget()
	search.SetWorkDirs(ws.Paths())
	search.Debounce.DelayMs = cfg.Settings.Search.Debounce
	changes := ui.NewChangesWidget(ws.Paths()...)

	sidebar := ui.NewSidebarWidget()
	sidebar.AddPanel("explorer", "Explore", explorer)
	sidebar.AddPanel("search", "Find", search)
	sidebar.AddPanel("changes", "Changes", changes)
	hasFolders := len(ws.Paths()) > 0
	sidebar.Visible = hasFolders
	sidebar.Borders = borders

	splitPanel := ui.NewSplitPanelWidget()
	splitPanel.Left = sidebar
	splitPanel.Right = contentSplit
	splitPanel.Borders = borders
	splitPanel.DividerPos = 30
	splitPanel.ShowLeft = sidebar.Visible
	splitPanel.RightBorderStartY = 2
	contentSplit.RightBorderStartY = &splitPanel.RightBorderStartY

	rootBox := &ui.VBox{}
	rootBox.AddChild(menuBar, ui.LayoutConstraint{Type: ui.Fixed, Value: 1})
	rootBox.AddChild(splitPanel, ui.LayoutConstraint{Type: ui.Flex, Value: 1})
	rootBox.AddChild(statusBar, ui.LayoutConstraint{Type: ui.Fixed, Value: 1})

	root := ui.NewRoot(rootBox)
	root.SetFocus(editorGroup)

	return &App{
		Root:           root,
		EditorGroup:    editorGroup,
		Sidebar:        sidebar,
		SplitPanel:     splitPanel,
		ContentSplit:   contentSplit,
		BottomPanel:    bottomPanel,
		Explorer:       explorer,
		Search:         search,
		Changes:        changes,
		MenuBar:        menuBar,
		StatusBar:      statusBar,
		Status:         status,
		Borders:        borders,
		Settings:       &cfg.Settings,
		Workspace:      ws,
		Palette:        BuildTerminalPalettePtr(cfg.Theme),
		TerminalPanel:  terminalPanel,
		Problems:       problems,
		References:     references,
		DocVersions:    make(map[string]int),
		AllDiagnostics: make(map[string][]ui.Diagnostic),
		LspNotified:    make(map[string]bool),
		NavHistory:     navhistory.New(100),
	}
}
