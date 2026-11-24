package plugin

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

func replacePluginDirectory(src, dst string) error {
	return replacePluginDirectoryWithRename(src, dst, os.Rename)
}

func replacePluginDirectoryWithRename(src, dst string, rename func(string, string) error) error {
	stage, err := os.MkdirTemp(filepath.Dir(dst), ".ttt-plugin-update-*")
	if err != nil {
		return fmt.Errorf("stage plugin update: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			if err := os.RemoveAll(stage); err != nil {
				slog.Warn("clean up plugin update", "path", stage, "error", err)
			}
		}
	}()
	next := filepath.Join(stage, "next")
	if err := copyDir(src, next); err != nil {
		return fmt.Errorf("stage plugin files: %w", err)
	}
	previous := filepath.Join(stage, "previous")
	if err := rename(dst, previous); err != nil {
		return fmt.Errorf("back up installed plugin: %w", err)
	}
	if err := rename(next, dst); err != nil {
		if rollbackErr := rename(previous, dst); rollbackErr != nil {
			// The backup is the only remaining copy of the installed plugin.
			cleanup = false
			return errors.Join(err, fmt.Errorf("restore plugin from %s: %w", previous, rollbackErr))
		}
		return fmt.Errorf("replace plugin: %w", err)
	}
	return nil
}
