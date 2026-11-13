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
	control := filepath.Join(dir, "control.txt")
	if err := os.WriteFile(control, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	wantUID, wantGID := ownerOf(t, control)
	if uid, gid := ownerOf(t, dest); uid != wantUID || gid != wantGID {
		t.Errorf("new file owner = %d:%d, want %d:%d like any new file in the directory", uid, gid, wantUID, wantGID)
	}
}

func TestSaveWritesInPlaceWhenOwnerCannotBeKept(t *testing.T) {
	group := otherGroup(t)
	chown = func(*os.File, int, int) error { return &os.PathError{Op: "chown", Err: syscall.EPERM} }
	t.Cleanup(func() { chown = (*os.File).Chown })

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

func TestSaveDoesNotFollowSwappedTempPath(t *testing.T) {
	group := otherGroup(t)
	dir := t.TempDir()
	victim := filepath.Join(t.TempDir(), "victim.txt")
	if err := os.WriteFile(victim, []byte("private\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(victim, 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "shared.txt")
	if err := os.WriteFile(target, []byte("old\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0o666); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(target, -1, group); err != nil {
		t.Fatal(err)
	}
	// Once the temp file belongs to the target's owner, that owner can swap
	// its path for a symlink before the save finishes.
	chown = func(f *os.File, uid, gid int) error {
		err := f.Chown(uid, gid)
		os.Remove(f.Name())
		if linkErr := os.Symlink(victim, f.Name()); linkErr != nil {
			t.Fatal(linkErr)
		}
		return err
	}
	t.Cleanup(func() { chown = (*os.File).Chown })

	b := &Buffer{Lines: []string{"new", ""}}
	_ = b.SaveFile(target)
	info, err := os.Stat(victim)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("save changed an unrelated file's mode to %o through the temp path", info.Mode().Perm())
	}
	if data, _ := os.ReadFile(victim); string(data) != "private\n" {
		t.Errorf("save changed an unrelated file to %q", data)
	}
}

func TestCheckSameFileDetectsReplacement(t *testing.T) {
	dir := t.TempDir()
	fname := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(fname, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(fname)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := checkSameFile(f, fname); err != nil {
		t.Fatalf("unreplaced file: %v", err)
	}
	other := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(other, []byte("new\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(other, fname); err != nil {
		t.Fatal(err)
	}
	if err := checkSameFile(f, fname); err == nil {
		t.Error("expected an error once another file was renamed over the path")
	}
}

func TestInPlaceSaveKeepsTempFilePrivate(t *testing.T) {
	group := otherGroup(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(target, []byte("old\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(target, -1, group); err != nil {
		t.Fatal(err)
	}
	var temp *os.File
	chown = func(f *os.File, _, _ int) error {
		var err error
		if temp, err = os.Open(f.Name()); err != nil {
			t.Fatal(err)
		}
		return &os.PathError{Op: "chown", Err: syscall.EPERM}
	}
	t.Cleanup(func() { chown = (*os.File).Chown })

	b := &Buffer{Lines: []string{"new", ""}}
	if err := b.SaveFile(target); err != nil {
		t.Fatal(err)
	}
	defer temp.Close()
	info, err := temp.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("temp file holding the new contents had mode %o, want 600", info.Mode().Perm())
	}
}
