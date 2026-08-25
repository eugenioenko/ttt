package app

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/github"
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
	TabName  string
	OldLines []string
	NewLines []string
	Err      error
}

func (a *App) FetchAndOpenPR(url string) {
	owner, repo, number, err := github.ParsePRURL(url)
	if err != nil {
		a.StatusError("Invalid PR URL: " + err.Error())
		return
	}

	a.StatusNotify(fmt.Sprintf("Fetching PR #%d...", number))

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
			file.Diff = diff.Parse(diffText)
		} else {
			file.Error = "No diff available for " + f.Path
		}
		files = append(files, file)
	}
	detail := ui.NewCommitDetailWidget(group.Dir, group.PRURL, "", a.EditorGroup.SyntaxHighlight)
	detail.Header = "Pull request"
	a.EditorGroup.ApplyDiffDefaults(detail)
	detail.SetDetail(fmt.Sprintf("%s\n%d file(s) changed", group.Name, len(files)), files, "")
	a.EditorGroup.OpenPluginTab(tabID, group.Name, detail)
	a.FocusEditorIfEnabled()
}
