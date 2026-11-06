package app

import "github.com/eugenioenko/ttt/internal/widgets"

func seedInputText(input *widgets.InputWidget, text string) {
	onChange := input.Config.OnChange
	input.Config.OnChange = nil
	input.SetText(text)
	input.Config.OnChange = onChange
}
