package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/core/undo"
	"github.com/eugenioenko/ttt/internal/jsonrpc"
	"github.com/eugenioenko/ttt/internal/lsp"
)

func TestSaveAsSynchronizesOpenDestinationBeforeSaving(t *testing.T) {
	for _, destination := range []string{"open", "new", "alias"} {
		t.Run(destination, func(t *testing.T) {
			logPath := filepath.Join(t.TempDir(), "notifications.jsonl")
			t.Setenv(fakePullSyncLogEnv, logPath)
			a, paths := fakePullServersApp(t, false, "target.go", "source.go")
			t.Cleanup(func() { a.CloseTasks(); a.LspManager.Shutdown() })
			a.EditorGroup.Editor.ExecCommand(&undo.InsertRuneCommand{Line: 0, Col: 0, Rune: 'X'})
			wantText := strings.Join(a.EditorGroup.ActiveBuffer().Lines, "\n")
			savePath, document := paths[0], paths[0]
			if destination == "new" {
				savePath = filepath.Join(filepath.Dir(paths[0]), "new.go")
				document = savePath
			} else if destination == "alias" {
				savePath = filepath.Join(filepath.Dir(paths[0]), "link.go")
				if err := os.Symlink(document, savePath); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			}
			a.saveFileAs(savePath, nil)
			a.DocVersionsMu.Lock()
			sent := a.lspSent[document]
			a.DocVersionsMu.Unlock()
			select {
			case <-sent:
			case <-time.After(5 * time.Second):
				t.Fatal("save notification was not sent")
			}
			client, err := a.LspManager.ClientForLanguage("go", a.lspWorkDir(document))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, _, err := client.Diagnostic(ctx, FileURI(document)); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			var methods []string
			for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
				var message jsonrpc.Message
				if err := json.Unmarshal([]byte(line), &message); err != nil {
					t.Fatal(err)
				}
				var params struct {
					TextDocument   lsp.TextDocumentIdentifier           `json:"textDocument"`
					ContentChanges []lsp.TextDocumentContentChangeEvent `json:"contentChanges"`
					Text           string                               `json:"text"`
				}
				if err := json.Unmarshal(message.Params, &params); err != nil {
					t.Fatal(err)
				}
				if params.TextDocument.URI != FileURI(document) || message.Method == "textDocument/didOpen" {
					continue
				}
				methods = append(methods, message.Method)
				if message.Method == "textDocument/didChange" && (len(params.ContentChanges) != 1 || params.ContentChanges[0].Text != wantText) {
					t.Fatalf("change did not contain the replacement text: %+v", params.ContentChanges)
				}
				if message.Method == "textDocument/didSave" && params.Text != wantText {
					t.Fatalf("saved text = %q, want %q", params.Text, wantText)
				}
			}
			want := []string{"textDocument/didSave"}
			if destination != "new" {
				want = append([]string{"textDocument/didChange"}, want...)
			}
			if !slices.Equal(methods, want) {
				t.Fatalf("destination notifications = %v, want %v", methods, want)
			}
		})
	}
}
