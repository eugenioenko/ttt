package app

import (
	"os"
	"strings"

	"github.com/eugenioenko/ttt/internal/command"
)

func registerPreviewCommands(app *App) {
	reg := app.Reg

	reg.Register(command.Command{
		ID:       "editor.openPreview",
		Title:    "Open Preview",
		Keywords: []string{"markdown", "preview", "md"},
		Handler: func() {
			path := app.EditorGroup.ActiveFilePath()
			// Fallback: if triggered from the explorer context menu, use the selected node
			if path == "" || !strings.HasSuffix(path, ".md") {
				if node := app.Explorer.SelectedNode(); node != nil && strings.HasSuffix(node.Path, ".md") {
					path = node.Path
				}
			}
			if path == "" {
				app.StatusWarn("No active file")
				return
			}
			if !strings.HasSuffix(path, ".md") {
				app.StatusWarn("Preview is only available for .md files")
				return
			}
			data, err := os.ReadFile(path)
			if err != nil {
				app.StatusWarn("Cannot read file: " + err.Error())
				return
			}
			app.EditorGroup.OpenPreview(path, string(data))
		},
	})
}
