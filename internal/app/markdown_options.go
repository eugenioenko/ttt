package app

import (
	"maps"
	"strconv"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/widgets"
)

func markdownWordWrapItems() []widgets.SelectItem {
	return []widgets.SelectItem{
		{ID: "inherit", Label: i18n.Text("markdown.useEditorDefault")},
		{ID: "on", Label: i18n.Text("markdown.wrapOn")},
		{ID: "off", Label: i18n.Text("markdown.wrapOff")},
	}
}

func markdownWrapWidthItems() []widgets.SelectItem {
	return []widgets.SelectItem{
		{ID: "inherit", Label: i18n.Text("markdown.useEditorDefault")},
		{ID: "pane", Label: i18n.Text("markdown.paneWidth")},
		{ID: "custom", Label: i18n.Text("markdown.customWidth")},
	}
}

func markdownWordWrap(s *config.Settings) string {
	value := s.Editor.LanguageOverrides["markdown"].WordWrap
	if value == nil {
		return "inherit"
	}
	if *value {
		return "on"
	}
	return "off"
}

func markdownWrapWidth(s *config.Settings) string {
	value := s.Editor.LanguageOverrides["markdown"].WordWrapColumn
	if value == nil {
		return "inherit"
	}
	if *value == 0 {
		return "pane"
	}
	return strconv.Itoa(*value)
}

func setMarkdownOverride(s *config.Settings, override config.EditorLanguageOverride) {
	// Settings drafts share maps with the live config until a field is edited.
	overrides := maps.Clone(s.Editor.LanguageOverrides)
	if overrides == nil {
		overrides = make(map[string]config.EditorLanguageOverride)
	}
	if override == (config.EditorLanguageOverride{}) {
		delete(overrides, "markdown")
	} else {
		overrides["markdown"] = override
	}
	s.Editor.LanguageOverrides = overrides
}

func setMarkdownWordWrap(s *config.Settings, value string) {
	o := s.Editor.LanguageOverrides["markdown"]
	switch value {
	case "inherit":
		o.WordWrap = nil
	case "on", "off":
		o.WordWrap = boolPtr(value == "on")
	default:
		return
	}
	setMarkdownOverride(s, o)
}

func setMarkdownWrapWidth(s *config.Settings, value string) {
	o := s.Editor.LanguageOverrides["markdown"]
	switch value {
	case "inherit":
		o.WordWrapColumn = nil
	case "pane":
		column := 0
		o.WordWrapColumn = &column
	default:
		column, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || column < 1 {
			return
		}
		o.WordWrapColumn = &column
	}
	setMarkdownOverride(s, o)
}

func markdownCustomColumn(s *config.Settings) string {
	_, column := s.Editor.ResolveWordWrap("markdown")
	if column < 1 {
		column = 80
	}
	return strconv.Itoa(column)
}

func (a *App) ShowMarkdownWordWrapPicker() {
	if a.Root.HasOverlay() {
		return
	}
	a.showSelectDialog(i18n.Text("markdown.wordWrapTitle"), markdownWordWrapItems(), markdownWordWrap(a.Settings), func(id string) {
		setMarkdownWordWrap(a.Settings, id)
		a.SaveAndApplySettings()
	}, nil)
}

func (a *App) ShowMarkdownWrapWidthPicker() {
	if a.Root.HasOverlay() {
		return
	}
	selected := markdownWrapWidth(a.Settings)
	if selected != "inherit" && selected != "pane" {
		selected = "custom"
	}
	a.showSelectDialog(i18n.Text("markdown.wrapWidthTitle"), markdownWrapWidthItems(), selected, func(id string) {
		if id == "custom" {
			a.showMarkdownWrapColumnInput(markdownCustomColumn(a.Settings))
			return
		}
		setMarkdownWrapWidth(a.Settings, id)
		a.SaveAndApplySettings()
	}, nil)
}

func (a *App) showMarkdownWrapColumnInput(initial string) {
	a.ShowInputDialog(i18n.Text("markdown.customWidthTitle"), i18n.Text("markdown.columnHint"), initial, func(value string) {
		column, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || column < 1 {
			a.StatusWarn(i18n.Text("markdown.invalidColumn"))
			a.showMarkdownWrapColumnInput(value)
			return
		}
		setMarkdownWrapWidth(a.Settings, strconv.Itoa(column))
		a.SaveAndApplySettings()
	})
}

func (v *settingsView) markdownWrapWidthControl(category string, f settingField) widgets.Widget {
	input := widgets.NewInputWidget(widgets.InputConfig{Placeholder: i18n.Text("markdown.columnHint")})
	row := widgets.NewHStackWidget()
	row.Gap = 2
	mode := "inherit"
	var sel *widgets.SelectWidget
	showMode := func(id string) {
		mode = id
		sel.SetSelectedID(id)
		row.Children = []widgets.Widget{sel}
		if id == "custom" {
			row.Children = append(row.Children, input)
		}
		if v.adapter != nil {
			v.adapter.RebuildFocus()
		}
	}
	sel = widgets.NewSelectWidget(widgets.SelectConfig{
		Items: markdownWrapWidthItems(), Collapsible: true,
		OnOpen:   func() { v.closeSelectsExcept(sel) },
		OnSelect: showMode,
	})
	sel.FixedWidth = settingsControlCols
	v.selects = append(v.selects, sel)
	refresh := func() {
		value := f.GetString(&v.working)
		input.SetText(markdownCustomColumn(&v.working))
		if value != "inherit" && value != "pane" {
			value = "custom"
		}
		showMode(value)
	}
	refresh()
	v.refresh = append(v.refresh, refresh)
	v.inputs = append(v.inputs, func() string {
		value := mode
		if mode == "custom" {
			column, err := strconv.Atoi(strings.TrimSpace(input.Text()))
			if err != nil || column < 1 {
				return category + " → " + f.Label
			}
			value = strconv.Itoa(column)
		}
		f.SetString(&v.working, value)
		return ""
	})
	return row
}
