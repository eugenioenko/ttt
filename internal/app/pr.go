package app

import (
	"strings"

	"github.com/eugenioenko/ttt/internal/command"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/git"
	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"

	"github.com/gdamore/tcell/v3"
)

type PrFetchResult struct {
	URL   string
	Info  *github.PRInfo
	Diffs map[string]string
	Err   error
}

type DiffContentResult struct {
	Widget   *ui.DiffEditorWidget
	TabName  string
	OldLines []string
	NewLines []string
	Err      error
}

func (a *App) applyDiffContent(result *DiffContentResult) {
	for _, group := range a.editorGroups() {
		for i := range group.TabCount() {
			dv, ok := group.TabContent(i).(*ui.DiffEditorWidget)
			if !ok || dv != result.Widget {
				continue
			}
			if result.Err != nil {
				dv.FailLoading()
				a.StatusError(i18n.Format("status.failedToFetchFileContent", i18n.Args{"error": result.Err.Error()}))
			} else {
				dv.SetOldLines(result.OldLines)
				dv.SetNewLines(result.NewLines)
				dv.FinishLoading()
			}
			return
		}
	}
}

func (a *App) FetchAndOpenPR(url string) {
	owner, repo, number, err := github.ParsePRURL(url)
	if err != nil {
		a.StatusError(i18n.Format("status.invalidPRURL", i18n.Args{"error": err.Error()}))
		return
	}

	a.StatusNotify(i18n.Format("status.fetchingPR", i18n.Args{"number": number}))

	go func() {
		info, err := github.FetchPRInfo(owner, repo, number)
		if err != nil {
			_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&PrFetchResult{URL: url, Err: err}))
			return
		}

		diffText, err := github.FetchPRDiff(owner, repo, number)
		if err != nil {
			_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&PrFetchResult{URL: url, Err: err}))
			return
		}

		diffs := github.SplitMultiFileDiff(diffText)
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&PrFetchResult{URL: url, Info: info, Diffs: diffs}))
	}()
}

func prDetailTabID(url string) string {
	return "pr-detail:" + url
}

// OpenPRDetail shows every changed file in a PR as one unified scrollable
// view, the same shape as a commit's detail tab. PR diffs are already fully
// fetched, so unlike OpenCommitDetail this needs no async read.
func (a *App) OpenPRDetail(group *ui.ChangesGroup) {
	tabID := prDetailTabID(group.PRURL)
	if existing := a.EditorGroup.CommitDetailWidgetByTab(tabID); existing != nil {
		a.EditorGroup.SwitchToTabByPath(tabID)
		a.FocusEditorIfEnabled()
		return
	}
	files := make([]ui.CommitDetailFile, 0, len(group.Unstaged))
	for _, f := range group.Unstaged {
		file := ui.CommitDetailFile{Status: f.Status, Path: f.Path, OldPath: f.OldPath}
		if diffText, ok := group.PRDiffs[f.Path]; ok && diffText != "" {
			parsed := diff.Parse(diffText)
			if len(parsed.Hunks) == 0 {
				file.Error = i18n.Format("pr.emptyDiffFor", i18n.Args{"path": f.Path})
			} else {
				file.Diff = parsed
			}
		} else {
			file.Error = i18n.Format("pr.noDiffAvailableFor", i18n.Args{"path": f.Path})
		}
		files = append(files, file)
	}
	detail := ui.NewCommitDetailWidget(group.Dir, group.PRURL, "", a.EditorGroup.SyntaxHighlight)
	detail.Header = i18n.Text("pr.pullRequest")
	detail.Metadata = i18n.Format("pr.fileSChanged", i18n.Args{"groupName": group.Name, "filesCount": len(files)})
	detail.EmptyMessage = i18n.Text("pr.noPullRequestDescription")
	a.EditorGroup.ApplyDiffDefaults(detail)
	detail.SetDetail(group.PRBody, files, "")
	a.EditorGroup.OpenPluginTab(tabID, group.Name, detail)
	a.FocusEditorIfEnabled()
}

func (a *App) OpenPRDiff(group *ui.ChangesGroup, status git.FileStatus, extended bool) {
	diffText, ok := group.PRDiffs[status.Path]
	if !ok || diffText == "" {
		a.StatusWarn(i18n.Format("status.noDiffAvailableFor", i18n.Args{"path": status.Path}))
		return
	}
	parsed := diff.Parse(diffText)
	if len(parsed.Hunks) == 0 {
		a.StatusWarn(i18n.Format("status.emptyDiffFor", i18n.Args{"path": status.Path}))
		return
	}
	a.EditorGroup.OpenDiff(status.Path, parsed, nil, nil, false)
	if dv := a.EditorGroup.ActiveDiffWidget(); dv != nil {
		dv.SetExtendedFetcher(func(dv *ui.DiffEditorWidget) {
			a.fetchPRFileContent(dv, group.PROwner, group.PRRepo, group.PRBaseSHA, group.PRHeadSHA, status.Path)
		})
		if extended {
			dv.SetExtended(true)
		}
	}
	a.FocusEditorIfEnabled()
}

func (a *App) fetchPRFileContent(dv *ui.DiffEditorWidget, owner, repo, baseSHA, headSHA, path string) {
	if owner == "" || baseSHA == "" {
		dv.FailLoading()
		return
	}
	tabName := path + " (diff)"
	go func() {
		var oldLines, newLines []string
		var fetchErr error
		if content, err := github.FetchFileContent(owner, repo, path, baseSHA); err == nil {
			oldLines = strings.Split(content, "\n")
			if len(oldLines) > 0 && oldLines[len(oldLines)-1] == "" {
				oldLines = oldLines[:len(oldLines)-1]
			}
		} else {
			fetchErr = err
		}
		if content, err := github.FetchFileContent(owner, repo, path, headSHA); err == nil {
			newLines = strings.Split(content, "\n")
			if len(newLines) > 0 && newLines[len(newLines)-1] == "" {
				newLines = newLines[:len(newLines)-1]
			}
		} else if fetchErr == nil {
			fetchErr = err
		}
		_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&DiffContentResult{
			Widget:   dv,
			TabName:  tabName,
			OldLines: oldLines,
			NewLines: newLines,
			Err:      fetchErr,
		}))
	}()
}

func (a *App) ShowPRGroupMenu(group *ui.ChangesGroup, sx, sy int) {
	reg := a.Reg
	name := group.Name
	url := group.PRURL
	refreshID := "pr.refresh." + name
	closeID := "pr.close." + name
	reg.Register(command.Command{
		ID: refreshID, Title: i18n.Text("general.refresh"),
		Handler: func() {
			a.Changes.RemovePRGroup(name)
			a.FetchAndOpenPR(url)
		},
	})
	reg.Register(command.Command{
		ID: closeID, Title: i18n.Text("general.close"),
		Handler: func() {
			a.Changes.RemovePRGroup(name)
		},
	})
	items := []ui.ContextMenuItem{
		{Label: i18n.Text("general.refresh"), Command: refreshID},
		{Label: i18n.Text("general.close"), Command: closeID},
	}
	openContextMenu(a, items, sx, sy)
}
