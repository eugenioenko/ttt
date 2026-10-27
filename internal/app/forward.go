package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/jsonrpc"
)

var ErrForwardUnavailable = errors.New("no running editor")

var forwardCallTimeout = 30 * time.Second

type forwardTarget struct {
	Path []byte
	Line int
	Col  int
}

func forwardDirectory() string {
	return filepath.Join(os.TempDir(), "ttt-"+strconv.Itoa(os.Getuid()))
}

func forwardPrefix() string {
	dir, _ := filepath.Abs(config.UserConfigDir())
	hash := sha256.Sum256([]byte(dir))
	return fmt.Sprintf("%x-", hash[:4])
}

func forwardSocketName(start time.Time, pid int) string {
	return fmt.Sprintf("%s%08s-%s.sock", forwardPrefix(), strconv.FormatInt(start.UnixMilli(), 36), strconv.FormatInt(int64(pid), 36))
}

func checkForwardDir(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || !forwardDirOK(info) {
		return fmt.Errorf("unsafe forwarding directory: %s", dir)
	}
	return nil
}

func ForwardableTargets() []FileTarget {
	ws, targets, _, prURLs := resolveArgs(false)
	if len(targets) == 0 || len(ws.Paths()) != 0 || len(prURLs) != 0 {
		return nil
	}
	return targets
}

func ForwardFiles(targets []FileTarget) error {
	dir := forwardDirectory()
	if err := checkForwardDir(dir); err != nil {
		return ErrForwardUnavailable
	}
	entries, _ := os.ReadDir(dir)
	slices.Reverse(entries)
	prefix := forwardPrefix()
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) || !strings.HasSuffix(entry.Name(), ".sock") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		conn, err := net.DialTimeout("unix", path, time.Second)
		if err != nil {
			if errors.Is(err, syscall.ECONNREFUSED) && entry.Type()&os.ModeSocket != 0 && forwardProcessDead(entry.Name()) {
				_ = os.Remove(path)
			}
			continue
		}
		defer conn.Close()
		rpc := jsonrpc.NewConn(conn, conn, jsonrpc.Options{MaxMessageSize: maxExecBodySize})
		wire := make([]forwardTarget, len(targets))
		for i, target := range targets {
			wire[i] = forwardTarget{Path: []byte(target.Path), Line: target.Line, Col: target.Col}
		}
		ctx, cancel := context.WithTimeout(context.Background(), forwardCallTimeout)
		deadline, _ := ctx.Deadline()
		_ = conn.SetDeadline(deadline)
		_, err = rpc.Call(ctx, "open.v1", wire)
		cancel()
		var netErr net.Error
		if err != nil && (errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) || !time.Now().Before(deadline)) {
			return errors.New("the running editor did not respond")
		}
		var rpcErr *jsonrpc.Error
		if errors.As(err, &rpcErr) {
			return errors.New(rpcErr.Message)
		}
		return err
	}
	return ErrForwardUnavailable
}

func StartForward(a *App) (stop func()) {
	dir := forwardDirectory()
	if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		a.StatusError(fmt.Sprintf("forward: create directory: %v", err))
		return func() {}
	}
	if err := checkForwardDir(dir); err != nil {
		a.StatusError(fmt.Sprintf("forward: check directory: %v", err))
		return func() {}
	}
	path := filepath.Join(dir, forwardSocketName(time.Now(), os.Getpid()))
	ln, err := net.Listen("unix", path)
	if err != nil {
		a.StatusError(fmt.Sprintf("forward: listen: %v", err))
		return func() {}
	}
	go serveForward(a, ln)
	return func() { ln.Close() }
}

func serveForward(a *App, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(execRequestTimeout))
		jsonrpc.NewConn(conn, conn, jsonrpc.Options{
			MaxMessageSize: maxExecBodySize,
			OnClose:        func(error) { conn.Close() },
			OnRequest: func(method string, params json.RawMessage) (any, error) {
				_ = conn.SetReadDeadline(time.Time{})
				if method != "open.v1" {
					return nil, &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "method not supported: " + method}
				}
				var wire []forwardTarget
				if err := json.Unmarshal(params, &wire); err != nil {
					return nil, err
				}
				targets := make([]FileTarget, len(wire))
				for i, target := range wire {
					targets[i] = FileTarget{Path: string(target.Path), Line: target.Line, Col: target.Col}
				}
				return nil, runExecMain(a, func() error { return a.openForwarded(targets) })
			},
		})
	}
}

func (a *App) openForwarded(targets []FileTarget) error {
	opened, err := openFileTargets(a.EditorGroup, targets)
	a.ApplyFileTargets(opened)
	a.FocusEditorIfEnabled()
	return err
}
