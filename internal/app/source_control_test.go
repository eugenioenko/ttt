package app

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/scm"
	"github.com/eugenioenko/ttt/internal/widgets"
)

type testSCMProvider struct {
	repo  scm.Repository
	calls []string
}

func (p *testSCMProvider) Discover(ctx context.Context, path string) (scm.Repository, error) {
	p.calls = append(p.calls, path)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return p.repo, nil
}

type testSCMRepository struct{ root string }

func (r testSCMRepository) Root() string { return r.root }
func (r testSCMRepository) Identity(context.Context) (scm.Identity, error) {
	return scm.Identity{Root: r.root, Label: "working-change"}, nil
}
func (r testSCMRepository) Changes(context.Context) ([]scm.Change, error) {
	return []scm.Change{{Path: "新規/[literal].txt", Status: "?", Group: scm.WorkingTree}, {Path: "renamed.txt", OldPath: "old.txt", Status: "R", Group: scm.Staged}}, nil
}
func (r testSCMRepository) Revision(context.Context) (string, error) { return "opaque-revision", nil }

type testSCMStaging struct {
	testSCMRepository
	staged, unstaged [][]string
}

func (r *testSCMStaging) Stage(ctx context.Context, paths []string) error {
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("missing mutation deadline")
	}
	r.staged = append(r.staged, append([]string(nil), paths...))
	return nil
}
func (r *testSCMStaging) Unstage(ctx context.Context, paths []string) error {
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("missing mutation deadline")
	}
	r.unstaged = append(r.unstaged, append([]string(nil), paths...))
	return nil
}

func TestRepositoryStateUsesInjectedSourceControl(t *testing.T) {
	root := t.TempDir()
	repo := &testSCMStaging{testSCMRepository: testSCMRepository{root: root}}
	provider := &testSCMProvider{repo: repo}
	cp := NewChangesPanelWithProvider(provider, root)
	state := NewRepositoryState(cp, []string{root})
	result := state.readStatus(context.Background(), []string{root, filepath.Join(root, "nested")}, 7)
	if result.Seq != 7 || len(result.Entries) != 1 {
		t.Fatalf("status: %+v", result)
	}
	entry := result.Entries[0]
	if entry.Group.Repository != repo || entry.Revision != "opaque-revision" || entry.Group.Dir != root {
		t.Fatalf("repository identity: %+v", entry)
	}
	if len(entry.Group.Staged) != 1 || entry.Group.Staged[0].OldPath != "old.txt" || len(entry.Group.Unstaged) != 1 || entry.Group.Unstaged[0].Path != "新規/[literal].txt" {
		t.Fatalf("changes: %+v", entry.Group)
	}
	identity := state.readIdentity(context.Background(), filepath.Join(root, "file.txt"), 8)
	if identity.Err != nil || identity.Seq != 8 || identity.Identity.Root != root || identity.Identity.Branch != "working-change" {
		t.Fatalf("identity: %+v", identity)
	}
	cp.groups = []changesGroup{entry.Group}
	cp.OnStatusChanged = func() {}
	cp.Tree.Config.OnSelect = nil
	cp.buildTree()
	discoveries := len(provider.calls)
	cp.stageAllInGroup(0)
	cp.unstageAllInGroup(0)
	if !reflect.DeepEqual(repo.staged, [][]string{{"新規/[literal].txt"}}) || !reflect.DeepEqual(repo.unstaged, [][]string{{"renamed.txt"}}) {
		t.Fatalf("staging calls: %v / %v", repo.staged, repo.unstaged)
	}
	if len(provider.calls) != discoveries {
		t.Fatal("mutations rediscovered an already resolved repository")
	}
}

func TestSourceControlStagingRoutesEveryEntryPoint(t *testing.T) {
	root := t.TempDir()
	repo := &testSCMStaging{testSCMRepository: testSCMRepository{root: root}}
	cp := NewChangesPanelWithProvider(&testSCMProvider{repo: repo}, root)
	cp.OnStatusChanged = func() {}
	cp.Tree.Config.OnSelect = nil
	cp.groups = readChangesGroups(cp.provider, []string{root})
	cp.buildTree()
	a := &App{Changes: cp, Reg: command.NewRegistry()}
	registerGitCommands(a)
	for _, staged := range []bool{false, true} {
		path := "新規/[literal].txt"
		inline, palette := "stage", "changes.stage"
		if staged {
			path = "renamed.txt"
			inline, palette = "unstage", "changes.unstage"
		}
		cp.Tree.SelectByID(workingNodeID(workNodeFile, root, path, staged))
		cp.handleCommand(inline, cp.Tree.Selected())
		cp.ToggleStageSelected()
		a.Reg.Execute(palette)
	}
	cp.stageAll()
	cp.unstageAll()
	if len(repo.staged) != 4 || len(repo.unstaged) != 4 {
		t.Fatalf("staging calls: %v / %v", repo.staged, repo.unstaged)
	}
}

func TestSourceControlWithoutStaging(t *testing.T) {
	root := t.TempDir()
	provider := &testSCMProvider{repo: testSCMRepository{root: root}}
	cp := NewChangesPanelWithProvider(provider, root)
	cp.Tree.Config.OnSelect = nil
	cp.groups = readChangesGroups(provider, []string{root})
	cp.buildTree()
	if cp.supportsStaging(root) {
		t.Fatal("read-only repository advertises staging")
	}
	var visit func([]*widgets.TreeNode)
	visit = func(nodes []*widgets.TreeNode) {
		for _, node := range nodes {
			for _, action := range node.Actions {
				if action.Command == "stage" || action.Command == "unstage" || action.Command == "stageAll" || action.Command == "unstageAll" || action.Command == "stageFolder" || action.Command == "unstageFolder" {
					t.Fatalf("unsupported action: %s", action.Command)
				}
			}
			visit(node.Children)
		}
	}
	visit(cp.Tree.Config.Items)
	for _, unstage := range []bool{false, true} {
		if err := cp.stagePaths(root, unstage, "file.txt"); !errors.Is(err, errors.ErrUnsupported) {
			t.Fatalf("unsupported staging: %v", err)
		}
	}
}
