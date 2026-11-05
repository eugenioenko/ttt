package app

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/jsonrpc"
	"github.com/gdamore/tcell/v3"
)

func TestForwardTargetsProtocol(t *testing.T) {
	file := FileTarget{Path: "/file-\xff", Line: 3, Col: 2}
	for _, tc := range []struct {
		name    string
		files   []FileTarget
		folders []string
		method  string
	}{
		{"files", []FileTarget{file}, nil, "open.v1"},
		{"folders", nil, []string{"/folder-\xff"}, "open.v2"},
		{"files and folders", []FileTarget{file}, []string{"/folder-\xff"}, "open.v2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			type receivedRequest struct {
				method string
				params json.RawMessage
			}
			received := make(chan receivedRequest, 1)
			client, server := net.Pipe()
			defer server.Close()
			jsonrpc.NewConn(server, server, jsonrpc.Options{
				OnRequest: func(method string, params json.RawMessage) (any, error) {
					received <- receivedRequest{method, params}
					return os.Getpid(), nil
				},
			})
			if pid, err := sendForwardRequest(client, tc.files, tc.folders); err != nil || pid != os.Getpid() {
				t.Fatalf("sendForwardRequest = (%d, %v), want (%d, nil)", pid, err, os.Getpid())
			}
			wire := <-received
			if wire.method != tc.method {
				t.Fatalf("method = %q, want %q", wire.method, tc.method)
			}
			var request forwardRequest
			if tc.method == "open.v1" {
				if err := json.Unmarshal(wire.params, &request.Files); err != nil {
					t.Fatal(err)
				}
			} else {
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(wire.params, &fields); err != nil {
					t.Fatal(err)
				}
				if fields["files"] == nil || fields["folders"] == nil {
					t.Fatalf("v2 request missing files or folders: %s", wire.params)
				}
				if err := json.Unmarshal(wire.params, &request); err != nil {
					t.Fatal(err)
				}
			}
			if len(request.Files) != len(tc.files) || len(request.Folders) != len(tc.folders) {
				t.Fatalf("target count changed: %+v", request)
			}
			for i, target := range request.Files {
				if string(target.Path) != tc.files[i].Path || target.Line != tc.files[i].Line || target.Col != tc.files[i].Col {
					t.Errorf("file target changed: %+v", target)
				}
			}
			for i, folder := range request.Folders {
				if string(folder) != tc.folders[i] {
					t.Errorf("folder bytes changed: %q", folder)
				}
			}
		})
	}
}

func TestForwardTargetsUnsupportedReceiver(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	jsonrpc.NewConn(server, server, jsonrpc.Options{
		OnRequest: func(method string, _ json.RawMessage) (any, error) {
			return nil, &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "method not supported: " + method}
		},
	})
	_, err := sendForwardRequest(client, nil, []string{"/folder"})
	if err == nil || !strings.Contains(err.Error(), "restart") || !strings.Contains(err.Error(), "--new-window") {
		t.Fatalf("unsupported receiver error = %v, want restart or --new-window guidance", err)
	}
}

func TestOpenForwardedFoldersPreservesEditor(t *testing.T) {
	saved := config.OverrideConfigDir
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = saved })
	settings := config.DefaultSettings()
	settings.Editor.FocusOnOpen = true
	a := buildTestApp(t, settings)
	file := filepath.Join(t.TempDir(), "unsaved.txt")
	if err := a.openForwarded([]FileTarget{{Path: file}}, nil); err != nil {
		t.Fatal(err)
	}
	editor := a.EditorGroup.Editor
	editor.HandleEvent(tcell.NewEventKey(tcell.KeyRune, "x", tcell.ModNone))
	a.Root.SetFocus(a.Search)
	cursor := *editor.Cursor
	text := slices.Clone(editor.Buf.Lines)
	first := t.TempDir()
	second := filepath.Join(first, "nested")
	if err := os.Mkdir(second, 0700); err != nil {
		t.Fatal(err)
	}
	a.Workspace.FilePath = filepath.Join(t.TempDir(), "saved.ttt")
	workspaceFile := a.Workspace.FilePath
	for _, folders := range [][]string{{first}, {first, second}} {
		tabs := a.EditorGroup.OpenFilePaths()
		if err := a.openForwarded(nil, folders); err != nil {
			t.Fatal(err)
		}
		if a.EditorGroup.Editor != editor || !slices.Equal(a.EditorGroup.OpenFilePaths(), tabs) || !slices.Equal(editor.Buf.Lines, text) || !editor.Buf.Dirty || *editor.Cursor != cursor || a.Root.Focused != a.Search {
			t.Fatal("adding folders changed the active editor, tabs, buffer, cursor, or focus")
		}
		if !slices.Equal(a.Workspace.Paths(), folders) || !slices.Equal(a.Explorer.Roots, folders) || !slices.Equal(a.Search.WorkDirs, folders) || a.Workspace.FilePath != workspaceFile {
			t.Fatalf("workspace, explorer, and search do not match folders %v", folders)
		}
	}
	root := a.Explorer.Tree.Config.Items[0]
	if err := a.openForwarded(nil, []string{first}); err != nil {
		t.Fatal(err)
	}
	if a.Explorer.Tree.Config.Items[0] != root {
		t.Fatal("duplicate folder unnecessarily rebuilt the explorer")
	}
}

func TestOpenForwardedFoldersPartialFailure(t *testing.T) {
	saved := config.OverrideConfigDir
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = saved })
	a := buildTestApp(t, config.DefaultSettings())
	folder := t.TempDir()
	missing := filepath.Join(folder, "missing")
	file := filepath.Join(folder, "new.txt")
	err := a.openForwarded([]FileTarget{{Path: file}}, []string{missing, folder})
	if err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("error = %v, want missing folder %s", err, missing)
	}
	if !slices.Equal(a.Workspace.Paths(), []string{folder}) || a.EditorGroup.ActiveFilePath() != file {
		t.Fatal("invalid folder prevented valid targets from opening")
	}
}
