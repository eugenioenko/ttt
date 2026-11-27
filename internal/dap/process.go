package dap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/eugenioenko/ttt/internal/i18n"
)

func (a Adapter) Process(folder string) (*exec.Cmd, error) {
	if len(a.Command) == 0 || a.Command[0] == "" {
		return nil, errors.New(i18n.Text("debug.emptyCommand"))
	}
	expand := strings.NewReplacer("${workspaceFolder}", folder).Replace
	argv := make([]string, len(a.Command))
	for i, s := range a.Command {
		argv[i] = expand(s)
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = expand(a.Cwd)
	if !filepath.IsAbs(cmd.Dir) {
		cmd.Dir = filepath.Join(folder, cmd.Dir)
	}
	cmd.Env = cmd.Environ()
	for k, v := range a.Env {
		cmd.Env = append(cmd.Env, k+"="+expand(v))
	}
	return cmd, nil
}

func (a Adapter) Open(ctx context.Context, folder string, onEvent func(Event), run RunInTerminal) (*Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	switch a.Transport {
	case "tcp":
		var dialer net.Dialer
		conn, err := dialer.DialContext(ctx, "tcp", a.Address)
		if err != nil {
			return nil, err
		}
		// A network peer must not be able to start a local terminal process.
		return NewSession(conn, onEvent), nil
	case "", "stdio":
		cmd, err := a.Process(folder)
		if err != nil {
			return nil, err
		}
		return StartStdio(cmd, onEvent, run)
	default:
		return nil, fmt.Errorf("unknown adapter transport %q", a.Transport)
	}
}
