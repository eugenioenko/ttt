package app

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/i18n"
)

func (a *App) RunExternalFormatter() {
	if !a.EditorGroup.IsEditorActive() {
		return
	}
	path := a.EditorGroup.ActiveFilePath()
	cmd := a.formatterForFile(path)
	if cmd == "" {
		ext := strings.TrimPrefix(filepath.Ext(path), ".")
		a.StatusWarn(i18n.Format("status.noFormatterConfiguredForFiles", i18n.Args{"ext": ext}))
		return
	}
	if err := a.applyExternalFormatter(cmd, path); err != nil {
		a.StatusWarn(i18n.Format("status.format", i18n.Args{"error": err.Error()}))
	}
}

func (a *App) FormatExternalOnSave(path string) bool {
	cmd := a.formatterForFile(path)
	if cmd == "" {
		return false
	}
	if err := a.applyExternalFormatter(cmd, path); err != nil {
		a.StatusWarn(i18n.Format("status.formatOnSave", i18n.Args{"error": err.Error()}))
		return false
	}
	return true
}

func (a *App) formatterForFile(path string) string {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	return a.Settings.FormatterForExt(ext)
}

func (a *App) applyExternalFormatter(cmdStr, filePath string) error {
	editor := a.EditorGroup.Editor
	if editor == nil {
		return nil
	}

	original := strings.Join(editor.Buf.Lines, "\n")

	expanded := strings.ReplaceAll(cmdStr, "{file}", filePath)
	parts := strings.Fields(expanded)
	if len(parts) == 0 {
		return fmt.Errorf("empty formatter command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdin = strings.NewReader(original)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}

	formatted := stdout.String()
	if formatted == original {
		return nil
	}
	formatted = strings.TrimSuffix(formatted, "\n")

	comparison := original
	if editor.Buf.ShowTrailingNewline {
		comparison = strings.TrimSuffix(comparison, "\n")
	}
	if formatted == comparison {
		return nil
	}

	lastLine := len(editor.Buf.Lines) - 1
	lastCol := len([]rune(editor.Buf.Lines[lastLine]))

	del := &undo.DeleteSelectionCommand{
		StartLine: 0, StartCol: 0,
		EndLine: lastLine, EndCol: lastCol,
	}
	paste := &undo.PasteCommand{
		Line: 0, Col: 0,
		Text: formatted, Suffix: "",
	}
	editor.ExecCommand(&undo.BatchCommand{Commands: []undo.EditCommand{del, paste}})
	editor.FlushOnChange()

	return nil
}
