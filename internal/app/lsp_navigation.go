package app

import (
	"log/slog"
	"strings"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v3"
)

func (a *App) RequestRename(path, lang string, line, col int, newName string) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		if lang != "" {
			a.StatusWarn(i18n.Format("status.languageServerIsNotConfigured", i18n.Args{"lang": lang}))
		}
		return
	}
	col = a.lspColumn(path, line, col)
	workDir := a.lspWorkDirFor(path, lang)
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			slog.Error("lsp client", "err", err)
			return
		}
		edit, err := client.Rename(FileURI(path), line, col, newName)
		if err != nil {
			slog.Error("lsp rename", "err", err)
			return
		}
		if edit != nil {
			_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&RenameResult{Edit: edit}))
		}
	}()
}

func (a *App) RequestReferences(path, lang string, line, col int) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		if lang != "" {
			a.StatusWarn(i18n.Format("status.languageServerIsNotConfigured", i18n.Args{"lang": lang}))
		}
		return
	}
	col = a.lspColumn(path, line, col)
	workDir := a.lspWorkDirFor(path, lang)
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			slog.Error("lsp client", "err", err)
			return
		}
		locs, err := client.References(FileURI(path), line, col, true)
		if err != nil {
			slog.Error("lsp references", "err", err)
			return
		}
		if len(locs) > 0 {
			_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&ReferencesResult{Locations: locs}))
		} else {
			_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&ReferencesResult{}))
		}
	}()
}

func (a *App) ShowReferences(locs []lsp.Location) {
	items := make([]ui.ReferenceItem, 0, len(locs))
	documents := make(map[string][]string)
	for _, loc := range locs {
		path := URIToPath(loc.URI)
		lines, ok := documents[path]
		if !ok {
			lines = a.lspLines(path)
			documents[path] = lines
		}
		var text string
		if line := loc.Range.Start.Line; line >= 0 && line < len(lines) {
			text = lines[line]
		}
		items = append(items, ui.ReferenceItem{
			File: path,
			Line: loc.Range.Start.Line,
			Col:  lsp.UTF16ToRune(text, loc.Range.Start.Character),
			Text: strings.TrimSpace(text),
		})
	}
	a.References.SetItems(items)
	a.BottomPanel.SetActivePanel("references")
	if !a.ContentSplit.ShowBottom {
		a.ContentSplit.ShowBottom = true
	}
}

func (a *App) RequestDefinition(path, lang string, line, col int) {
	a.requestLocation("textDocument/definition", path, lang, line, col)
}

func (a *App) RequestImplementation(path, lang string, line, col int) {
	a.requestLocation("textDocument/implementation", path, lang, line, col)
}

func (a *App) RequestTypeDefinition(path, lang string, line, col int) {
	a.requestLocation("textDocument/typeDefinition", path, lang, line, col)
}

func (a *App) requestLocation(method, path, lang string, line, col int) {
	serverKey, _, ok := a.lspResolve(path, lang)
	if !ok {
		if lang != "" {
			a.StatusWarn(i18n.Format("status.languageServerIsNotConfigured", i18n.Args{"lang": lang}))
		}
		return
	}
	col = a.lspColumn(path, line, col)
	workDir := a.lspWorkDirFor(path, lang)
	go func() {
		client, err := a.LspManager.ClientForLanguage(serverKey, workDir)
		if err != nil {
			slog.Error("lsp client", "err", err)
			return
		}
		var locs []lsp.Location
		var reqErr error
		switch method {
		case "textDocument/definition":
			locs, reqErr = client.Definition(FileURI(path), line, col)
		case "textDocument/implementation":
			locs, reqErr = client.Implementation(FileURI(path), line, col)
		case "textDocument/typeDefinition":
			locs, reqErr = client.TypeDefinition(FileURI(path), line, col)
		}
		if reqErr != nil {
			slog.Error("lsp "+method, "err", reqErr)
			return
		}
		if len(locs) > 0 {
			_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&LocationResult{Locations: locs}))
		}
	}()
}
