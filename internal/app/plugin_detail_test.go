package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/plugin"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/ui"
)

type readmeTransport func(*http.Request) (*http.Response, error)

func (f readmeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func readmeTestApp(t *testing.T, transport readmeTransport) *App {
	t.Helper()
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() { http.DefaultTransport = previous })
	a := buildTestApp(t, config.DefaultSettings())
	a.PluginManager = plugin.NewManager(t.TempDir(), filepath.Join(t.TempDir(), "plugins.json"))
	a.EditorGroup.OnContentTabClose = a.cleanupPluginDetailTab
	t.Cleanup(a.CloseTasks)
	return a
}

func readmeResponse(text string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(text))}, nil
}

func nextReadmeDelivery(t *testing.T, deliveries <-chan func()) func() {
	t.Helper()
	select {
	case fn := <-deliveries:
		return fn
	case <-time.After(2 * time.Second):
		t.Fatal("README work did not queue UI delivery")
		return nil
	}
}

func pluginReadmeText(state *pluginDetailState) string {
	r := ui.Rect{W: 100, H: 10}
	cells := make([][]term.Cell, r.H)
	for y := range cells {
		cells[y] = make([]term.Cell, r.W)
	}
	state.markdown.SetRect(r)
	state.markdown.Render(ui.NewRenderSurface(cells, r))
	var text strings.Builder
	for _, row := range cells {
		for _, cell := range row {
			text.WriteRune(cell.Ch)
		}
		text.WriteByte('\n')
	}
	return text.String()
}

func TestPluginReadmeRootFallback(t *testing.T) {
	pluginURL := "https://raw.githubusercontent.com/owner/repo/main/plugin/README.md"
	rootURL := "https://raw.githubusercontent.com/owner/repo/main/README.md"

	tests := []struct {
		name     string
		statuses map[string]int // URL -> status to serve
		want     []string       // fragments expected in the rendered view
		counts   map[string]int // URL -> expected request count
	}{
		{
			name:     "plugin README loads directly",
			statuses: map[string]int{pluginURL: http.StatusOK, rootURL: http.StatusOK},
			want:     []string{"plugin readme loaded"},
			counts:   map[string]int{pluginURL: 1, rootURL: 0},
		},
		{
			name:     "subdirectory 404 falls back to root README",
			statuses: map[string]int{pluginURL: http.StatusNotFound, rootURL: http.StatusOK},
			want:     []string{"root readme loaded"},
			counts:   map[string]int{pluginURL: 1, rootURL: 1},
		},
		{
			name:     "both READMEs missing shows friendly message with repo link",
			statuses: map[string]int{pluginURL: http.StatusNotFound, rootURL: http.StatusNotFound},
			want:     []string{"No documentation is available", "https://github.com/owner/repo"},
			counts:   map[string]int{pluginURL: 1, rootURL: 1},
		},
		{
			name:     "non-404 plugin error is not masked as missing",
			statuses: map[string]int{pluginURL: http.StatusInternalServerError, rootURL: http.StatusOK},
			want:     []string{"Could not load README: readme returned status 500"},
			counts:   map[string]int{pluginURL: 1, rootURL: 0},
		},
		{
			name:     "non-404 root error is not masked as missing",
			statuses: map[string]int{pluginURL: http.StatusNotFound, rootURL: http.StatusInternalServerError},
			want:     []string{"Could not load README: readme returned status 500"},
			counts:   map[string]int{pluginURL: 1, rootURL: 1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			counts := map[string]int{}
			a := readmeTestApp(t, func(req *http.Request) (*http.Response, error) {
				u := req.URL.String()
				counts[u]++
				body := "**unknown**"
				switch u {
				case pluginURL:
					body = "**plugin readme loaded**"
				case rootURL:
					body = "**root readme loaded**"
				}
				response, err := readmeResponse(body)
				if status, ok := tc.statuses[u]; ok {
					response.StatusCode = status
				} else {
					response.StatusCode = http.StatusNotFound
				}
				return response, err
			})
			a.OpenPluginDetail(plugin.RemoteRegistryEntry{Name: "example", Repo: "https://github.com/owner/repo", Path: "plugin"})
			text := pluginReadmeText(a.pluginDetailWidgets["example"])
			for _, want := range tc.want {
				if !strings.Contains(text, want) {
					t.Fatalf("README view = %q, want fragment %q", text, want)
				}
			}
			for u, wantCount := range tc.counts {
				if counts[u] != wantCount {
					t.Fatalf("requests to %s = %d, want %d", u, counts[u], wantCount)
				}
			}
		})
	}
}

func TestPluginReadmeAtRepoRootFetchesOnce(t *testing.T) {
	var requests int
	a := readmeTestApp(t, func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.String() != "https://raw.githubusercontent.com/owner/repo/main/README.md" {
			t.Errorf("README URL = %s", req.URL)
		}
		return readmeResponse("**root readme loaded**")
	})
	// No Path: the plugin lives at the repository root, so the root README is
	// the only URL worth requesting.
	a.OpenPluginDetail(plugin.RemoteRegistryEntry{Name: "example", Repo: "https://github.com/owner/repo"})
	text := pluginReadmeText(a.pluginDetailWidgets["example"])
	if requests != 1 {
		t.Fatalf("requests = %d, want exactly 1", requests)
	}
	if !strings.Contains(text, "root readme loaded") {
		t.Fatalf("README view = %q, want root readme", text)
	}
}

func TestPluginReadmeQueuedReplyCannotUpdateClosedOrReplacedTab(t *testing.T) {
	for _, action := range []string{"close and reopen", "replace", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			var reads atomic.Int32
			a := readmeTestApp(t, func(*http.Request) (*http.Response, error) {
				return readmeResponse(fmt.Sprintf("README request %d", reads.Add(1)))
			})
			deliveries := make(chan func(), 2)
			a.tasks.post = func(fn func()) error { deliveries <- fn; return nil }
			baselineTasks := len(a.tasks.tasks)
			entry := plugin.RemoteRegistryEntry{Name: "example", Repo: "https://github.com/owner/repo"}
			a.OpenPluginDetail(entry)
			old := a.pluginDetailWidgets[entry.Name]
			stale := nextReadmeDelivery(t, deliveries)
			switch action {
			case "close and reopen":
				a.EditorGroup.ClosePluginTab("plugin-detail:" + entry.Name)
				a.OpenPluginDetail(entry)
			case "replace":
				a.OpenPluginDetail(entry)
			case "shutdown":
				a.CloseTasks()
			}
			if action != "shutdown" {
				nextReadmeDelivery(t, deliveries)()
				if len(a.tasks.tasks) != baselineTasks+1 {
					t.Fatalf("registered tasks = %d, want %d", len(a.tasks.tasks), baselineTasks+1)
				}
				if text := pluginReadmeText(a.pluginDetailWidgets[entry.Name]); !strings.Contains(text, "README request 2") {
					t.Fatalf("replacement README = %q", text)
				}
			}
			stale()
			if text := pluginReadmeText(old); !strings.Contains(text, "Loading README") {
				t.Fatalf("invalidated request updated the old widget: %q", text)
			}
			if action != "shutdown" {
				if text := pluginReadmeText(a.pluginDetailWidgets[entry.Name]); !strings.Contains(text, "README request 2") {
					t.Fatalf("stale request replaced the new README: %q", text)
				}
			}
		})
	}
}

func TestPluginReadmeTabsLoadIndependently(t *testing.T) {
	a := readmeTestApp(t, func(req *http.Request) (*http.Response, error) {
		return readmeResponse(req.URL.Path)
	})
	deliveries := make(chan func(), 2)
	a.tasks.post = func(fn func()) error { deliveries <- fn; return nil }
	for _, name := range []string{"first", "second"} {
		a.OpenPluginDetail(plugin.RemoteRegistryEntry{Name: name, Repo: "https://github.com/owner/" + name})
	}
	for range 2 {
		nextReadmeDelivery(t, deliveries)()
	}
	for _, name := range []string{"first", "second"} {
		if text := pluginReadmeText(a.pluginDetailWidgets[name]); !strings.Contains(text, "/owner/"+name+"/main/README.md") {
			t.Fatalf("README for %s = %q", name, text)
		}
	}
}

func TestPluginReadmeCloseAndShutdownCancelHTTP(t *testing.T) {
	for _, shutdown := range []bool{false, true} {
		t.Run(fmt.Sprintf("shutdown=%v", shutdown), func(t *testing.T) {
			started := make(chan context.Context, 1)
			finished := make(chan struct{})
			a := readmeTestApp(t, func(req *http.Request) (*http.Response, error) {
				started <- req.Context()
				<-req.Context().Done()
				close(finished)
				return nil, req.Context().Err()
			})
			deliveries := make(chan func(), 1)
			a.tasks.post = func(fn func()) error { deliveries <- fn; return nil }
			remainingTasks := len(a.tasks.tasks)
			a.OpenPluginDetail(plugin.RemoteRegistryEntry{Name: "example", Repo: "https://github.com/owner/repo"})
			var ctx context.Context
			select {
			case ctx = <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("README HTTP request did not start")
			}
			if shutdown {
				a.CloseTasks()
				remainingTasks = 0
			} else {
				a.EditorGroup.ClosePluginTab("plugin-detail:example")
			}
			select {
			case <-finished:
			case <-time.After(2 * time.Second):
				t.Fatal("README HTTP request did not observe cancellation")
			}
			if ctx.Err() == nil || len(a.tasks.tasks) != remainingTasks {
				t.Fatal("closed README retained its request or registered task")
			}
		})
	}
}
