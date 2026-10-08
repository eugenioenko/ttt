package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const maxStorageBytes = 1 << 20

type pluginStore struct {
	path   string
	data   map[string]any
	loaded bool
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

func (s *pluginStore) load() error {
	if s.loaded {
		return nil
	}
	s.data = map[string]any{}
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.loaded = true
			return nil
		}
		return err
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return fmt.Errorf("read plugin storage: %w", err)
	}
	if s.data == nil {
		s.data = map[string]any{}
	}
	s.loaded = true
	return nil
}

func (s *pluginStore) get(key string) (any, bool, error) {
	if err := s.load(); err != nil {
		return nil, false, err
	}
	v, ok := s.data[key]
	return v, ok, nil
}

func (s *pluginStore) set(key string, value any) error {
	if err := s.load(); err != nil {
		return err
	}
	next := make(map[string]any, len(s.data)+1)
	for k, v := range s.data {
		next[k] = v
	}
	if value == nil {
		delete(next, key)
	} else {
		next[key] = value
	}
	if err := s.write(next); err != nil {
		return err
	}
	s.data = next
	return nil
}

func (s *pluginStore) keys() ([]string, error) {
	if err := s.load(); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
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
