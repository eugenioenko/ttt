package app

import (
	"fmt"
	"github.com/eugenioenko/ttt/internal/core/diff"
	"github.com/eugenioenko/ttt/internal/github"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/ui"
	"sort"
	"strconv"
	"strings"
)

func (a *App) wirePluginChanges(p *plugin.Plugin) {
	p.OpenChanges = func(opts plugin.ChangesOptions) (string, error) {
		parts := github.SplitMultiFileDiff(opts.Diff)
		if len(parts) == 0 {
			return "", fmt.Errorf("no files found in diff")
		}
		paths := make([]string, 0, len(parts))
		for path := range parts {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		files := make([]ui.CommitDetailFile, 0, len(paths))
		for _, path := range paths {
			text := parts[path]
			status := "M"
			if strings.Contains(text, "\nnew file mode ") {
				status = "A"
			} else if strings.Contains(text, "\ndeleted file mode ") {
				status = "D"
			} else if strings.Contains(text, "\nrename from ") {
				status = "R"
			}
			oldPath := ""
			if status == "R" {
				for _, line := range strings.Split(text, "\n") {
					if strings.HasPrefix(line, "rename from ") {
						oldPath = strings.TrimPrefix(line, "rename from ")
						if strings.HasPrefix(oldPath, "\"") {
							if decoded, err := strconv.Unquote(oldPath); err == nil {
								oldPath = decoded
							}
						}
						break
					}
				}
			}
			file := ui.CommitDetailFile{Path: path, OldPath: oldPath, Status: status, Diff: diff.Parse(text)}
			if len(file.Diff.Hunks) == 0 {
				file.Error = "No textual changes"
			}
			files = append(files, file)
		}
		id := "plugin:" + strconv.Quote(p.Name) + ":changes:" + strconv.Quote(opts.ID)
		detail := a.EditorGroup.CommitDetailWidgetByTab(id)
		if detail == nil {
			detail = ui.NewCommitDetailWidget("", "", "", a.EditorGroup.SyntaxHighlight)
			a.EditorGroup.ApplyDiffDefaults(detail)
		}
		detail.Header = opts.Title
		detail.SetDetail(opts.Description, files, "")
		a.EditorGroup.OpenPluginTab(id, opts.Title, detail)
		if !a.Root.HasModalOverlay() {
			a.FocusEditor()
		}
		return id, nil
	}
}
