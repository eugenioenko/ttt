package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/i18n"
)

const formatterTimeout = 5 * time.Second

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
		a.StatusWarn(i18n.Format("status.format", i18n.Args{"error": formatterErrorMessage(err)}))
	}
}

func (a *App) FormatExternalOnSave(path string) bool {
	cmd := a.formatterForFile(path)
	if cmd == "" {
		return false
	}
	if err := a.applyExternalFormatter(cmd, path); err != nil {
		a.StatusWarn(i18n.Format("status.formatOnSave", i18n.Args{"error": formatterErrorMessage(err)}))
		// A timeout must continue saving, not start another formatter through LSP.
		return errors.Is(err, context.DeadlineExceeded)
	}
	return true
}

func formatterErrorMessage(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return i18n.Format("formatter.timeout", i18n.Args{"seconds": int(formatterTimeout / time.Second)})
	}
	return err.Error()
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

	parts := strings.Fields(cmdStr)
	args := parts[:0]
	for _, part := range parts {
		part = strings.ReplaceAll(part, "{file}", filePath)
		if part != "" {
			args = append(args, part)
		}
	}
	if len(args) == 0 {
		return fmt.Errorf("empty formatter command")
	}

	ctx, cancel := context.WithTimeout(context.Background(), formatterTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	configureFormatterProcess(cmd)
	// Detached descendants must not keep the output pipes open indefinitely.
	cmd.WaitDelay = 100 * time.Millisecond
	input := original
	if editor.Buf.UTF8BOM {
		input = buffer.UTF8ByteOrderMark + input
	}
	cmd.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}

	formatted := stdout.String()
	// Without BOM metadata, an existing leading U+FEFF is editable text.
	if editor.Buf.UTF8BOM || !strings.HasPrefix(original, buffer.UTF8ByteOrderMark) {
		formatted = strings.TrimPrefix(formatted, buffer.UTF8ByteOrderMark)
	}
	formatted = strings.ReplaceAll(formatted, "\r\n", "\n")
	if formatted == original {
		return nil
	}
	if formatted == "" {
		return errors.New(i18n.Text("formatter.emptyOutput"))
	}
	comparison := original
	if editor.Buf.ShowTrailingNewline {
		formatted = strings.TrimSuffix(formatted, "\n")
		comparison = strings.TrimSuffix(comparison, "\n")
	} else if editor.Buf.InsertFinalNewline {
		// Save adds a newline only when the buffer has no trailing empty line.
		// Keep multiple newlines intact so their blank lines survive saving.
		if !strings.HasSuffix(formatted, "\n\n") {
			formatted = strings.TrimSuffix(formatted, "\n")
		}
		if !strings.HasSuffix(comparison, "\n\n") {
			comparison = strings.TrimSuffix(comparison, "\n")
		}
	}
	if formatted == comparison {
		return nil
	}
	if formatted == "" {
		return errors.New(i18n.Text("formatter.emptyOutput"))
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
