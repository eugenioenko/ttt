package app

import (
	"context"
	"errors"
	"github.com/eugenioenko/ttt/internal/asynctask"
	"os"
	"path/filepath"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/scm"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func mergeViewItems() []widgets.SelectItem {
	return []widgets.SelectItem{{ID: config.MergeTwo, Label: i18n.Text("merge.two")}, {ID: config.MergeThree, Label: i18n.Text("merge.three")}, {ID: config.MergeText, Label: i18n.Text("merge.text")}}
}

func (a *App) OpenMergeEditor() {
	g := a.EditorGroup
	path := g.ActiveFilePath()
	if g.ActiveBuffer() == nil || !filepath.IsAbs(path) {
		return
	}
	buf := g.ActiveBuffer()
	version := buf.Version()
	view := a.Settings.Editor.MergeConflictView
	if view != config.MergeThree {
		view = config.MergeTwo
	}
	root, _ := a.Repository.RepositoryForPath(path)
	if root == "" {
		root = filepath.Dir(path)
	}
	source := a.Repository.repositorySource(root)
	a.startDiffOpen(func(ctx context.Context) *diffOpenResult {
		repo, err := source.resolve(ctx)
		if err != nil {
			return &diffOpenResult{Warn: err.Error()}
		}
		rel, err := filepath.Rel(repo.Root(), path)
		reader, ok := repo.(scm.ConflictReader)
		if !ok || err != nil {
			return &diffOpenResult{Warn: i18n.Text("merge.unsupported")}
		}
		inputs, err := reader.ReadConflict(ctx, filepath.ToSlash(rel))
		if errors.Is(err, errors.ErrUnsupported) {
			return &diffOpenResult{Warn: i18n.Text("merge.unsupported")}
		}
		if err != nil {
			return &diffOpenResult{Warn: err.Error()}
		}
		return &diffOpenResult{Merge: &inputs, Path: path, MergeView: view, MergeTarget: buf, MergeVersion: version, MergeGroup: g}
	})
}
func (a *App) showMerge(r *diffOpenResult) {
	if a.EditorGroup != r.MergeGroup || a.EditorGroup.ActiveFilePath() != r.Path || a.EditorGroup.ActiveBuffer() != r.MergeTarget || r.MergeTarget.Version() != r.MergeVersion {
		return
	}
	if !a.EditorGroup.OpenMerge(*r.Merge, r.MergeView, a.mergeAction, a.confirmMergeEdit) {
		a.StatusWarn(i18n.Text("merge.unsupported"))
	}
}
func (a *App) confirmMergeEdit(apply func()) {
	a.ShowConfirmDialog(i18n.Text("merge.confirm"), []string{i18n.Text("general.cancel"), i18n.Text("general.replace")}, []func(){a.DismissDialog, func() { a.DismissDialog(); apply() }})
}
func (a *App) mergeAction(id string) {
	m := a.EditorGroup.MergeEditor()
	if m == nil {
		return
	}
	switch id {
	case "view":
		a.ShowSelectDialog(i18n.Text("merge.view"), mergeViewItems(), func(id string) { m.View = id }, nil)
	case "actions":
		items := []widgets.SelectItem{
			{ID: "incoming", Label: i18n.Text("merge.chooseIncoming")}, {ID: "current", Label: i18n.Text("merge.chooseCurrent")},
			{ID: "focusNext", Label: i18n.Text("merge.focusNext")},
			{ID: "swap", Label: i18n.Text("merge.swap")}, {ID: "neither", Label: i18n.Text("merge.neither")},
			{ID: "manual", Label: i18n.Text("merge.manual")}, {ID: "complete", Label: i18n.Text("merge.complete")},
		}
		a.ShowSelectDialog(i18n.Text("merge.actions"), items, a.mergeAction, nil)
	case "next":
		m.Navigate(1)
	case "previous":
		m.Navigate(-1)
	case "complete":
		a.completeMerge()
	default:
		m.ApplyAction(id)
	}
}
func (a *App) completeMerge() {
	g := a.EditorGroup
	m := g.MergeEditor()
	if m == nil || !m.CanComplete() {
		a.StatusWarn(i18n.Text("merge.unresolved"))
		return
	}
	path := g.ActiveFilePath()
	root, _ := a.Repository.RepositoryForPath(path)
	if root == "" {
		root = filepath.Dir(path)
	}
	source := a.Repository.repositorySource(root)
	buf := g.ActiveBuffer()
	version, identity := buf.Version(), m.Identity
	active := func() bool {
		return a.EditorGroup == g && g.ActiveFilePath() == path && g.ActiveBuffer() == buf && g.MergeEditor() == m
	}
	if a.mergeCompletion == nil {
		a.mergeCompletion = NewTask[mergeStageTarget](a, asynctask.TaskOptions{})
	}
	if a.mergeCompletion.Pending() {
		return
	}
	a.mergeCompletion.Schedule(func(ctx context.Context) (mergeStageTarget, error) {
		return prepareMergeStage(ctx, source, path, identity)
	}, func(target mergeStageTarget, err error) {
		if !active() || buf.Version() != version {
			return
		}
		if err != nil {
			a.StatusWarn(err.Error())
			return
		}
		a.saveFile(func() {
			if !active() || !m.CanComplete() {
				return
			}
			saved, err := os.Stat(path)
			if err != nil {
				a.StatusError(err.Error())
				return
			}
			a.mergeCompletion.Schedule(func(ctx context.Context) (mergeStageTarget, error) {
				return target, target.stageSaved(ctx, identity, path, saved)
			}, func(_ mergeStageTarget, err error) {
				if err != nil {
					a.StatusError(err.Error())
					return
				}
				g.EndMergeSession(m)
				a.Changes.applied(nil)
			})
		})
	})
}

type mergeStageTarget struct {
	reader scm.ConflictReader
	stager scm.Staging
	rel    string
}

func prepareMergeStage(ctx context.Context, source repositorySource, path, identity string) (mergeStageTarget, error) {
	ctx, cancel := context.WithTimeout(ctx, repositoryStatusTimeout)
	defer cancel()
	repo, err := source.resolve(ctx)
	if err != nil {
		return mergeStageTarget{}, err
	}
	reader, ok := repo.(scm.ConflictReader)
	stager, canStage := repo.(scm.Staging)
	rel, err := filepath.Rel(repo.Root(), path)
	if !ok || !canStage || err != nil {
		return mergeStageTarget{}, errors.ErrUnsupported
	}
	target := mergeStageTarget{reader: reader, stager: stager, rel: filepath.ToSlash(rel)}
	return target, target.check(ctx, identity)
}

func (target mergeStageTarget) stageSaved(ctx context.Context, identity, path string, saved os.FileInfo) error {
	ctx, cancel := context.WithTimeout(ctx, repositoryStatusTimeout)
	defer cancel()
	if err := target.check(ctx, identity); err != nil {
		return err
	}
	now, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !os.SameFile(now, saved) || now.Size() != saved.Size() || !now.ModTime().Equal(saved.ModTime()) {
		return errors.New(i18n.Text("merge.changed"))
	}
	return target.stager.Stage(ctx, []string{target.rel})
}

func (t mergeStageTarget) check(ctx context.Context, identity string) error {
	got, err := t.reader.ConflictIdentity(ctx, t.rel)
	if err != nil && !errors.Is(err, errors.ErrUnsupported) {
		return err
	}
	if err != nil || got != identity {
		return errors.New(i18n.Text("merge.changed"))
	}
	return nil
}

func (a *App) openSelectedMerge() {
	dir, change, ok := a.Changes.SelectedFile()
	if !ok || change.Kind != scm.Conflicted {
		return
	}
	if err := a.EditorGroup.OpenFile(filepath.Join(dir, change.Path)); err != nil {
		a.StatusError(err.Error())
		return
	}
	a.FocusEditorIfEnabled()
	a.OpenMergeEditor()
}
func (a *App) setMergeDefault() {
	a.ShowSelectDialog(i18n.Text("merge.setting"), mergeViewItems(), func(id string) { a.Settings.Editor.MergeConflictView = id; a.SaveAndApplySettings() }, nil)
}
func (a *App) registerMergeCommands() {
	a.Reg.Register(command.Command{ID: "changes.openMergeEditor", Title: i18n.Text("merge.openSelected"), Handler: a.openSelectedMerge, PaletteHidden: true})
	a.Reg.Register(command.Command{ID: "merge.openEditor", Title: i18n.Text("merge.open"), Handler: a.OpenMergeEditor, PaletteWhen: func() bool { return a.EditorGroup != nil && a.EditorGroup.HasMergeBanner() }})
	a.Reg.Register(command.Command{ID: "options.mergeConflictView", Title: i18n.Text("merge.setting"), Handler: a.setMergeDefault})
	for _, c := range []struct{ id, title string }{
		{"focusNext", i18n.Text("merge.focusNext")}, {"incoming", i18n.Text("merge.chooseIncoming")}, {"current", i18n.Text("merge.chooseCurrent")}, {"swap", i18n.Text("merge.swap")}, {"neither", i18n.Text("merge.neither")}, {"manual", i18n.Text("merge.manual")}, {"next", i18n.Text("merge.next")}, {"previous", i18n.Text("merge.previous")}, {"view", i18n.Text("merge.changeView")}, {"actions", i18n.Text("merge.actions")}, {"complete", i18n.Text("merge.complete")},
	} {
		a.Reg.Register(command.Command{ID: "merge." + c.id, Title: c.title, PaletteWhen: func() bool {
			return a.EditorGroup != nil && a.EditorGroup.MergeEditor() != nil && a.EditorGroup.MergeEditor().View != config.MergeText
		}, Handler: func() { a.mergeAction(c.id) }})
	}
}
