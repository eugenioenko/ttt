package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestEditorSessionRoundTripIsolationAndBounds(t *testing.T) {
	withTempConfigDir(t)
	want := []EditorTabState{{Path: "/a.go", Line: 9, Col: 3, TopLine: 7, LeftCol: 2, Pinned: true, Active: true}}
	if err := SaveEditorSession("first", want); err != nil {
		t.Fatal(err)
	}
	if err := SaveEditorSession("second", []EditorTabState{{Path: "/b.go"}}); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadEditorSession("first"); err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("round-trip = %+v, %v", got, err)
	}
	tabs := make([]EditorTabState, MaxSessionTabs+1)
	tabs[MaxSessionTabs] = want[0]
	if err := SaveEditorSession("first", tabs); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadEditorSession("first"); err != nil || len(got) != MaxSessionTabs {
		t.Fatalf("tab bound = %d, %v", len(got), err)
	} else if got[len(got)-1] != want[0] {
		t.Fatal("tab bound dropped the active document")
	}
	if err := SaveEditorSession("first", nil); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadEditorSession("first"); err != nil || len(got) != 0 {
		t.Fatalf("cleared workspace = %+v, %v", got, err)
	}
	if got, err := LoadEditorSession("second"); err != nil || len(got) != 1 || got[0].Path != "/b.go" {
		t.Fatalf("other workspace = %+v, %v", got, err)
	}
}

func TestEditorSessionCleanupAndInvalidData(t *testing.T) {
	withTempConfigDir(t)
	now := time.Now()
	for i := range maxEditorSessions + 2 {
		key := fmt.Sprintf("workspace-%d", i)
		if err := SaveEditorSession(key, []EditorTabState{{Path: "/a.go"}}); err != nil {
			t.Fatal(err)
		}
		stamp := now.Add(time.Duration(i-maxEditorSessions-2) * time.Hour)
		if err := os.Chtimes(editorSessionPath(key), stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	expired := editorSessionPath("expired")
	if err := os.WriteFile(expired, []byte("[]"), 0600); err != nil {
		t.Fatal(err)
	}
	stamp := now.Add(-editorSessionAge - time.Hour)
	if err := os.Chtimes(expired, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	unowned := filepath.Join(ConfigFilePath("sessions"), "notes.txt")
	if err := os.WriteFile(unowned, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := PruneEditorSessions(now); err != nil {
		t.Fatal(err)
	}
	files, err := editorSessionFiles()
	if err != nil || len(files) != maxEditorSessions {
		t.Fatalf("history bound = %d, %v", len(files), err)
	}
	if _, err := os.Stat(expired); !os.IsNotExist(err) {
		t.Fatalf("expired session remains: %v", err)
	}
	path := editorSessionPath("bad")
	if err := os.WriteFile(path, []byte("{broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadEditorSession("bad"); err == nil {
		t.Fatal("invalid JSON was accepted")
	}
	if err := os.Truncate(path, maxEditorSessionBytes+1); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadEditorSession("bad"); err != nil || len(got) != 0 {
		t.Fatal("oversized session was loaded")
	}
	if err := ClearEditorSessions(); err != nil {
		t.Fatal(err)
	}
	if files, err := editorSessionFiles(); err != nil || len(files) != 0 {
		t.Fatal("clear left saved sessions")
	}
	if _, err := os.Stat(unowned); err != nil {
		t.Fatal("cleanup removed an unrelated file")
	}
}
