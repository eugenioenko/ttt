package filewrite

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

type Staged struct {
	File   *os.File
	target string
	path   string
}

// Stage writes and syncs a sibling temporary file. The callback may apply
// access metadata before writing, but must not close the file. Callers must
// defer Discard and apply any remaining metadata through File before Commit.
func Stage(target string, mode os.FileMode, write func(*os.File) error) (*Staged, error) {
	f, err := createTemp(filepath.Dir(target), mode)
	if err != nil {
		return nil, err
	}
	staged := &Staged{File: f, target: target, path: f.Name()}
	if err := write(f); err != nil {
		staged.Discard()
		return nil, err
	}
	if err := f.Sync(); err != nil {
		staged.Discard()
		return nil, err
	}
	return staged, nil
}

// Commit closes the file before replacement. A successful replace must consume
// the temporary path; a failed replace must leave cleanup to Discard.
func (s *Staged) Commit(replace func(string, string) error) error {
	defer s.Discard()
	if err := s.File.Close(); err != nil {
		return err
	}
	if err := replace(s.path, s.target); err != nil {
		return err
	}
	s.path = ""
	return nil
}

func (s *Staged) Discard() {
	if s.path != "" {
		_ = s.File.Close()
		_ = os.Remove(s.path)
		s.path = ""
	}
}

func createTemp(dir string, mode os.FileMode) (*os.File, error) {
	for range 100 {
		var suffix [12]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return nil, err
		}
		name := filepath.Join(dir, ".ttt-"+hex.EncodeToString(suffix[:])+".tmp")
		file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
		if os.IsExist(err) {
			continue
		}
		return file, err
	}
	return nil, fmt.Errorf("could not create temporary file")
}
