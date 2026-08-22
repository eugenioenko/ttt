package app

import (
	"strconv"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

const (
	settingsTabID       = "settings"
	settingsLabelCols   = 30
	settingsControlCols = 34
)

type settingKind int

const (
	settingBool settingKind = iota
	settingInt
	settingString
	settingEnum
)

type settingField struct {
	Label   string
	Kind    settingKind
	Restart bool
	Options func() []widgets.SelectItem
	// Min is the smallest accepted value for settingInt. Fields whose json tag
	// carries omitempty must set Min >= 1, since a stored 0 would be dropped on
	// save and silently revert to the default on the next load.
	Min int

	GetBool func(*config.Settings) bool
	SetBool func(*config.Settings, bool)

	GetString func(*config.Settings) string
	SetString func(*config.Settings, string)

	GetInt func(*config.Settings) int
	SetInt func(*config.Settings, int)
}

type settingsCategory struct {
	Title  string
	Fields []settingField
}

func boolPtr(b bool) *bool { return &b }

// LSP servers and the formatters map are deliberately absent: both are
// structured config that a form handles badly, and stay JSON-only.
func settingsCategories() []settingsCategory {
	return []settingsCategory{
		{Title: "Editor", Fields: []settingField{
			{Label: "Tab size", Kind: settingInt, Min: 1,
				GetInt: func(s *config.Settings) int { return s.Editor.TabSize },
				SetInt: func(s *config.Settings, v int) { s.Editor.TabSize = v }},
			{Label: "Insert spaces", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.InsertSpaces },
				SetBool: func(s *config.Settings, v bool) { s.Editor.InsertSpaces = v }},
			{Label: "Word wrap", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.WordWrap },
				SetBool: func(s *config.Settings, v bool) { s.Editor.WordWrap = v }},
			{Label: "Diff mode", Kind: settingEnum, Options: diffModeItems,
				GetString: func(s *config.Settings) string { return s.Editor.DiffMode },
				SetString: func(s *config.Settings, v string) { s.Editor.DiffMode = v }},
			{Label: "Diff word wrap", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.DiffWordWrap },
				SetBool: func(s *config.Settings, v bool) { s.Editor.DiffWordWrap = v }},
			{Label: "Line numbers", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.LineNumbers },
				SetBool: func(s *config.Settings, v bool) { s.Editor.LineNumbers = v }},
			{Label: "Auto indent", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.IsAutoIndentEnabled() },
				SetBool: func(s *config.Settings, v bool) { s.Editor.AutoIndent = boolPtr(v) }},
			{Label: "Auto dedent", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.IsAutoDedentEnabled() },
				SetBool: func(s *config.Settings, v bool) { s.Editor.AutoDedent = boolPtr(v) }},
			{Label: "Insert final newline", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.InsertFinalNewline },
				SetBool: func(s *config.Settings, v bool) { s.Editor.InsertFinalNewline = v }},
			{Label: "Show trailing newline", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.IsShowTrailingNewlineEnabled() },
				SetBool: func(s *config.Settings, v bool) { s.Editor.ShowTrailingNewline = boolPtr(v) }},
			{Label: "Trim trailing whitespace", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.TrimTrailingWhitespace },
				SetBool: func(s *config.Settings, v bool) { s.Editor.TrimTrailingWhitespace = v }},
			{Label: "Format on save", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.FormatOnSave },
				SetBool: func(s *config.Settings, v bool) { s.Editor.FormatOnSave = v }},
			{Label: "Focus editor on open", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.FocusOnOpen },
				SetBool: func(s *config.Settings, v bool) { s.Editor.FocusOnOpen = v }},
		}},
		{Title: "Appearance", Fields: []settingField{
			{Label: "Theme", Kind: settingEnum, Options: themeItems,
				GetString: func(s *config.Settings) string { return s.Theme },
				SetString: func(s *config.Settings, v string) { s.Theme = v }},
			{Label: "Diff context", Kind: settingEnum, Options: diffContextItems,
				GetString: func(s *config.Settings) string { return s.Editor.DiffContext },
				SetString: func(s *config.Settings, v string) { s.Editor.DiffContext = v }},
			{Label: "High contrast diffs", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.DiffHighContrast },
				SetBool: func(s *config.Settings, v bool) { s.Editor.DiffHighContrast = v }},
			{Label: "Border style", Kind: settingEnum, Options: borderStyleItems,
				GetString: func(s *config.Settings) string { return s.Editor.BorderStyle },
				SetString: func(s *config.Settings, v string) { s.Editor.BorderStyle = v }},
			{Label: "Gutter style", Kind: settingEnum, Options: gutterStyleItems,
				GetString: func(s *config.Settings) string { return s.Editor.GutterStyle },
				SetString: func(s *config.Settings, v string) { s.Editor.GutterStyle = v }},
			{Label: "Cursor style", Kind: settingEnum, Options: cursorStyleItems,
				GetString: func(s *config.Settings) string { return s.Editor.CursorStyle },
				SetString: func(s *config.Settings, v string) { s.Editor.CursorStyle = v }},
			{Label: "Syntax highlight", Kind: settingBool, Restart: true,
				GetBool: func(s *config.Settings) bool { return s.Editor.IsSyntaxHighlightEnabled() },
				SetBool: func(s *config.Settings, v bool) { s.Editor.SyntaxHighlight = boolPtr(v) }},
			{Label: "Bracket pair colors", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.BracketPairColorization },
				SetBool: func(s *config.Settings, v bool) { s.Editor.BracketPairColorization = v }},
			{Label: "Menu bar", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.IsMenuBarVisible() },
				SetBool: func(s *config.Settings, v bool) { s.Editor.MenuBar = boolPtr(v) }},
			{Label: "Git gutter", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Editor.IsGitGutterEnabled() },
				SetBool: func(s *config.Settings, v bool) { s.Editor.GitGutter = boolPtr(v) }},
			{Label: "Markdown wrap width", Kind: settingInt, Min: 1,
				GetInt: func(s *config.Settings) int { return s.Markdown.WrapWidth },
				SetInt: func(s *config.Settings, v int) { s.Markdown.WrapWidth = v }},
		}},
		{Title: "Completion", Fields: []settingField{
			{Label: "Enable completion", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Autocomplete.Enabled },
				SetBool: func(s *config.Settings, v bool) { s.Autocomplete.Enabled = v }},
			{Label: "Suggest as you type", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Autocomplete.AutoSuggest },
				SetBool: func(s *config.Settings, v bool) { s.Autocomplete.AutoSuggest = v }},
			{Label: "Signature help", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Autocomplete.SignatureHelp },
				SetBool: func(s *config.Settings, v bool) { s.Autocomplete.SignatureHelp = v }},
			{Label: "Debounce (ms)", Kind: settingInt,
				GetInt: func(s *config.Settings) int { return s.Autocomplete.Debounce },
				SetInt: func(s *config.Settings, v int) { s.Autocomplete.Debounce = v }},
		}},
		{Title: "Advanced", Fields: []settingField{
			{Label: "Git: file view", Kind: settingEnum, Options: gitFileViewItems,
				GetString: func(s *config.Settings) string { return s.Git.FileView },
				SetString: func(s *config.Settings, v string) { s.Git.FileView = v }},
			{Label: "Explorer: hidden files", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Explorer.ShowHidden },
				SetBool: func(s *config.Settings, v bool) { s.Explorer.ShowHidden = v }},
			{Label: "Explorer: git-ignored files", Kind: settingBool,
				GetBool: func(s *config.Settings) bool { return s.Explorer.ShowGitIgnored },
				SetBool: func(s *config.Settings, v bool) { s.Explorer.ShowGitIgnored = v }},
			{Label: "Terminal shell", Kind: settingString, Restart: true,
				GetString: func(s *config.Settings) string { return s.Terminal.Shell },
				SetString: func(s *config.Settings, v string) { s.Terminal.Shell = v }},
			{Label: "Terminal scrollback", Kind: settingInt, Restart: true, Min: 1,
				GetInt: func(s *config.Settings) int { return s.Terminal.Scrollback },
				SetInt: func(s *config.Settings, v int) { s.Terminal.Scrollback = v }},
			{Label: "Search debounce (ms)", Kind: settingInt,
				GetInt: func(s *config.Settings) int { return s.Search.Debounce },
				SetInt: func(s *config.Settings, v int) { s.Search.Debounce = v }},
			{Label: "Enable plugins", Kind: settingBool, Restart: true,
				GetBool: func(s *config.Settings) bool { return s.Plugins.IsEnabled() },
				SetBool: func(s *config.Settings, v bool) { s.Plugins.Enabled = boolPtr(v) }},
			{Label: "Debug mode", Kind: settingBool, Restart: true,
				GetBool: func(s *config.Settings) bool { return s.DebugMode },
				SetBool: func(s *config.Settings, v bool) { s.DebugMode = v }},
		}},
	}
}

func gitFileViewItems() []widgets.SelectItem {
	return []widgets.SelectItem{
		{ID: config.GitFileViewTree, Label: "Tree"},
		{ID: config.GitFileViewList, Label: "List"},
	}
}

func themeItems() []widgets.SelectItem {
	names := config.ListThemes()
	items := make([]widgets.SelectItem, 0, len(names)+1)
	items = append(items, widgets.SelectItem{ID: "", Label: "Default"})
	for _, n := range names {
		items = append(items, widgets.SelectItem{ID: n, Label: n})
	}
	return items
}

// Mirrors term.ParseCursorStyle. "" means unset and behaves as a blinking bar,
// so it is offered as "Default" rather than duplicated as "Bar".
func cursorStyleItems() []widgets.SelectItem {
	return []widgets.SelectItem{
		{ID: "", Label: "Default"},
		{ID: "bar", Label: "Bar (blinking)"},
		{ID: "steadyBar", Label: "Bar (steady)"},
		{ID: "block", Label: "Block (blinking)"},
		{ID: "steadyBlock", Label: "Block (steady)"},
		{ID: "underline", Label: "Underline (blinking)"},
		{ID: "steadyUnderline", Label: "Underline (steady)"},
	}
}

type settingsView struct {
	app        *App
	working    config.Settings
	categories []settingsCategory
	adapter    *ui.WidgetAdapter
	status     *widgets.LabelWidget
	inputs     []func() string
	selects    []*widgets.SelectWidget
}

// commitTo copies the fields this form owns out of the working copy and onto s,
// leaving everything else on s untouched. Assigning the whole working struct
// would also write back its snapshot of settings the form never shows, undoing
// anything changed elsewhere — a theme picked from the palette, an Options
// toggle — while the tab sat open.
func (v *settingsView) commitTo(s *config.Settings) {
	for _, cat := range v.categories {
		for _, f := range cat.Fields {
			switch f.Kind {
			case settingBool:
				f.SetBool(s, f.GetBool(&v.working))
			case settingInt:
				f.SetInt(s, f.GetInt(&v.working))
			default:
				f.SetString(s, f.GetString(&v.working))
			}
		}
	}
}

func (v *settingsView) closeSelectsExcept(keep *widgets.SelectWidget) {
	for _, s := range v.selects {
		if s != keep {
			s.ClosePopup()
		}
	}
}

func (a *App) ShowSettings() {
	// Reopening while the tab is already open must not discard pending edits.
	if v := a.settingsView; v != nil {
		if !a.EditorGroup.SwitchToTabByPath(settingsTabID) {
			a.EditorGroup.OpenPluginTab(settingsTabID, "Settings", v.adapter)
		}
		a.FocusEditor()
		v.adapter.SetFocused(true)
		return
	}

	v := &settingsView{app: a, working: *a.Settings, categories: settingsCategories()}
	a.settingsView = v

	tabItems := make([]widgets.TabItem, 0, len(v.categories))
	panes := make([]widgets.Widget, 0, len(v.categories))
	for _, cat := range v.categories {
		tabItems = append(tabItems, widgets.TabItem{ID: cat.Title, Label: cat.Title})
		panes = append(panes, v.buildPane(cat))
	}
	tabs := widgets.NewTabsWidget(widgets.TabsConfig{Items: tabItems, Align: "left"})
	tabbed := widgets.NewTabbedWidget(tabs, panes)
	tabbed.Fill = true

	v.status = widgets.NewLabelWidget(widgets.LabelConfig{Style: term.StyleMuted})
	cancelBtn := widgets.NewButtonWidget(widgets.ButtonConfig{Label: "Cancel", OnClick: v.cancel})
	applyBtn := widgets.NewButtonWidget(widgets.ButtonConfig{Label: "Apply", OnClick: v.apply})
	buttons := widgets.NewHStackWidget(v.status, cancelBtn, applyBtn)
	buttons.Gap = 1
	buttons.FixedHeight = 1
	buttons.Box.PaddingLeft = 1
	buttons.Box.PaddingRight = 1

	root := widgets.NewVStackWidget(
		tabbed,
		widgets.NewDividerWidget(widgets.DividerConfig{}),
		buttons,
	)

	// NewWidgetAdapter wires TabbedWidget.OnChange to rebuild focus on tab change.
	v.adapter = ui.NewWidgetAdapter(root)
	v.adapter.EnableScrollIntoView()

	a.EditorGroup.OpenPluginTab(settingsTabID, "Settings", v.adapter)
	a.FocusEditor()
	v.adapter.SetFocused(true)
}

func (v *settingsView) buildPane(cat settingsCategory) widgets.Widget {
	rows := make([]widgets.Widget, 0, len(cat.Fields))
	for _, f := range cat.Fields {
		rows = append(rows, v.buildRow(cat.Title, f))
	}
	stack := widgets.NewVStackWidget(rows...)
	stack.MeasureGrow = true
	stack.Box.PaddingLeft = 1
	stack.Box.PaddingTop = 1

	// The divider sits inside the pane so it reads as the tab strip's bottom
	// border, and stays put while the fields scroll under it.
	return widgets.NewVStackWidget(
		widgets.NewDividerWidget(widgets.DividerConfig{}),
		widgets.NewScrollViewWidget(stack),
	)
}

// One row per setting: label in a fixed left column, control on the right.
// Each control keeps its native shape, so its type is readable at a glance.
func (v *settingsView) buildRow(category string, f settingField) widgets.Widget {
	label := f.Label
	if f.Restart {
		label += " (restart)"
	}
	name := widgets.NewLabelWidget(widgets.LabelConfig{Text: label})
	name.FixedWidth = settingsLabelCols

	var control widgets.Widget
	switch f.Kind {
	case settingBool:
		control = v.boolControl(f)
	case settingEnum:
		control = v.enumControl(f)
	default:
		control = v.textControl(category, f)
	}

	row := widgets.NewHStackWidget(name, control)
	row.Gap = 2
	row.FixedHeight = 1
	return row
}

func (v *settingsView) boolControl(f settingField) widgets.Widget {
	return widgets.NewCheckboxWidget(widgets.CheckboxConfig{
		Checked:  f.GetBool(&v.working),
		OnChange: func(checked bool) { f.SetBool(&v.working, checked) },
	})
}

func (v *settingsView) enumControl(f settingField) widgets.Widget {
	var sel *widgets.SelectWidget
	sel = widgets.NewSelectWidget(widgets.SelectConfig{
		Items:       f.Options(),
		Collapsible: true,
		OnOpen:      func() { v.closeSelectsExcept(sel) },
		OnSelect: func(id string) {
			f.SetString(&v.working, id)
			sel.SetSelectedID(id)
		},
	})
	v.selects = append(v.selects, sel)
	sel.FixedWidth = settingsControlCols
	sel.SetSelectedID(f.GetString(&v.working))
	return sel
}

// Text and numeric fields are parsed on Apply rather than per keystroke, so a
// half-typed value never reaches the working copy.
func (v *settingsView) textControl(category string, f settingField) widgets.Widget {
	current := ""
	if f.Kind == settingInt {
		current = strconv.Itoa(f.GetInt(&v.working))
	} else {
		current = f.GetString(&v.working)
	}

	// Borderless: InputWidget draws a "❯" prefix and recolours it on focus, which
	// is affordance enough for a one-line field.
	inp := widgets.NewInputWidget(widgets.InputConfig{})
	inp.SetText(current)

	// Returns a description of the offending field, or "" when the value is good.
	v.inputs = append(v.inputs, func() string {
		text := inp.Text()
		if f.Kind == settingString {
			f.SetString(&v.working, text)
			return ""
		}
		n, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || n < f.Min {
			inp.SetText(strconv.Itoa(f.GetInt(&v.working)))
			return category + " → " + f.Label
		}
		f.SetInt(&v.working, n)
		return ""
	})
	return inp
}

func (v *settingsView) apply() {
	// Validate every field before committing, so one bad value does not hide the
	// rest and the user fixes them in a single pass.
	var bad []string
	for _, commit := range v.inputs {
		if msg := commit(); msg != "" {
			bad = append(bad, msg)
		}
	}
	if len(bad) > 0 {
		v.setStatus("Invalid value for " + strings.Join(bad, ", "))
		return
	}
	v.commitTo(v.app.Settings)
	v.app.SaveAndApplySettings()
	v.working = *v.app.Settings
	v.setStatus("Settings applied")
}

// Dropping the working copy is what discards unapplied edits, so it does not
// wait on OnContentTabClose: that hook only exists once App.Init has run, and
// clearing it here is idempotent with it.
func (v *settingsView) cancel() {
	v.app.settingsView = nil
	v.app.EditorGroup.ClosePluginTab(settingsTabID)
}

func (v *settingsView) setStatus(msg string) {
	if v.status != nil {
		v.status.Config.Text = msg
	}
}

func (a *App) ApplySettingsView() {
	if a.settingsView == nil {
		a.StatusNotify("No settings editor open")
		return
	}
	a.settingsView.apply()
}

func (a *App) CancelSettingsView() {
	if a.settingsView == nil {
		a.StatusNotify("No settings editor open")
		return
	}
	a.settingsView.cancel()
}

func (a *App) cleanupSettingsTab(id string) {
	if id == settingsTabID {
		a.settingsView = nil
	}
}
