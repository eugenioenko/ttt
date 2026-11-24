package dap

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Adapter struct {
	Command        []string          `json:"command,omitempty"`
	Transport      string            `json:"transport,omitempty"`
	Address        string            `json:"address,omitempty"`
	AddressPattern string            `json:"addressPattern,omitempty"`
	Languages      []string          `json:"languages,omitempty"`
	Cwd            string            `json:"cwd,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Provider       string            `json:"provider,omitempty"`
}

func (a Adapter) Process(folder string, vars ...string) (*exec.Cmd, error) {
	if len(a.Command) == 0 || a.Command[0] == "" {
		return nil, fmt.Errorf("adapter command is empty")
	}
	expand := strings.NewReplacer(append([]string{"${workspaceFolder}", folder}, vars...)...).Replace
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

func (a Adapter) Open(ctx context.Context, t Target, args map[string]any, event func(Event), options SessionOptions) (*Session, error) {
	var address string
	var vars []string
	if python, ok := args["python"].(string); ok {
		vars = append(vars, "${python}", python)
	}
	switch a.Transport {
	case "tcp":
		return Connect(ctx, a.Address, event, options)
	case "server":
		if !strings.Contains(strings.Join(a.Command, "\x00"), "${port}") {
			return nil, fmt.Errorf("server command requires ${port}")
		}
		l, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		address = l.Addr().String()
		vars = append(vars, "${port}", strconv.Itoa(l.Addr().(*net.TCPAddr).Port))
		if err = l.Close(); err != nil {
			return nil, err
		}
	case "", "stdio", "listen":
	default:
		return nil, fmt.Errorf("unknown adapter transport %q", a.Transport)
	}
	if a.Transport == "listen" && strings.Contains(strings.Join(a.Command, "\x00"), "${socket}") {
		dir, err := os.MkdirTemp("", "ttt-dap-")
		if err != nil {
			return nil, err
		}
		session, err := a.openProcess(ctx, t, event, options, address, append(vars, "${socket}", filepath.Join(dir, "adapter.sock")))
		if err != nil {
			_ = os.RemoveAll(dir)
			return nil, err
		}
		session.OnCleanup(func() { _ = os.RemoveAll(dir) })
		return session, nil
	}
	return a.openProcess(ctx, t, event, options, address, vars)
}
func (a Adapter) openProcess(ctx context.Context, t Target, event func(Event), options SessionOptions, address string, vars []string) (*Session, error) {
	cmd, err := a.Process(t.Folder, vars...)
	if err != nil {
		return nil, err
	}
	if a.Transport == "" || a.Transport == "stdio" {
		return stdioProcess(ctx, cmd, event, options)
	}
	var pattern *regexp.Regexp
	if a.Transport == "listen" {
		pattern, err = regexp.Compile(a.AddressPattern)
		if err != nil {
			return nil, err
		}
		if pattern.NumSubexp() != 1 {
			return nil, fmt.Errorf("addressPattern requires one capture group")
		}
	}
	return socketProcess(ctx, cmd, address, pattern, event, options)
}

func socketProcess(ctx context.Context, cmd *exec.Cmd, address string, pattern *regexp.Regexp, event func(Event), options SessionOptions) (*Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	isolateAdapter(cmd)
	cmd.Stderr = adapterStderr(event)
	cmd.WaitDelay = time.Second
	addresses := make(chan string, 1)
	reader, writer := io.Pipe()
	cmd.Stdout = writer
	go func() {
		defer reader.Close()
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			if pattern != nil {
				if m := pattern.FindStringSubmatch(scanner.Text()); len(m) == 2 {
					select {
					case addresses <- m[1]:
					default:
					}
				}
			}
		}
	}()
	if err := cmd.Start(); err != nil {
		_ = writer.Close()
		return nil, err
	}
	exited := make(chan struct{})
	var processErr error
	go func() { processErr = cmd.Wait(); _ = writer.Close(); close(exited) }()
	stop := func() {
		select {
		case <-exited:
			return
		default:
			killAdapter(cmd)
			<-exited
		}
	}
	exitError := func() error { return fmt.Errorf("%s exited before connecting (%v)", cmd.Path, processErr) }
	if pattern != nil {
		select {
		case address = <-addresses:
		case <-ctx.Done():
			stop()
			return nil, ctx.Err()
		case <-exited:
			return nil, exitError()
		}
	}
	network := "tcp"
	if filepath.IsAbs(address) {
		network = "unix"
	} else if host, _, err := net.SplitHostPort(address); err != nil || (host != "localhost" && !net.ParseIP(host).IsLoopback()) {
		stop()
		return nil, fmt.Errorf("owned adapter requires a loopback address, got %q", address)
	}
	for {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
		if err == nil {
			return SocketSession(conn, func() {
				select {
				case <-exited:
				case <-time.After(time.Second):
					stop()
				}
			}, event, options), nil
		}
		select {
		case <-ctx.Done():
			stop()
			return nil, ctx.Err()
		case <-exited:
			return nil, exitError()
		case <-time.After(25 * time.Millisecond):
		}
	}
}
