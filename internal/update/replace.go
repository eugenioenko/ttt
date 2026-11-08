package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var ErrNotWritable = errors.New("executable directory is not writable")

func ParseChecksums(r io.Reader) (map[string]string, error) {
	checksums := map[string]string{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 2 || len(fields[0]) != sha256.Size*2 {
			return nil, fmt.Errorf("invalid checksum line %q", scanner.Text())
		}
		if _, err := hex.DecodeString(fields[0]); err != nil {
			return nil, err
		}
		name := strings.TrimPrefix(fields[1], "*")
		if _, duplicate := checksums[name]; duplicate {
			return nil, fmt.Errorf("duplicate checksum for %s", name)
		}
		checksums[name] = strings.ToLower(fields[0])
	}
	return checksums, scanner.Err()
}

func (s Service) replace(ctx context.Context, release Release, path, name string, rename func(string, string) error) error {
	if rename == nil {
		rename = os.Rename
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ttt-update-*")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotWritable, err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	checksumURL, err := release.assetURL("checksums.txt")
	if err != nil {
		return err
	}
	body, err := s.get(ctx, checksumURL)
	if err != nil {
		return err
	}
	checksums, err := ParseChecksums(io.LimitReader(body, 1<<20))
	body.Close()
	if err != nil {
		return err
	}
	want := checksums[name]
	if want == "" {
		return fmt.Errorf("missing checksum for %s", name)
	}
	assetURL, err := release.assetURL(name)
	if err != nil {
		return err
	}
	body, err = s.get(ctx, assetURL)
	if err != nil {
		return err
	}
	hash := sha256.New()
	_, err = io.Copy(io.MultiWriter(tmp, hash), body)
	body.Close()
	if err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != want {
		return fmt.Errorf("SHA-256 mismatch for %s", name)
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	old := path + ".old"
	// Never overwrite a backup left by a failed rollback or a still-running Windows binary.
	if _, err := os.Lstat(old); err == nil {
		return fmt.Errorf("backup already exists: %s", old)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := rename(path, old); err != nil {
		return err
	}
	if err := rename(tmp.Name(), path); err != nil {
		if rollback := rename(old, path); rollback != nil {
			return fmt.Errorf("install: %w; rollback: %v (original saved at %s)", err, rollback, old)
		}
		return err
	}
	if err := os.Remove(old); err != nil && runtime.GOOS != "windows" {
		return err
	}
	return nil
}

func CleanupOldExecutable() {
	if runtime.GOOS != "windows" {
		return
	}
	if path, err := os.Executable(); err == nil {
		if path, err = filepath.EvalSymlinks(path); err == nil {
			_ = os.Remove(path + ".old")
		}
	}
}
