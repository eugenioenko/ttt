package jsdebug

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

func inspectorWebsocket(ctx context.Context, pid, port int, address string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	host := net.JoinHostPort(address, strconv.Itoa(port))
	verify := func() error {
		ok, err := ownsInspectorListener(ctx, pid, port, host)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("process %d needs an active Node inspector on %s; start the target with --inspect=%s", pid, host, host)
		}
		return nil
	}
	if err := verify(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+host+"/json/list", nil)
	if err != nil {
		return "", err
	}
	transport := &http.Transport{DisableKeepAlives: true}
	client := http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var targets []struct{ ID, Type, WebSocketDebuggerURL string }
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("inspector discovery: %s", response.Status)
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&targets); err != nil {
		return "", err
	}
	if err = verify(); err != nil {
		return "", err
	}
	for _, target := range targets {
		u, err := url.Parse(target.WebSocketDebuggerURL)
		if err == nil && target.Type == "node" && target.ID != "" && u.Scheme == "ws" && u.Host == host && u.User == nil && u.Path == "/"+target.ID {
			return u.String(), nil
		}
	}
	return "", fmt.Errorf("no matching Node inspector target on %s", host)
}

func ownsInspectorListener(ctx context.Context, pid, port int, host string) (bool, error) {
	id := strconv.Itoa(pid)
	if runtime.GOOS == "windows" {
		out, err := exec.CommandContext(ctx, "netstat", "-ano", "-p", "tcp").Output()
		if err != nil {
			return false, err
		}
		for line := range strings.SplitSeq(string(out), "\n") {
			f := strings.Fields(line)
			if len(f) == 5 && f[0] == "TCP" && f[1] == host && f[3] == "LISTENING" && f[4] == id {
				return true, nil
			}
		}
		return false, nil
	}
	out, err := exec.CommandContext(ctx, "lsof", "-nP", "-a", "-p", id, "-iTCP:"+strconv.Itoa(port), "-sTCP:LISTEN", "-Fpn").Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	fields := strings.Fields(string(out))
	return slices.Contains(fields, "p"+id) && slices.Contains(fields, "n"+host), nil
}
