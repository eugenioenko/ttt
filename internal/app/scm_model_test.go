package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/scm"
	"github.com/eugenioenko/ttt/internal/term"
)

type modelRepository struct {
	testSCMRepository
	changes []scm.Change
}

func (r *modelRepository) Changes(context.Context) ([]scm.Change, error) { return r.changes, nil }

func TestSCMModelRendersWithoutGitTypesOrStaging(t *testing.T) {
	root := t.TempDir()
	repo := &modelRepository{testSCMRepository: testSCMRepository{root: root}, changes: []scm.Change{
		{Path: "src/changed.go", Kind: scm.Modified, Detail: "U"},
		{Path: "src/conflict.go", Kind: scm.Conflicted},
		{Path: "src/unknown.go", Kind: scm.Unknown, Detail: "M", Group: "pending"},
		{Path: "src/unknown.go", Kind: scm.Unknown, Group: scm.WorkingTree},
	}}
	cp := NewChangesPanelWithProvider(&testSCMProvider{repo: repo}, root)
	cp.Tree.Config.OnSelect = nil
	state := NewRepositoryState(cp, []string{root})
	state.requested = 1
	state.HandleStatus(state.readStatus(context.Background(), []string{root}, 1))
	state.SetActiveFile(filepath.Join(root, "src", "changed.go"))
	if gotRoot, label := state.ActiveRepository(); gotRoot != root || label != "working-change" {
		t.Fatalf("active identity: %q %q", gotRoot, label)
	}
	if cp.TotalChanges() != 4 || cp.supportsStaging(root) {
		t.Fatalf("provider state: %+v", cp.groups)
	}
	styles := explorerGitStyles(cp.groups)
	if styles[filepath.Join(root, "src", "changed.go")] != term.StyleWarning || styles[filepath.Join(root, "src", "conflict.go")] != term.StyleGitConflict || styles[filepath.Join(root, "src", "unknown.go")] != term.StyleDefault {
		t.Fatalf("decoration parsed provider diagnostics: %v", styles)
	}
	id := workingChangeNodeID(workNodeFile, root, "src/unknown.go", "pending")
	if nodeWithID(cp.Tree.Config.Items, workingNodeID(workNodeFile, root, "src/unknown.go", false)) == nil {
		t.Fatal("provider groups lost distinct path identities")
	}
	for _, view := range []string{config.GitFileViewList, config.GitFileViewTree, config.GitFileViewList} {
		cp.SetFileView(view)
		if !revealTreeSelection(cp.Tree, id) {
			t.Fatalf("missing neutral change in view %s", view)
		}
		dir, change, ok := cp.SelectedFile()
		if !ok || dir != root || change.Kind != scm.Unknown || change.Group != "pending" || change.Detail != "M" {
			t.Fatalf("selection lost provider data: %+v", change)
		}
	}
	repo.changes[0].Kind = scm.Deleted
	if cp.groups[0].Changes[0].Kind != scm.Modified {
		t.Fatal("provider mutated the published snapshot")
	}
}

func TestSCMCurrentChangesIdentityIncludesKindDetailAndGroup(t *testing.T) {
	original := scm.Change{Path: "file.txt", Kind: scm.Modified, Group: scm.WorkingTree}
	input := newCurrentChangesInput("opaque", []scm.Change{original})
	for _, change := range []scm.Change{
		{Path: "file.txt", Kind: scm.Deleted, Group: scm.WorkingTree},
		{Path: "file.txt", Kind: scm.Modified, Group: scm.Staged},
		{Path: "file.txt", Kind: scm.Modified, Group: "pending"},
		{Path: "file.txt", Kind: scm.Modified, Group: scm.WorkingTree, Detail: "changed"},
	} {
		if got := newCurrentChangesInput("opaque", []scm.Change{change}); got.identity == input.identity {
			t.Fatalf("identity ignores metadata: %+v", change)
		}
	}
}
