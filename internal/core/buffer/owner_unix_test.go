//go:build unix

package buffer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func ownerOf(t *testing.T, name string) (uid, gid uint32) {
	t.Helper()
	info, err := os.Stat(name)
	if err != nil {
		t.Fatal(err)
	}
	st := info.Sys().(*syscall.Stat_t)
	return st.Uid, st.Gid
}

// otherGroup returns a group the test process belongs to besides its primary
// one, so a file can be given a group a new file would not get.
func otherGroup(t *testing.T) int {
	t.Helper()
	if os.Geteuid() == 0 {
		if os.Getgid() != 1 {
			return 1
		}
		return 2
	}
	groups, err := os.Getgroups()
	if err != nil {
		t.Skip(err)
	}
	for _, g := range groups {
		if g != os.Getgid() {
			return g
		}
	}
	t.Skip("the test user belongs to no supplementary group")
	return 0
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestSaveKeepsGroup(t *testing.T) {
	group := otherGroup(t)
	dir := t.TempDir()
	fname := filepath.Join(dir, "shared.txt")
	if err := os.WriteFile(fname, []byte("old\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(fname, -1, group); err != nil {
		t.Fatal(err)
	}
	b := &Buffer{Lines: []string{"new", ""}}
	if err := b.SaveFile(fname); err != nil {
		t.Fatal(err)
	}
	if _, gid := ownerOf(t, fname); gid != uint32(group) {
		t.Errorf("group = %d, want %d", gid, group)
	}
	if data, _ := os.ReadFile(fname); string(data) != "new\n" {
		t.Errorf("saved %q", data)
	}
}

func TestSaveKeepsOwnerAsRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root to give the file another owner")
	}
	dir := t.TempDir()
	fname := filepath.Join(dir, "owned.txt")
	if err := os.WriteFile(fname, []byte("old\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(fname, 1, 2); err != nil {
		t.Fatal(err)
	}
	b := &Buffer{Lines: []string{"new", ""}}
	if err := b.SaveFile(fname); err != nil {
		t.Fatal(err)
	}
	if uid, gid := ownerOf(t, fname); uid != 1 || gid != 2 {
		t.Errorf("owner = %d:%d, want 1:2", uid, gid)
	}
}

func TestSaveAsNewFileUsesNewFileOwnership(t *testing.T) {
	group := otherGroup(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(source, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(source, -1, group); err != nil {
		t.Fatal(err)
	}
	b := &Buffer{}
	if err := b.LoadFile(source); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "copy.txt")
	if err := b.SaveFile(dest); err != nil {
		t.Fatal(err)
	}
	if uid, gid := ownerOf(t, dest); uid != uint32(os.Geteuid()) || gid != uint32(os.Getegid()) {
		t.Errorf("new file owner = %d:%d, want %d:%d", uid, gid, os.Geteuid(), os.Getegid())
	}
}

func TestSaveWritesInPlaceWhenOwnerCannotBeKept(t *testing.T) {
	group := otherGroup(t)
	chown = func(string, int, int) error { return &os.PathError{Op: "chown", Err: syscall.EPERM} }
	t.Cleanup(func() { chown = os.Chown })

	dir := t.TempDir()
	fname := filepath.Join(dir, "shared.txt")
	if err := os.WriteFile(fname, []byte("old contents\n"), 0o664); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(fname, 0o664); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(fname, -1, group); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(fname)
	if err != nil {
		t.Fatal(err)
	}
	b := &Buffer{Lines: []string{"new", ""}}
	if err := b.SaveFile(fname); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(fname)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) {
		t.Error("the file was replaced instead of written in place")
	}
	if _, gid := ownerOf(t, fname); gid != uint32(group) {
		t.Errorf("group = %d, want %d", gid, group)
	}
	if after.Mode().Perm() != 0o664 {
		t.Errorf("mode = %o, want 664", after.Mode().Perm())
	}
	if data, _ := os.ReadFile(fname); string(data) != "new\n" {
		t.Errorf("saved %q", data)
	}
	if names := dirNames(t, dir); len(names) != 1 {
		t.Errorf("left files behind: %v", names)
	}
	if b.DiskChanged(fname) {
		t.Error("DiskChanged right after an in-place save")
	}
}

func TestWriteInPlaceKeepsRecoveryCopyOnFailure(t *testing.T) {
	dir := t.TempDir()
	fname := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(fname, []byte("old contents\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Reading a directory fails after the target has been opened and truncated.
	unreadable := t.TempDir()
	err := writeInPlace(fname, unreadable)
	if err == nil {
		t.Fatal("expected an error")
	}
	var recovery string
	for _, name := range dirNames(t, dir) {
		if strings.HasPrefix(name, ".file.txt.ttt-recovery-") {
			recovery = filepath.Join(dir, name)
		}
	}
	if recovery == "" {
		t.Fatalf("no recovery copy kept: %v", dirNames(t, dir))
	}
	if !strings.Contains(err.Error(), recovery) {
		t.Errorf("error %q does not name the recovery copy %s", err, recovery)
	}
	if data, _ := os.ReadFile(recovery); string(data) != "old contents\n" {
		t.Errorf("recovery copy holds %q", data)
	}
	info, err := os.Stat(recovery)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("recovery copy mode = %o, want 600", info.Mode().Perm())
	}
}

func TestWriteInPlaceLeavesNoRecoveryCopyWhenTargetIsNotWritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write a read-only file")
	}
	dir := t.TempDir()
	fname := filepath.Join(dir, "file.txt")
	src := filepath.Join(t.TempDir(), "src.txt")
	if err := os.WriteFile(fname, []byte("old\n"), 0o444); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeInPlace(fname, src); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("err = %v, want a permission error", err)
	}
	if names := dirNames(t, dir); len(names) != 1 {
		t.Errorf("left files behind: %v", names)
	}
	if data, _ := os.ReadFile(fname); string(data) != "old\n" {
		t.Errorf("file changed to %q", data)
	}
}
