package app

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
	"github.com/eugenioenko/ttt/internal/widgets"
)

//go:embed contributors.json
var contributorsJSON []byte

const (
	aboutCreditsMaxHeight = 5
	aboutCreditsMinHeight = 3
	// Screen rows needed besides the credits list: the dialog's top offset, borders,
	// title, info rows, divider, heading, "Thank you!", Close button and gaps, plus the status bar.
	aboutDialogChromeRows = 20
)

type aboutCredits struct {
	Contributors []string `json:"contributors"`
	Reporters    []string `json:"reporters"`
}

func creditNames() []string {
	var c aboutCredits
	if err := json.Unmarshal(contributorsJSON, &c); err != nil {
		return nil
	}
	return append(c.Contributors, c.Reporters...)
}

func (a *App) ShowAbout() {
	if a.Root.HasOverlay() {
		return
	}
	info := widgets.NewKeyValueListWidget([]widgets.KeyValueEntry{
		{Key: "Version", Value: a.Version},
		{Key: "Website", Value: "https://tttedit.dev"},
		{Key: "GitHub", Value: "https://github.com/eugenioenko/ttt"},
	})
	info.InvertStyles = true

	divider := widgets.NewDividerWidget(widgets.DividerConfig{})
	divider.Box.MarginTop = 1

	heading := widgets.NewParagraphWidget("ttt is great because of everyone who contributes code, reports issues, and uses it")
	heading.Box.MarginTop = 1
	heading.Box.MarginBottom = 1

	// wrapText only breaks at ' ', so the no-break space keeps each "·" on its name's line.
	names := widgets.NewParagraphWidget(strings.Join(creditNames(), " · "))
	names.Style = term.StyleMuted
	list := widgets.NewVStackWidget(names)
	list.MeasureGrow = true
	credits := widgets.NewBoxWidget(widgets.BoxModel{})
	credits.Child = widgets.NewScrollViewWidget(list)
	credits.FixedHeight = aboutCreditsMaxHeight
	if a.Screen != nil {
		_, sh := a.Screen.Size()
		credits.FixedHeight = max(aboutCreditsMinHeight, min(aboutCreditsMaxHeight, sh-aboutDialogChromeRows))
	}

	thanks := widgets.NewParagraphWidget("Thank you!")
	thanks.Box.MarginTop = 1

	dialog := widgets.NewDialogWidget(60)
	dialog.Title = "About TTT Editor"
	dialog.Borders = *a.Borders
	dialog.SetContent(widgets.NewVStackWidget(info, divider, heading, credits, thanks))
	dialog.Buttons = []widgets.DialogButton{
		{Label: "&Close", Handler: func() { a.DismissDialog() }},
	}
	dialog.OnDismiss = func() { a.DismissDialog() }
	dialog.Build()
	a.ShowDialog(ui.NewWidgetAdapter(dialog))
}
