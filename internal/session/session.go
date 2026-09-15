// Package session persists auto-sessions: which files were open, where, and
// in which workspace. One session per directory key (usually the primary
// workspace folder), stored as JSON under the config dir. Only paths and
// 0-based cursor positions are stored — unsaved text is never preserved,
// matching the quit gate that blocks on dirty buffers.
package session

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type TabState struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Col  int    `json:"col"`
}

const sessionVersion = 1

type Session struct {
	Version int        `json:"version"`
	Folders []string   `json:"folders"`
	Tabs    []TabState `json:"tabs"`
	Active  string     `json:"active,omitempty"`
}

// pathEscaper flattens a directory key into one filename. % goes first: the
// replacements never rescan, so encoded output can't collide with raw input.
var pathEscaper = strings.NewReplacer("%", "%25", "/", "%", "\\", "%", ":", "%")

// PathForDir maps a directory key to its session file inside configDir.
// The key is cleaned and resolved through symlinks best-effort so spellings
// of the same directory share one session.
func PathForDir(configDir, dirKey string) string {
	key := filepath.Clean(dirKey)
	if abs, err := filepath.EvalSymlinks(key); err == nil {
		key = abs
	}
	return filepath.Join(configDir, "sessions", pathEscaper.Replace(key)+".json")
}

func Save(configDir, dirKey string, s Session) error {
	if dirKey == "" {
		return errors.New("session: empty directory key")
	}
	dir := filepath.Join(configDir, "sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	s.Version = sessionVersion
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	// Atomic write: a crash mid-save must not truncate the last good session.
	tmp, err := os.CreateTemp(dir, ".tmp-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0600); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, PathForDir(configDir, dirKey))
}

func Load(configDir, dirKey string) (Session, error) {
	var s Session
	if dirKey == "" {
		return s, errors.New("session: empty directory key")
	}
	data, err := os.ReadFile(PathForDir(configDir, dirKey))
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return Session{}, err
	}
	return s, nil
}
