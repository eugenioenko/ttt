package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/command"
)

func TestPaletteRequestsFilesOnlyOnFirstFileMode(t *testing.T) {
	p := NewSelectDialogWidget(nil)
	requests := 0
	p.OnRequestFiles = func() { requests++ }
	for _, input := range []string{">open", ":12", "?", "?editing", ">"} {
		p.Input.SetText(input)
	}
	if requests != 0 {
		t.Fatal("non-file modes requested workspace files")
	}
	for _, input := range []string{"", "old", ">", "latest"} {
		p.Input.SetText(input)
	}
	if requests != 1 || !p.filesLoading {
		t.Fatalf("requests = %d, loading = %v", requests, p.filesLoading)
	}
	if p.calculateLayout(80, 24).visibleItems != 1 {
		t.Fatal("loading message has no row")
	}
	p.SetFiles([]PaletteFile{{Rel: "old.txt", Abs: "/old.txt"}, {Rel: "latest.txt", Abs: "/latest.txt"}})
	if p.filesLoading || len(p.Items) != 1 || p.Items[0].ID != "/latest.txt" {
		t.Fatalf("late files did not use current query: %+v", p.Items)
	}
	if requests != 1 {
		t.Fatal("applying files started another request")
	}
}

func TestPaletteFilesArrivingOutsideFileModePreserveResults(t *testing.T) {
	p := NewSelectDialogWidget([]command.Command{{ID: "test.run", Title: "Run"}})
	p.OnRequestFiles = func() {}
	p.Input.SetText("")
	p.Input.SetText(">Run")
	p.SetFiles([]PaletteFile{{Rel: "file.txt", Abs: "/file.txt"}})
	if len(p.Items) != 1 || p.Items[0].ID != "test.run" {
		t.Fatalf("files replaced command results: %+v", p.Items)
	}
	p.Input.SetText("")
	if len(p.Items) != 1 || p.Items[0].ID != "/file.txt" {
		t.Fatalf("returning to file mode lost files: %+v", p.Items)
	}
}
