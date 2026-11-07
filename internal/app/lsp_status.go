package app

import (
	"fmt"
	"os/exec"

	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/view"
)

// SyncLanguageSegment refreshes the language status bar segment. It is separate
// from syncStatus so a language server changing state asynchronously can update
// the indicator without the full status sync, which the event loop runs only on
// user input.
func (a *App) SyncLanguageSegment() {
	if a.EditorGroup.ActiveBuffer() == nil {
		a.Status.SetSegment(view.StatusSegment{ID: "language", Side: "right", Priority: 500, Text: ""})
		a.Status.SetSegment(view.StatusSegment{ID: "lsp", Side: "right", Priority: 501, Text: ""})
		return
	}

	lang := i18n.Text("lsp.plainText")
	if h := a.EditorGroup.Editor.Highlighter; h != nil {
		lang = h.Language()
	}
	a.Status.SetSegment(view.StatusSegment{
		ID: "language", Side: "right", Priority: 500,
		Text:    lang,
		OnClick: a.ShowLanguagePicker,
	})
	icon, iconStyle := a.lspStatusIcon(a.EditorGroup.ActiveFilePath(), a.EditorGroup.ActiveLanguage())
	a.Status.SetSegment(view.StatusSegment{
		ID: "lsp", Side: "right", Priority: 501,
		Text:    icon,
		Style:   iconStyle,
		OnClick: a.ShowOutputPanel,
	})
}

// lspStatusIcon reports the language server indicator for the status bar.
func (a *App) lspStatusIcon(filePath, lang string) (string, term.Style) {
	serverKey, _, configured := a.lspResolve(filePath, lang)
	if !configured {
		return "", 0
	}

	return lspIcon(a.lspServerState(serverKey))
}

func (a *App) lspServerState(serverKey string) (state lsp.ServerState, binaryOK bool) {
	// The status bar syncs on every keystroke, so only stat the binary when its
	// presence is what the icon actually turns on.
	state = a.LspManager.State(serverKey)
	binaryOK = true
	if state == lsp.ServerStopped {
		if cfg := a.LspManager.ServerConfig(serverKey); len(cfg.Command) > 0 {
			_, err := exec.LookPath(cfg.Command[0])
			binaryOK = err == nil
		}
	}
	return state, binaryOK
}

func (a *App) ShowLSPServerLog() {
	path, lang := a.editorPathLang()
	serverKey, _, ok := a.lspResolve(path, lang)
	if path == "" || !ok {
		a.StatusNotify(i18n.Text("status.noLanguageServerForThisFile"))
		return
	}
	a.ShowOutputPanel()
	a.StatusNotify(fmt.Sprintf("%s: %s", serverKey, lspStateLabel(a.lspServerState(serverKey))))
}

func lspStateLabel(state lsp.ServerState, binaryOK bool) string {
	switch state {
	case lsp.ServerReady:
		return i18n.Text("lsp.running")
	case lsp.ServerStarting:
		return i18n.Text("lsp.starting")
	case lsp.ServerFailed:
		return i18n.Text("lsp.failed")
	}
	if !binaryOK {
		return i18n.Text("lsp.notInstalled")
	}
	return i18n.Text("lsp.notStarted")
}

// lspIcon maps a server state to its indicator. Every icon is single-width
// under both normal and East Asian width rules, so the segment does not shift
// as the state changes — ⊕, ● and ○ are ambiguous width and would.
func lspIcon(state lsp.ServerState, binaryOK bool) (string, term.Style) {
	switch state {
	case lsp.ServerReady:
		return "◉", 0
	case lsp.ServerStarting:
		return "◌", 0
	case lsp.ServerFailed:
		return "⚠", term.StyleDanger
	}

	// Not started — servers launch lazily on the first request. A missing
	// binary is the one failure knowable before then, and ttt never even tries.
	if !binaryOK {
		return "⚠", term.StyleDanger
	}
	return "◌", 0
}

// LSPStateChanged tells the event loop a language server changed state.
type LSPStateChanged struct{}
