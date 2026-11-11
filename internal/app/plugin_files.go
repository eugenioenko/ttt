package app

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/config"
)

func replacePluginContributionFiles(kind, name string, files map[string][]byte) error {
	if !isPlainName(name) {
		return errInvalidPluginName
	}
	dir := filepath.Join(kind, "plugins", name)
	if len(files) == 0 {
		if _, err := os.Lstat(filepath.Join(config.UserConfigDir(), dir)); os.IsNotExist(err) {
			return nil
		}
	} else if err := os.MkdirAll(config.UserConfigDir(), 0755); err != nil {
		return err
	}
	root, err := os.OpenRoot(config.UserConfigDir())
	if err != nil {
		return err
	}
	defer root.Close()
	for _, dir := range []string{kind, filepath.Join(kind, "plugins")} {
		if err := root.Mkdir(dir, 0755); err != nil && !os.IsExist(err) {
			return err
		}
		info, err := root.Lstat(dir)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("contribution directory %s must not be a symlink", dir)
		}
	}
	if len(files) == 0 {
		return root.RemoveAll(dir)
	}
	stage := filepath.Join(kind, "plugins", ".stage-"+rand.Text())
	if err := root.Mkdir(stage, 0755); err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			root.RemoveAll(stage)
		}
	}()
	next := filepath.Join(stage, "next")
	if err := root.Mkdir(next, 0755); err != nil {
		return err
	}
	for filename, data := range files {
		if err := root.WriteFile(filepath.Join(next, filename), data, 0644); err != nil {
			return err
		}
	}
	backup := filepath.Join(stage, "previous")
	err = root.Rename(dir, backup)
	hadPrevious := err == nil
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := root.Rename(next, dir); err != nil {
		if hadPrevious {
			if rollbackErr := root.Rename(backup, dir); rollbackErr != nil {
				cleanup = false
				return errors.Join(err, fmt.Errorf("restore contributions from %s: %w", backup, rollbackErr))
			}
		}
		return err
	}
	return nil
}
