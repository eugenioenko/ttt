package app

import (
	"github.com/eugenioenko/ttt/internal/core/diff"
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
		dv := group.DiffWidgetByTab(result.TabName)
		if dv == nil || dv != result.Widget {
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
			a.Screen.PostEvent(tcell.NewEventInterrupt(&PrFetchResult{URL: url, Err: err}))
			return
		}

		diffText, err := github.FetchPRDiff(owner, repo, number)
		if err != nil {
			a.Screen.PostEvent(tcell.NewEventInterrupt(&PrFetchResult{URL: url, Err: err}))
			return
		}

		diffs := github.SplitMultiFileDiff(diffText)
		a.Screen.PostEvent(tcell.NewEventInterrupt(&PrFetchResult{URL: url, Info: info, Diffs: diffs}))
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
