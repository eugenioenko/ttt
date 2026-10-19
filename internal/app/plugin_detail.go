package app

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"

	"github.com/gdamore/tcell/v3"
)

type pluginReadmeResult struct {
	name    string
	content string
	err     error
}

type pluginDetailState struct {
	markdown   *widgets.MarkdownWidget
	installBtn *widgets.ButtonWidget
}

func (a *App) OpenPluginDetail(entry plugin.RemoteRegistryEntry) {
	tabID := "plugin-detail:" + entry.Name

	md := widgets.NewMarkdownWidget()
	md.MaxWidth = a.Settings.Markdown.WrapWidth
	md.SetContent("Loading README...")

	installed := a.isPluginInstalled(entry.Name)

	installBtn := widgets.NewButtonWidget(widgets.ButtonConfig{
		Label: "Install",
		Style: term.StyleDefault,
		OnClick: func() {
			if a.PluginsPanel != nil && a.PluginsPanel.OnInstall != nil {
				a.PluginsPanel.OnInstall(entry.Repo, entry.Path, entry.Name)
				if state, ok := a.pluginDetailWidgets[entry.Name]; ok {
					state.installBtn.SetLabel("Installing...")
					state.installBtn.Disabled = true
				}
			}
		},
	})

	if installed {
		installBtn.SetLabel("Installed")
		installBtn.Disabled = true
	}

	a.pluginDetailWidgets[entry.Name] = &pluginDetailState{
		markdown:   md,
		installBtn: installBtn,
	}

	nameLabel := widgets.NewLabelWidget(widgets.LabelConfig{
		Text:  entry.Title(),
		Style: term.StyleHoverBold,
	})

	sourceBtn := widgets.NewButtonWidget(widgets.ButtonConfig{
		Label: "Source",
		Style: term.StyleDefault,
		OnClick: func() {
			OpenURL(repoFolderURL(entry.Repo, entry.Path))
		},
	})

	// Title row: three columns — the plugin name grows to fill, with the Source
	// and Install buttons as fixed columns on the right. The buttons report
	// their own width, so no wrapper is needed; letting each button's rect hug
	// its label also keeps the focus highlight tight (no trailing padding).
	headerRow := widgets.NewHStackWidget(nameLabel, sourceBtn, installBtn)
	headerRow.Gap = 1
	headerRow.FixedHeight = 1

	var metaParts []string
	if entry.Featured {
		metaParts = append(metaParts, "Featured")
	}
	if entry.Version != "" {
		metaParts = append(metaParts, "v"+entry.Version)
	}
	metaParts = append(metaParts, entry.Author)
	metaLabel := widgets.NewLabelWidget(widgets.LabelConfig{
		Text:  strings.Join(metaParts, " · "),
		Style: term.StyleMuted,
	})

	descLabel := widgets.NewParagraphWidget(entry.Description)
	descLabel.Style = term.StyleDefault

	headerWidgets := []widgets.Widget{headerRow, metaLabel, descLabel}

	if len(entry.Tags) > 0 {
		tagsLabel := widgets.NewLabelWidget(widgets.LabelConfig{
			Text:  strings.Join(entry.Tags, " · "),
			Style: term.StyleMuted,
		})
		headerWidgets = append(headerWidgets, tagsLabel)
	}

	if installed {
		if p := a.PluginManager.FindPlugin(entry.Name); p != nil && len(p.FileHandlers) > 0 {
			handlerLabels := make([]string, 0, len(p.FileHandlers))
			for _, handler := range p.FileHandlers {
				label := handler.Title
				if len(handler.Extensions) > 0 {
					extensions := make([]string, len(handler.Extensions))
					for i, ext := range handler.Extensions {
						extensions[i] = "." + ext
					}
					label = strings.Join(extensions, ", ") + " (" + handler.Title + ")"
				}
				handlerLabels = append(handlerLabels, label)
			}
			headerWidgets = append(headerWidgets, widgets.NewLabelWidget(widgets.LabelConfig{
				Text:  "Handles: " + strings.Join(handlerLabels, ", "),
				Style: term.StyleMuted,
			}))
		}
	}

	header := widgets.NewVStackWidget(headerWidgets...)
	header.Box.PaddingLeft = 1
	header.Box.PaddingRight = 1

	divider := widgets.NewDividerWidget(widgets.DividerConfig{})

	md.Box.PaddingLeft = 1
	md.Box.PaddingRight = 1
	scroll := widgets.NewScrollViewWidget(md)
	md.SetScrollParent(scroll)

	content := widgets.NewVStackWidget(header, divider, scroll)
	adapter := ui.NewWidgetAdapter(content)

	a.EditorGroup.OpenPluginTab(tabID, entry.Title(), adapter)

	go func() {
		readme, err := fetchPluginReadme(entry.Repo, entry.Path)
		a.Screen.PostEvent(tcell.NewEventInterrupt(&pluginReadmeResult{
			name:    entry.Name,
			content: readme,
			err:     err,
		}))
	}()
}

func (a *App) handlePluginReadmeResult(result *pluginReadmeResult) {
	state, ok := a.pluginDetailWidgets[result.name]
	if !ok {
		return
	}
	if result.err != nil {
		state.markdown.SetContent(fmt.Sprintf("Could not load README: %s", result.err))
	} else {
		state.markdown.SetContent(result.content)
	}
}

func (a *App) updatePluginDetailButtons() {
	for name, state := range a.pluginDetailWidgets {
		if a.isPluginInstalled(name) {
			state.installBtn.SetLabel("Installed")
			state.installBtn.Disabled = true
		}
	}
}

func (a *App) cleanupPluginDetailTab(id string) {
	const prefix = "plugin-detail:"
	if name, ok := strings.CutPrefix(id, prefix); ok {
		delete(a.pluginDetailWidgets, name)
	}
}

func (a *App) isPluginInstalled(name string) bool {
	for _, n := range a.PluginManager.InstalledPluginNames() {
		if n == name {
			return true
		}
	}
	return false
}

func fetchPluginReadme(repoURL, repoPath string) (string, error) {
	rawURL := repoToRawReadme(repoURL, repoPath)
	if rawURL == "" {
		return "", fmt.Errorf("unsupported repository URL")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", fmt.Errorf("fetch readme: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("readme returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read readme: %w", err)
	}

	return string(body), nil
}

// repoFolderURL returns the GitHub URL of the plugin's source folder — the repo
// root, or the plugin's subdirectory when the plugin lives in a monorepo.
func repoFolderURL(repoURL, repoPath string) string {
	repoURL = strings.TrimSuffix(repoURL, "/")
	repoURL = strings.TrimSuffix(repoURL, ".git")
	if repoPath != "" {
		return repoURL + "/tree/main/" + repoPath
	}
	return repoURL
}

func repoToRawReadme(repoURL, repoPath string) string {
	repoURL = strings.TrimSuffix(repoURL, "/")
	repoURL = strings.TrimSuffix(repoURL, ".git")

	if path, ok := strings.CutPrefix(repoURL, "https://github.com/"); ok {
		prefix := "https://raw.githubusercontent.com/" + path + "/main/"
		if repoPath != "" {
			return prefix + repoPath + "/README.md"
		}
		return prefix + "README.md"
	}

	return ""
}
