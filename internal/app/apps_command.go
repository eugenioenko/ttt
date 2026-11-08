package app

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"unicode/utf16"

	"github.com/eugenioenko/ttt/internal/config"
)

func appTerminalSpec(app config.Application, file, dir, workspace, goos string) TerminalSpec {
	spec := TerminalSpec{
		Shell: "/bin/sh", Label: app.Name, InEditor: app.Location != "panel", KeepOpen: app.KeepOpen,
		Cwd: strings.NewReplacer("{file}", file, "{dir}", dir, "{workspace}", workspace).Replace(app.Cwd),
		Env: []string{"TTT_APP_FILE=" + file, "TTT_APP_DIR=" + dir, "TTT_APP_WORKSPACE=" + workspace},
	}
	fileVar, dirVar, workspaceVar := `"${TTT_APP_FILE}"`, `"${TTT_APP_DIR}"`, `"${TTT_APP_WORKSPACE}"`
	if goos == "windows" {
		spec.Shell = "powershell.exe"
		fileVar, dirVar, workspaceVar = `"${env:TTT_APP_FILE}"`, `"${env:TTT_APP_DIR}"`, `"${env:TTT_APP_WORKSPACE}"`
	}
	// Expand environment references rather than path text: neither shell reparses
	// quotes, metacharacters or placeholder-looking text in a path as code.
	command := strings.NewReplacer("{file}", fileVar, "{dir}", dirVar, "{workspace}", workspaceVar).Replace(app.Command)
	if goos == "windows" {
		// EncodedCommand avoids Windows argv quoting changing the shell program.
		units := utf16.Encode([]rune(command))
		data := make([]byte, 2*len(units))
		for i, unit := range units {
			binary.LittleEndian.PutUint16(data[2*i:], unit)
		}
		spec.Args = []string{"-NoLogo", "-NoProfile", "-EncodedCommand", base64.StdEncoding.EncodeToString(data)}
	} else {
		spec.Args = []string{"-c", command}
	}
	return spec
}
