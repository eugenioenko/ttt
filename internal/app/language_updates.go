package app

import (
	"context"
	"net/http"
	"reflect"
	"slices"
	"time"

	"github.com/eugenioenko/ttt/internal/asynctask"
	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/plugin"
)

type languageState struct {
	locale, requested string
	updates           *asynctask.Task[[]languageUpdateResult]
}

type languageUpdateResult struct {
	source []config.LanguagePackSetting
	update plugin.LanguageUpdate
	err    error
}

func (a *App) CheckLanguagePackUpdates() {
	if !a.Settings.Language.IsAutoUpdateEnabled() {
		return
	}
	groups := make(map[string][]config.LanguagePackSetting)
	for _, pack := range a.Settings.Language.Packs {
		if pack.Plugin != "" && pack.Repo != "" {
			groups[pack.Plugin] = append(groups[pack.Plugin], pack)
		}
	}
	var sources [][]config.LanguagePackSetting
	for _, packs := range groups {
		if slices.ContainsFunc(packs, func(pack config.LanguagePackSetting) bool {
			return plugin.LanguageNeedsUpdate(a.Version, pack.TTTVersion)
		}) {
			sources = append(sources, packs)
		}
	}
	if len(sources) == 0 {
		return
	}
	if a.language.updates == nil {
		a.language.updates = NewTask[[]languageUpdateResult](a, asynctask.TaskOptions{})
	}
	version := a.Version
	a.language.updates.Schedule(func(ctx context.Context) ([]languageUpdateResult, error) {
		client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
		var results []languageUpdateResult
		for _, packs := range sources {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			source := plugin.LanguageUpdateSource{Name: packs[0].Plugin, Repo: packs[0].Repo, RepoPath: packs[0].RepoPath}
			for _, pack := range packs {
				source.Languages = append(source.Languages, plugin.Language{Locale: pack.Locale, TTTVersion: pack.TTTVersion})
			}
			update, err := plugin.FetchLanguageUpdate(ctx, client, source, version)
			results = append(results, languageUpdateResult{source: packs, update: update, err: err})
		}
		return results, nil
	}, func(results []languageUpdateResult, err error) {
		if err != nil || !a.Settings.Language.IsAutoUpdateEnabled() {
			return
		}
		for _, result := range results {
			a.applyLanguageUpdate(result)
		}
	})
}

func (a *App) applyLanguageUpdate(result languageUpdateResult) {
	name := result.source[0].Plugin
	if result.err != nil {
		a.LogOutput("warn", "languages", result.err.Error())
		return
	}
	if len(result.update.Languages) == 0 {
		return
	}
	current := slices.DeleteFunc(slices.Clone(a.Settings.Language.Packs), func(pack config.LanguagePackSetting) bool { return pack.Plugin != name })
	if !reflect.DeepEqual(current, result.source) {
		return
	}
	if err := replacePluginContributionFiles("languages", name, result.update.Files); err != nil {
		a.LogOutput("error", "languages", err.Error())
		return
	}
	activeUpdated := false
	for _, update := range result.update.Languages {
		for i := range a.Settings.Language.Packs {
			pack := &a.Settings.Language.Packs[i]
			if pack.Plugin == name && pack.Locale == update.Locale {
				pack.TTTVersion = update.TTTVersion
				pack.Name = update.Name
				pack.SourceHashes = ""
				if update.SourceHashes != "" {
					pack.SourceHashes = languageCachePath(name, pack.Locale+".source")
				}
				activeUpdated = activeUpdated || a.language.locale == pack.Locale
			}
		}
	}
	if err := config.SaveSettings(*a.Settings); err != nil {
		a.LogOutput("error", "languages", err.Error())
	}
	if activeUpdated {
		a.StatusNotify(i18n.Text("status.languagePackUpdatedRestart"))
	}
}
