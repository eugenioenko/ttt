package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFindRoslynWorkspacePrefersRootSolution(t *testing.T) {
	root := t.TempDir()
	writeMarkerFile(t, filepath.Join(root, "App.sln"))
	writeMarkerFile(t, filepath.Join(root, "src", "App.csproj"))

	solution, projects := FindRoslynWorkspace(root)
	if solution != filepath.Join(root, "App.sln") {
		t.Errorf("solution = %q", solution)
	}
	if len(projects) != 0 {
		t.Errorf("projects = %v, want none", projects)
	}
}

func TestFindRoslynWorkspaceFindsSolutionInAncestor(t *testing.T) {
	root := t.TempDir()
	writeMarkerFile(t, filepath.Join(root, "App.sln"))
	start := filepath.Join(root, "src", "App")

	solution, _ := FindRoslynWorkspace(start)
	if solution != filepath.Join(root, "App.sln") {
		t.Errorf("solution = %q", solution)
	}
}

func TestFindRoslynWorkspaceFindsSingleNestedSolution(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "src", "App.sln")
	writeMarkerFile(t, nested)

	solution, _ := FindRoslynWorkspace(root)
	if solution != nested {
		t.Errorf("solution = %q, want %q", solution, nested)
	}
}

func TestFindRoslynWorkspaceFallsBackToProjects(t *testing.T) {
	root := t.TempDir()
	writeMarkerFile(t, filepath.Join(root, "src", "A", "A.csproj"))
	writeMarkerFile(t, filepath.Join(root, "src", "B", "B.csproj"))
	writeMarkerFile(t, filepath.Join(root, "src", "A", "bin", "Ignored.csproj"))
	writeMarkerFile(t, filepath.Join(root, "src", "A", "obj", "Ignored.csproj"))

	_, projects := FindRoslynWorkspace(root)
	want := []string{
		filepath.Join(root, "src", "A", "A.csproj"),
		filepath.Join(root, "src", "B", "B.csproj"),
	}
	if len(projects) != len(want) {
		t.Fatalf("projects = %v, want %v", projects, want)
	}
	for i := range want {
		if projects[i] != want[i] {
			t.Errorf("projects[%d] = %q, want %q", i, projects[i], want[i])
		}
	}
}

func TestOpenRoslynWorkspaceSendsSolutionOpen(t *testing.T) {
	root := t.TempDir()
	solution := filepath.Join(root, "App.sln")
	writeMarkerFile(t, solution)
	c, server := pipeClient(t)
	done := make(chan struct{})
	go func() {
		c.OpenRoslynWorkspace(root)
		close(done)
	}()

	message := receiveWithin(t, server)
	<-done
	if message.Method != "solution/open" {
		t.Fatalf("method = %q, want solution/open", message.Method)
	}
	var params roslynSolutionParams
	if err := json.Unmarshal(message.Params, &params); err != nil {
		t.Fatal(err)
	}
	if params.Solution != FileURI(solution) {
		t.Errorf("solution = %q, want %q", params.Solution, FileURI(solution))
	}
}

func TestOpenRoslynWorkspaceSendsProjectOpen(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "App.csproj")
	writeMarkerFile(t, project)
	c, server := pipeClient(t)
	done := make(chan struct{})
	go func() {
		c.OpenRoslynWorkspace(root)
		close(done)
	}()

	message := receiveWithin(t, server)
	<-done
	if message.Method != "project/open" {
		t.Fatalf("method = %q, want project/open", message.Method)
	}
	var params roslynProjectsParams
	if err := json.Unmarshal(message.Params, &params); err != nil {
		t.Fatal(err)
	}
	if len(params.Projects) != 1 || params.Projects[0] != FileURI(project) {
		t.Errorf("projects = %v, want [%q]", params.Projects, FileURI(project))
	}
}

func TestIsRoslynServer(t *testing.T) {
	cases := []struct {
		key     string
		command []string
		want    bool
	}{
		{"roslyn", []string{"roslyn-language-server", "--stdio"}, true},
		{"csharp", []string{"dotnet", "/opt/roslyn/Microsoft.CodeAnalysis.LanguageServer.dll"}, true},
		{"csharp", []string{"csharp-ls"}, false},
		{"gopls", []string{"gopls"}, false},
	}
	for _, tc := range cases {
		if got := isRoslynServer(tc.key, tc.command); got != tc.want {
			t.Errorf("isRoslynServer(%q, %v) = %v, want %v", tc.key, tc.command, got, tc.want)
		}
	}
}

func TestFindRoslynWorkspaceEmptyRoot(t *testing.T) {
	if solution, projects := FindRoslynWorkspace(""); solution != "" || len(projects) != 0 {
		t.Errorf("got %q, %v", solution, projects)
	}
}

func TestFindRoslynWorkspaceMissingRoot(t *testing.T) {
	if solution, projects := FindRoslynWorkspace(filepath.Join(os.TempDir(), "nonexistent-ttt-root")); solution != "" || len(projects) != 0 {
		t.Errorf("got %q, %v", solution, projects)
	}
}
