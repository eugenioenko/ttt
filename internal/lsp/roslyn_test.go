package lsp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// roslynBoundary returns a directory holding two solutions, which discovery
// reads as ambiguous and stops at, so a search never ascends into directories
// outside the fixture.
func roslynBoundary(t *testing.T) string {
	t.Helper()
	boundary := t.TempDir()
	writeMarkerFile(t, filepath.Join(boundary, "One.sln"))
	writeMarkerFile(t, filepath.Join(boundary, "Two.sln"))
	return boundary
}

func roslynFixtureRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(roslynBoundary(t), "project")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestFindRoslynWorkspacePrefersRootSolution(t *testing.T) {
	root := t.TempDir()
	writeMarkerFile(t, filepath.Join(root, "App.sln"))
	writeMarkerFile(t, filepath.Join(root, "src", "App.csproj"))

	solution, projects, err := FindRoslynWorkspace(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
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

	solution, _, err := FindRoslynWorkspace(context.Background(), start)
	if err != nil {
		t.Fatal(err)
	}
	if solution != filepath.Join(root, "App.sln") {
		t.Errorf("solution = %q", solution)
	}
}

func TestFindRoslynWorkspaceFindsSingleNestedSolution(t *testing.T) {
	root := roslynFixtureRoot(t)
	nested := filepath.Join(root, "src", "App.sln")
	writeMarkerFile(t, nested)

	solution, _, err := FindRoslynWorkspace(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if solution != nested {
		t.Errorf("solution = %q, want %q", solution, nested)
	}
}

func TestFindRoslynWorkspaceFallsBackToProjects(t *testing.T) {
	root := roslynFixtureRoot(t)
	writeMarkerFile(t, filepath.Join(root, "src", "A", "A.csproj"))
	writeMarkerFile(t, filepath.Join(root, "src", "B", "B.csproj"))
	writeMarkerFile(t, filepath.Join(root, "src", "A", "bin", "Ignored.csproj"))
	writeMarkerFile(t, filepath.Join(root, "src", "A", "obj", "Ignored.csproj"))

	_, projects, err := FindRoslynWorkspace(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
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
		_ = c.OpenRoslynWorkspace(context.Background(), root)
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
	root := roslynFixtureRoot(t)
	project := filepath.Join(root, "App.csproj")
	writeMarkerFile(t, project)
	c, server := pipeClient(t)
	done := make(chan struct{})
	go func() {
		_ = c.OpenRoslynWorkspace(context.Background(), root)
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

// signalingWriteCloser reports when the first frame starts writing, so a test
// can cancel only once the notification is blocked on an unread pipe.
type signalingWriteCloser struct {
	*io.PipeWriter
	started chan struct{}
	once    sync.Once
}

func (w *signalingWriteCloser) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.started) })
	return w.PipeWriter.Write(p)
}

func TestOpenRoslynWorkspaceStopsWhenCanceled(t *testing.T) {
	root := t.TempDir()
	writeMarkerFile(t, filepath.Join(root, "App.sln"))
	clientR, clientW := io.Pipe()
	writer := &signalingWriteCloser{PipeWriter: clientW, started: make(chan struct{})}
	c := newClient(clientR, writer, ClientHooks{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- c.OpenRoslynWorkspace(ctx, root) }()
	select {
	case <-writer.started:
	case <-time.After(5 * time.Second):
		t.Fatal("client never wrote solution/open")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("OpenRoslynWorkspace waited for an unread stdin write")
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
	if solution, projects, err := FindRoslynWorkspace(context.Background(), ""); solution != "" || len(projects) != 0 || err != nil {
		t.Errorf("got %q, %v, %v", solution, projects, err)
	}
}

func TestFindRoslynWorkspaceMissingRoot(t *testing.T) {
	root := filepath.Join(roslynBoundary(t), "nonexistent-ttt-root")
	if solution, projects, err := FindRoslynWorkspace(context.Background(), root); solution != "" || len(projects) != 0 || err != nil {
		t.Errorf("got %q, %v, %v", solution, projects, err)
	}
}

func TestFindProjectFilesStopsWalkingWhenCanceled(t *testing.T) {
	root := t.TempDir()
	writeMarkerFile(t, filepath.Join(root, "a.csproj"))
	writeMarkerFile(t, filepath.Join(root, "b.csproj"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	match := func(name string) bool {
		if name == "a.csproj" {
			cancel()
		}
		return isProjectFile(name)
	}

	projects, err := findProjectFiles(ctx, root, match, maxRoslynProjects)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if projects != nil {
		t.Errorf("projects = %v, want none", projects)
	}
}

func TestFindRoslynWorkspaceStopsWhenCanceled(t *testing.T) {
	root := t.TempDir()
	writeMarkerFile(t, filepath.Join(root, "src", "App.csproj"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	solution, projects, err := FindRoslynWorkspace(ctx, root)
	if err == nil {
		t.Fatalf("got solution %q projects %v, want the cancellation error", solution, projects)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
