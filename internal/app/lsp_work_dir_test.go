package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/lsp"
	"github.com/eugenioenko/ttt/internal/term"
	"github.com/eugenioenko/ttt/internal/workspace"
)

func lspWorkDirApp(t *testing.T, markers []string) (*App, string, string) {
	t.Helper()
	workspaceDir := t.TempDir()
	project := filepath.Join(workspaceDir, "project")
	src := filepath.Join(project, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "App.csproj"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{
		Settings:  &config.Settings{},
		Workspace: workspace.New([]string{workspaceDir}),
		LspManager: lsp.NewManager(&config.LSPSettings{
			Servers: map[string]config.LSPServerConfig{
				"csharp": {
					Command:     []string{"roslyn-language-server", "--stdio"},
					Languages:   map[string]string{".cs": "csharp"},
					RootMarkers: markers,
				},
			},
		}),
	}
	return a, workspaceDir, filepath.Join(src, "Program.cs")
}

func TestLSPWorkDirUsesRootMarkers(t *testing.T) {
	a, workspaceDir, file := lspWorkDirApp(t, []string{"*.csproj"})

	if got := a.lspWorkDirFor(file, "csharp"); got != filepath.Join(workspaceDir, "project") {
		t.Errorf("workDir = %q, want project root", got)
	}
}

func TestLSPWorkDirFallsBackWithoutRootMarkers(t *testing.T) {
	a, workspaceDir, file := lspWorkDirApp(t, nil)

	if got := a.lspWorkDirFor(file, "csharp"); got != workspaceDir {
		t.Errorf("workDir = %q, want workspace folder %q", got, workspaceDir)
	}
}

func TestLSPWorkDirCachesRootUntilMarkerAppears(t *testing.T) {
	a, workspaceDir, file := lspWorkDirApp(t, []string{"*.csproj"})
	project := filepath.Join(workspaceDir, "project")
	if err := os.Remove(filepath.Join(project, "App.csproj")); err != nil {
		t.Fatal(err)
	}
	if got := a.lspWorkDirFor(file, "csharp"); got != workspaceDir {
		t.Fatalf("workDir = %q, want fallback %q", got, workspaceDir)
	}

	if err := os.WriteFile(filepath.Join(project, "App.csproj"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := a.lspWorkDirFor(file, "csharp"); got != workspaceDir {
		t.Fatalf("workDir = %q, want the cached fallback %q", got, workspaceDir)
	}

	a.HandleExplorerDirChanged(project)
	if got := a.lspWorkDirFor(file, "csharp"); got != project {
		t.Fatalf("workDir = %q, want the newly created project root %q", got, project)
	}
}

func TestLSPWorkDirSeparatesProjectRoots(t *testing.T) {
	workspaceDir := t.TempDir()
	var files []string
	for _, name := range []string{"one", "two"} {
		project := filepath.Join(workspaceDir, name)
		src := filepath.Join(project, "src")
		if err := os.MkdirAll(src, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(project, "App.sln"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, filepath.Join(src, "Program.cs"))
	}
	a := &App{
		Settings:  &config.Settings{},
		Workspace: workspace.New([]string{workspaceDir}),
		LspManager: lsp.NewManager(&config.LSPSettings{
			Servers: map[string]config.LSPServerConfig{
				"csharp": {
					Command:     []string{"roslyn-language-server", "--stdio"},
					Languages:   map[string]string{".cs": "csharp"},
					RootMarkers: []string{"*.sln"},
				},
			},
		}),
	}

	for i, name := range []string{"one", "two"} {
		want := filepath.Join(workspaceDir, name)
		if got := a.lspWorkDirFor(files[i], "csharp"); got != want {
			t.Errorf("workDir for %s = %q, want its own project root %q", files[i], got, want)
		}
	}
}

func TestLSPDocumentKeepsRootItWasOpenedOn(t *testing.T) {
	t.Setenv(fakePullServerEnv, "intrafile")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	a := buildTestApp(t, config.DefaultSettings())
	a.LspManager = lsp.NewManager(&config.LSPSettings{Servers: map[string]config.LSPServerConfig{
		"go": {Command: []string{exe}, Languages: map[string]string{".go": "go"}, RootMarkers: []string{"go.mod"}},
	}})
	t.Cleanup(a.LspManager.Shutdown)

	project := filepath.Join(t.TempDir(), "project")
	src := filepath.Join(project, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(src, "a.go")
	if err := os.WriteFile(path, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	requireNoError(t, a.EditorGroup.OpenFile(path))
	a.NotifyLSPOpen(path, "go", []string{"package a", ""})
	if got := a.lspWorkDirFor(path, "go"); got != project {
		t.Fatalf("workDir = %q, want the root it was opened on %q", got, project)
	}

	// A nearer marker appears: the open document must stay on the client that
	// received its didOpen rather than move to the new root's client.
	if err := os.WriteFile(filepath.Join(src, "go.mod"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	a.HandleExplorerDirChanged(src)
	if got := a.lspWorkDirFor(path, "go"); got != project {
		t.Fatalf("open document moved to %q after a nearer marker appeared, want %q", got, project)
	}

	// Closing it releases the pin, so the next open resolves the nearer root.
	a.NotifyLSPClose(path, "go")
	if got := a.lspWorkDirFor(path, "go"); got != src {
		t.Fatalf("workDir after close = %q, want the nearer root %q", got, src)
	}
}

func TestLSPRestartRerootsOpenDocuments(t *testing.T) {
	t.Setenv(fakePullServerEnv, "intrafile")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	a := buildTestApp(t, config.DefaultSettings())
	a.LspManager = lsp.NewManager(&config.LSPSettings{Servers: map[string]config.LSPServerConfig{
		"go": {Command: []string{exe}, Languages: map[string]string{".go": "go"}, RootMarkers: []string{"go.mod"}},
	}})
	sim := term.NewSimScreen()
	if err := sim.Init(); err != nil {
		t.Fatal(err)
	}
	a.Screen = term.NewTcellScreenFrom(sim)
	t.Cleanup(func() {
		a.LspManager.Shutdown()
		a.CloseTasks()
		a.Repository.Close()
		a.EditorGroup.CloseAllTabs()
		sim.Fini()
	})

	project := filepath.Join(t.TempDir(), "project")
	src := filepath.Join(project, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(src, "a.go")
	if err := os.WriteFile(path, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	requireNoError(t, a.EditorGroup.OpenFile(path))
	a.NotifyLSPOpen(path, "go", []string{"package a", ""})
	if got := a.lspWorkDirFor(path, "go"); got != project {
		t.Fatalf("workDir = %q, want %q", got, project)
	}

	// The marker appears without a watcher event; an explicit restart is the
	// documented way to re-root already open documents.
	if err := os.WriteFile(filepath.Join(src, "go.mod"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	a.handleLSPServerStopped(&LSPServerStopped{Server: "go", WorkDir: project})
	if got := a.lspWorkDirFor(path, "go"); got != src {
		t.Fatalf("workDir after restart = %q, want the nearer root %q", got, src)
	}
}

func TestLSPWorkDirCacheDropsWhenRootMarkersChange(t *testing.T) {
	workspaceDir := t.TempDir()
	project := filepath.Join(workspaceDir, "project")
	src := filepath.Join(project, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "App.csproj"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(src, "Program.cs")

	settings := config.DefaultSettings()
	settings.LSP.Servers = map[string]config.LSPServerConfig{
		"csharp": {
			Command:     []string{"roslyn-language-server", "--stdio"},
			Languages:   map[string]string{".cs": "csharp"},
			RootMarkers: []string{"*.csproj"},
		},
	}
	a := buildTestApp(t, settings)
	a.Workspace = workspace.New([]string{workspaceDir})
	a.LspManager = lsp.NewManager(&a.Settings.LSP)
	a.ApplySettings(settings)

	if got := a.lspWorkDirForServer(file, "csharp"); got != project {
		t.Fatalf("workDir = %q, want project root %q", got, project)
	}

	changed := settings
	changed.LSP.Servers = map[string]config.LSPServerConfig{
		"csharp": {
			Command:   []string{"roslyn-language-server", "--stdio"},
			Languages: map[string]string{".cs": "csharp"},
		},
	}
	a.ApplySettings(changed)

	if got := a.lspWorkDirForServer(file, "csharp"); got != workspaceDir {
		t.Fatalf("workDir = %q, want workspace folder %q after markers were removed", got, workspaceDir)
	}
}
