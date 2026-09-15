package session

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := Session{
		Folders: []string{"/proj", "/proj/other"},
		Tabs: []TabState{
			{Path: "/proj/a.go", Line: 12, Col: 4},
			{Path: "/proj/b.go"},
		},
		Active: "/proj/b.go",
	}
	if err := Save(dir, "/proj", want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(dir, "/proj")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Version != 1 {
		t.Errorf("version = %d, want 1", got.Version)
	}
	want.Version = 1
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip = %+v, want %+v", got, want)
	}
}

func TestKeysDifferPerDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, "/a", Session{Folders: []string{"/a"}}); err != nil {
		t.Fatal(err)
	}
	if err := Save(dir, "/b", Session{Folders: []string{"/b"}}); err != nil {
		t.Fatal(err)
	}
	a, err := Load(dir, "/a")
	if err != nil || len(a.Folders) != 1 || a.Folders[0] != "/a" {
		t.Errorf("cwd /a loaded %+v, err %v", a, err)
	}
}

func TestLoadMissingOrCorrupt(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(dir, "/nothing"); err == nil {
		t.Error("missing session should error")
	}
	// A failed load must not create anything: reads have no side effects.
	if _, err := os.Stat(filepath.Join(dir, "sessions")); !os.IsNotExist(err) {
		t.Error("load created session directories")
	}
	bad := PathForDir(dir, "/bad")
	os.MkdirAll(filepath.Dir(bad), 0755)
	os.WriteFile(bad, []byte("{nope"), 0644)
	if _, err := Load(dir, "/bad"); err == nil {
		t.Error("corrupt session should error")
	}
}

func TestEmptyKeyRejected(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, "", Session{}); err == nil {
		t.Error("save with empty key should error")
	}
	if _, err := Load(dir, ""); err == nil {
		t.Error("load with empty key should error")
	}
}

func TestPercentEscapedFirst(t *testing.T) {
	dir := t.TempDir()
	if PathForDir(dir, "/a%b") == PathForDir(dir, "/a/b") {
		t.Error("escaped and raw separators collide")
	}
}
