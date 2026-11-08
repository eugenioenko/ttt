package app

import (
	"strings"

	"github.com/eugenioenko/ttt/internal/config"
)

func appPaddingX(app config.Application) int {
	if app.PaddingX != nil {
		return max(*app.PaddingX, 0)
	}
	return 1
}

func appTerminalSpec(app config.Application, file, dir, workspace string) TerminalSpec {
	plain := strings.NewReplacer("{file}", file, "{dir}", dir, "{workspace}", workspace)
	padding := appPaddingX(app)
	spec := TerminalSpec{
		Shell: plain.Replace(app.Command), Label: app.Name, InEditor: app.Location != "panel", KeepOpen: app.KeepOpen,
		Cwd: plain.Replace(app.Cwd), PaddingX: &padding,
		Args: make([]string, 0, len(app.Args)),
	}
	for _, arg := range app.Args {
		if file == "" && strings.Contains(arg, "{file}") {
			continue
		}
		spec.Args = append(spec.Args, plain.Replace(arg))
	}
	return spec
}
