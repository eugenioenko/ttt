package app

import (
	"github.com/eugenioenko/ttt/internal/plugin"
	"testing"
)

func TestPluginChangesUsesNativeMultiFileView(t *testing.T) {
	a := prepareListenTestApp(t)
	p := &plugin.Plugin{Name: "review"}
	a.wirePluginChanges(p)
	opts := plugin.ChangesOptions{ID: "pr-1", Title: "PR #1", Description: "A change", Diff: `diff --git a/first.go b/first.go
new file mode 100644
--- /dev/null
+++ b/first.go
@@ -0,0 +1 @@
+package first
diff --git a/second.go b/second.go
--- a/second.go
+++ b/second.go
@@ -1 +1 @@
-old
+new
`}
	id, err := p.OpenChanges(opts)
	if err != nil {
		t.Fatal(err)
	}
	detail := a.EditorGroup.CommitDetailWidgetByTab(id)
	if detail == nil || len(detail.Files) != 2 || detail.Files[0].Status != "A" || len(detail.Files[1].Diff.Hunks) != 1 {
		t.Fatal("changes did not open as a native multi-file diff")
	}
	tabs := a.EditorGroup.TabCount()
	if _, err := p.OpenChanges(opts); err != nil || a.EditorGroup.TabCount() != tabs {
		t.Fatal("reopening the same changes duplicated the tab")
	}
	for _, invalid := range []string{"invalid", "diff --git a/a b/a", "diff --git a/a b/a\n--- a/a\n+++ b/a"} {
		if _, err := p.OpenChanges(plugin.ChangesOptions{Diff: invalid}); err == nil {
			t.Fatalf("invalid diff was accepted: %q", invalid)
		}
	}
	a.ShowConfirmDialog("Keep focus", []string{"Close"}, []func(){a.DismissDialog})
	focused := a.Root.Focused
	if _, err := p.OpenChanges(opts); err != nil || a.Root.Focused != focused {
		t.Fatal("opening changes moved focus out of a modal dialog")
	}
}

func TestPluginChangesPreservesRenamePaths(t *testing.T) {
	a := prepareListenTestApp(t)
	p := &plugin.Plugin{Name: "review"}
	a.wirePluginChanges(p)
	id, err := p.OpenChanges(plugin.ChangesOptions{ID: "rename", Title: "Rename", Diff: "diff --git \"a/old\\tname.txt\" \"b/new name.txt\"\nsimilarity index 100%\nrename from \"old\\tname.txt\"\nrename to new name.txt\n"})
	if err != nil {
		t.Fatal(err)
	}
	files := a.EditorGroup.CommitDetailWidgetByTab(id).Files
	if len(files) != 1 || files[0].Path != "new name.txt" || files[0].OldPath != "old\tname.txt" || files[0].Status != "R" {
		t.Fatalf("files=%+v", files)
	}
}

func TestPluginChangesIDsDoNotCollide(t *testing.T) {
	a := prepareListenTestApp(t)
	first := &plugin.Plugin{Name: "a"}
	second := &plugin.Plugin{Name: "a:changes:b"}
	a.wirePluginChanges(first)
	a.wirePluginChanges(second)
	opts := plugin.ChangesOptions{ID: "b:changes:c", Title: "First", Diff: "diff --git a/a b/a\n@@ -1 +1 @@\n-old\n+new"}
	firstID, err := first.OpenChanges(opts)
	if err != nil {
		t.Fatal(err)
	}
	opts.ID, opts.Title = "c", "Second"
	secondID, err := second.OpenChanges(opts)
	if err != nil {
		t.Fatal(err)
	}
	if firstID == secondID {
		t.Fatal("distinct plugins and requests shared a tab ID")
	}
	if a.EditorGroup.CommitDetailWidgetByTab(firstID).Header != "First" || a.EditorGroup.CommitDetailWidgetByTab(secondID).Header != "Second" {
		t.Fatal("opening a second plugin's changes replaced the first tab")
	}
	tabs := a.EditorGroup.TabCount()
	if id, err := second.OpenChanges(opts); err != nil || id != secondID || a.EditorGroup.TabCount() != tabs {
		t.Fatal("reopening the same request did not reuse its tab")
	}
}
