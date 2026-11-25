package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/eugenioenko/ttt/internal/lsp"
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
