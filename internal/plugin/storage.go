package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

const maxStorageBytes = 1 << 20

type pluginStore struct {
	path    string
	data    map[string]any
	loaded  bool
	modTime time.Time
	size    int64
}

func storagePath(dir, name string) (string, error) {
	if dir == "" {
		return "", errors.New("plugin storage is not available")
	}
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name {
		return "", fmt.Errorf("plugin name %q cannot be used for storage", name)
	}
	return filepath.Join(dir, name+".json"), nil
}

// Reloads when mtime or size changed so another ttt process's keys survive our
// next write. No cross-process lock: simultaneous writes can still lose one.
func (s *pluginStore) read() (map[string]any, error) {
	info, err := os.Stat(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		s.data, s.loaded, s.modTime, s.size = map[string]any{}, true, time.Time{}, 0
		return s.data, nil
	}
	if s.loaded && info.ModTime().Equal(s.modTime) && info.Size() == s.size {
		return s.data, nil
	}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	data := map[string]any{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("read plugin storage: %w", err)
	}
	if data == nil {
		data = map[string]any{}
	}
	s.data, s.loaded, s.modTime, s.size = data, true, info.ModTime(), info.Size()
	return s.data, nil
}

func (s *pluginStore) get(key string) (any, bool, error) {
	data, err := s.read()
	if err != nil {
		return nil, false, err
	}
	v, ok := data[key]
	return v, ok, nil
}

func (s *pluginStore) set(key string, value any) error {
	data, err := s.read()
	if err != nil {
		return err
	}
	next := make(map[string]any, len(data)+1)
	for k, v := range data {
		next[k] = v
	}
	if value == nil {
		if _, ok := next[key]; !ok {
			return nil
		}
		delete(next, key)
	} else {
		next[key] = value
	}
	if err := s.write(next); err != nil {
		return err
	}
	s.data = next
	if info, err := os.Stat(s.path); err == nil {
		s.modTime, s.size = info.ModTime(), info.Size()
	} else {
		s.modTime, s.size = time.Time{}, 0
	}
	return nil
}

func (s *pluginStore) keys() ([]string, error) {
	data, err := s.read()
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

func (s *pluginStore) write(data map[string]any) error {
	if len(data) == 0 {
		if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if len(raw) > maxStorageBytes {
		return fmt.Errorf("plugin storage limit exceeded (%d bytes, max %d)", len(raw), maxStorageBytes)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(s.path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
