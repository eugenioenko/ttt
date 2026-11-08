package app

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/i18n"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

var indentSizes = []int{1, 2, 3, 4, 6, 8}

func (a *App) showIndentDialog(title string, onApply func(useTabs bool, tabSize int)) {
	useTabs := false
	tabSize := 4
	if a.EditorGroup.Editor != nil {
		useTabs = a.EditorGroup.Editor.UseTabs
		tabSize = a.EditorGroup.Editor.TabSize
	}
	if tabSize <= 0 {
		tabSize = a.Settings.Editor.TabSize
	}

	styleTab := widgets.NewTabsWidget(widgets.TabsConfig{
		Align: "center",
		Items: []widgets.TabItem{
			{ID: "spaces", Label: i18n.Text("options.spaces")},
			{ID: "tabs", Label: i18n.Text("options.tabs")},
		},
	})
	if useTabs {
		styleTab.SetActive("tabs")
	} else {
		styleTab.SetActive("spaces")
	}
	styleTab.Config.OnTabClick = func(index int) {
		if index == 0 {
			styleTab.SetActive("spaces")
		} else {
			styleTab.SetActive("tabs")
		}
	}

	sizeItems := make([]widgets.TabItem, len(indentSizes))
	for i, s := range indentSizes {
		sizeItems[i] = widgets.TabItem{
			ID:    fmt.Sprintf("%d", s),
			Label: fmt.Sprintf("%d", s),
		}
	}
	sizeTab := widgets.NewTabsWidget(widgets.TabsConfig{
		Align: "center",
		Items: sizeItems,
	})
	sizeTab.SetActive(fmt.Sprintf("%d", tabSize))
	sizeTab.Config.OnTabClick = func(index int) {
		sizeTab.SetActive(sizeItems[index].ID)
	}

	content := widgets.NewVStackWidget(styleTab, sizeTab)
	content.Gap = 1

	dialog := widgets.NewDialogWidget(40)
	dialog.Title = title
	dialog.Borders = *a.Borders
	dialog.SetContent(content)
	dialog.Buttons = []widgets.DialogButton{
		{Label: i18n.Text("general.cancelButton"), Handler: func() {
			a.DismissDialog()
		}},
		{Label: i18n.Text("options.aUto"), Handler: func() {
			if a.EditorGroup.Editor != nil && a.EditorGroup.Editor.Buf != nil {
				info := buffer.DetectIndent(a.EditorGroup.Editor.Buf.Lines)
				ut := info.UseTabs
				ts := tabSize
				if info.Size > 0 {
					ts = info.Size
				}
				a.DismissDialog()
				onApply(ut, ts)
			}
		}},
		{Label: i18n.Text("general.applyButton"), Handler: func() {
			ut := styleTab.ActiveID() == "tabs"
			ts := tabSize
			for _, s := range indentSizes {
				if fmt.Sprintf("%d", s) == sizeTab.ActiveID() {
					ts = s
					break
				}
			}
			a.DismissDialog()
			onApply(ut, ts)
		}},
	}
	dialog.OnDismiss = func() { a.DismissDialog() }
	dialog.Build()

	adapter := ui.NewWidgetAdapter(dialog)
	a.ShowDialog(adapter)
}
