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

const (
	forwardSocketEnv     = "TTT_FORWARD_SOCKET"
	forwardFilesMethod   = "open.v1"
	forwardFoldersMethod = "open.v2"
)

type forwardTarget struct {
	Path []byte
	Line int
	Col  int
}

type forwardRequest struct {
	Files   []forwardTarget `json:"files"`
	Folders [][]byte        `json:"folders"`
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

// ForwardableTargets excludes implicit cwd roots, saved workspaces, and PR URLs.
// The caller must gate startup-only flags and piped stdin before forwarding.
func ForwardableTargets() (files []FileTarget, folders []string) {
	args := parseOpenArgs()
	if args.workspaceFile != "" || len(args.prURLs) != 0 {
		return nil, nil
	}
	return args.files, args.folders
}

// ForwardTargets returns ErrForwardUnavailable only before connecting to an editor.
func ForwardTargets(files []FileTarget, folders []string) (int, error) {
	prefix := forwardPrefix()
	preferred := filepath.Clean(os.Getenv(forwardSocketEnv))
	name := filepath.Base(preferred)
	triedPreferred := false
	if filepath.IsAbs(preferred) && strings.HasPrefix(name, prefix) && strings.HasSuffix(name, ".sock") && checkForwardDir(filepath.Dir(preferred)) == nil {
		triedPreferred = true
		if conn, err := dialForward(preferred); err == nil {
			return sendForwardRequest(conn, files, folders)
		}
	}
	dir := forwardDirectory()
	if err := checkForwardDir(dir); err != nil {
		return 0, ErrForwardUnavailable
	}
	entries, _ := os.ReadDir(dir)
	slices.Reverse(entries)
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), prefix) || !strings.HasSuffix(entry.Name(), ".sock") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if triedPreferred && path == preferred {
			continue
		}
		conn, err := dialForward(path)
		if err != nil {
			continue
		}
		return sendForwardRequest(conn, files, folders)
	}
	return 0, ErrForwardUnavailable
}

func dialForward(path string) (net.Conn, error) {
	conn, err := net.DialTimeout("unix", path, time.Second)
	if errors.Is(err, syscall.ECONNREFUSED) && forwardProcessDead(filepath.Base(path)) {
		if info, statErr := os.Lstat(path); statErr == nil && info.Mode()&os.ModeSocket != 0 {
			_ = os.Remove(path)
		}
	}
	return conn, err
}

func sendForwardRequest(conn net.Conn, files []FileTarget, folders []string) (int, error) {
	defer conn.Close()
	rpc := jsonrpc.NewConn(conn, conn, jsonrpc.Options{MaxMessageSize: maxExecBodySize})
	wire := make([]forwardTarget, len(files))
	for i, target := range files {
		wire[i] = forwardTarget{Path: []byte(target.Path), Line: target.Line, Col: target.Col}
	}
	// Keep file-only requests compatible with editors started before folder forwarding.
	method, params := forwardFilesMethod, any(wire)
	if len(folders) > 0 {
		request := forwardRequest{Files: wire, Folders: make([][]byte, len(folders))}
		for i, folder := range folders {
			request.Folders[i] = []byte(folder)
		}
		method, params = forwardFoldersMethod, request
	}
	ctx, cancel := context.WithTimeout(context.Background(), forwardCallTimeout)
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)
	result, err := rpc.Call(ctx, method, params)
	cancel()
	var netErr net.Error
	if err != nil && (errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) || !time.Now().Before(deadline)) {
		return 0, errors.New("the running editor did not respond")
	}
	var rpcErr *jsonrpc.Error
	if errors.As(err, &rpcErr) {
		if method == forwardFoldersMethod && rpcErr.Code == jsonrpc.CodeMethodNotFound {
			return 0, errors.New("the running editor does not support folder forwarding; restart it with this version of ttt or use --new-window")
		}
		return 0, errors.New(rpcErr.Message)
	}
	if err != nil {
		return 0, err
	}
	var pid int
	if err := json.Unmarshal(result, &pid); err != nil {
		pid = 0
	}
	return pid, nil
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
	a.forwardSocket = path
	go serveForward(a, ln)
	return func() {
		ln.Close()
		if a.forwardSocket == path {
			a.forwardSocket = ""
		}
	}
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
				var request forwardRequest
				var err error
				switch method {
				case forwardFilesMethod:
					err = json.Unmarshal(params, &request.Files)
				case forwardFoldersMethod:
					err = json.Unmarshal(params, &request)
				default:
					return nil, &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "method not supported: " + method}
				}
				if err != nil {
					return nil, err
				}
				targets := make([]FileTarget, len(request.Files))
				for i, target := range request.Files {
					targets[i] = FileTarget{Path: string(target.Path), Line: target.Line, Col: target.Col}
				}
				folders := make([]string, len(request.Folders))
				for i, folder := range request.Folders {
					folders[i] = string(folder)
				}
				return os.Getpid(), runExecMain(a, func() error { return a.openForwarded(targets, folders) })
			},
		})
	}
}

func (a *App) openForwarded(files []FileTarget, folders []string) error {
	folderErr := a.addWorkspaceFolders(folders)
	if len(files) == 0 {
		return folderErr
	}
	opened, err := openFileTargets(a.EditorGroup, files)
	a.ApplyFileTargets(opened)
	a.FocusEditorIfEnabled()
	return errors.Join(folderErr, err)
}
