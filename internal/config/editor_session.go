package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const MaxSessionTabs = 128
const maxEditorSessions = 32
const maxEditorSessionBytes = 1 << 20
const editorSessionAge = 30 * 24 * time.Hour

type EditorTabState struct {
	Path    string `json:"path"`
	Line    int    `json:"line,omitempty"`
	Col     int    `json:"col,omitempty"`
	TopLine int    `json:"topLine,omitempty"`
	LeftCol int    `json:"leftCol,omitempty"`
	Pinned  bool   `json:"pinned,omitempty"`
	Active  bool   `json:"active,omitempty"`
}

func editorSessionPath(key string) string {
	hash := sha256.Sum256([]byte(key))
	return filepath.Join(ConfigFilePath("sessions"), hex.EncodeToString(hash[:])+".json")
}

func LoadEditorSession(key string) ([]EditorTabState, error) {
	path := editorSessionPath(key)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxEditorSessionBytes || time.Since(info.ModTime()) > editorSessionAge {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxEditorSessionBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxEditorSessionBytes {
		return nil, nil
	}
	var tabs []EditorTabState
	if err := json.Unmarshal(data, &tabs); err != nil {
		return nil, err
	}
	return boundedSessionTabs(tabs), nil
}

func boundedSessionTabs(tabs []EditorTabState) []EditorTabState {
	if len(tabs) > MaxSessionTabs {
		for _, tab := range tabs[MaxSessionTabs:] {
			if tab.Active {
				return append(append([]EditorTabState{}, tabs[:MaxSessionTabs-1]...), tab)
			}
		}
	}
	return tabs[:min(len(tabs), MaxSessionTabs)]
}

func SaveEditorSession(key string, tabs []EditorTabState) error {
	path := editorSessionPath(key)
	if len(tabs) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return PruneEditorSessions(time.Now())
	}
	data, err := json.Marshal(boundedSessionTabs(tabs))
	if err != nil {
		return err
	}
	if len(data) > maxEditorSessionBytes {
		return fmt.Errorf("editor session exceeds %d bytes", maxEditorSessionBytes)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".session-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(data)
	if err := errors.Join(writeErr, f.Close()); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return err
	}
	return PruneEditorSessions(time.Now())
}

func editorSessionFiles() ([]os.DirEntry, error) {
	entries, err := os.ReadDir(ConfigFilePath("sessions"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var files []os.DirEntry
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || len(name) != 69 || !strings.HasSuffix(name, ".json") {
			continue
		}
		if _, err := hex.DecodeString(name[:64]); err == nil {
			files = append(files, entry)
		}
	}
	return files, nil
}

func ClearEditorSessions() error {
	files, err := editorSessionFiles()
	if err != nil {
		return err
	}
	var errs []error
	for _, file := range files {
		if err := os.Remove(filepath.Join(ConfigFilePath("sessions"), file.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func PruneEditorSessions(now time.Time) error {
	files, err := editorSessionFiles()
	if err != nil {
		return err
	}
	var keep []os.FileInfo
	for _, file := range files {
		info, err := file.Info()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		if now.Sub(info.ModTime()) <= editorSessionAge && info.Mode().IsRegular() && info.Size() <= maxEditorSessionBytes {
			keep = append(keep, info)
			continue
		}
		if err := os.Remove(filepath.Join(ConfigFilePath("sessions"), file.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	sort.Slice(keep, func(i, j int) bool {
		if keep[i].ModTime().Equal(keep[j].ModTime()) {
			return keep[i].Name() < keep[j].Name()
		}
		return keep[i].ModTime().After(keep[j].ModTime())
	})
	for _, file := range keep[min(len(keep), maxEditorSessions):] {
		if err := os.Remove(filepath.Join(ConfigFilePath("sessions"), file.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
