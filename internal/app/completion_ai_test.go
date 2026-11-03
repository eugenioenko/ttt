package app

import (
	"reflect"
	"testing"

	"github.com/eugenioenko/ttt/internal/ai"
	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/ui"
)

func TestMergedCompletionItemsPutAIItemsFirst(t *testing.T) {
	aiItems := []ui.CompletionItem{{Label: "Z AI suggestion", SortText: "\x00ttt-ai:Z AI suggestion"}}
	lspItems := []ui.CompletionItem{{Label: "A LSP suggestion"}}

	got := mergedCompletionItems(aiItems, lspItems)
	if len(got) != 2 || got[0].Label != "Z AI suggestion" || got[1].Label != "A LSP suggestion" {
		t.Fatalf("merged completion order = %v, want AI before LSP", got)
	}
	filtered := ui.FilterCompletions(got, "")
	if len(filtered) != 2 || filtered[0].Label != "Z AI suggestion" {
		t.Fatalf("filtered completion order = %v, want AI first", filtered)
	}
}

func TestAIContext(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		cursor   int
		maxLines int
		expected string
	}{
		{name: "short buffer", lines: []string{"a", "b"}, cursor: 1, maxLines: 80, expected: "a\nb"},
		{name: "centered window", lines: []string{"0", "1", "2", "3", "4", "5"}, cursor: 3, maxLines: 3, expected: "2\n3\n4"},
		{name: "window at end", lines: []string{"0", "1", "2", "3", "4"}, cursor: 4, maxLines: 2, expected: "3\n4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &buffer.Buffer{Lines: tt.lines}
			if got := aiContext(buf, tt.cursor, tt.maxLines); got != tt.expected {
				t.Fatalf("aiContext() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestAICompletionItems(t *testing.T) {
	got := aiCompletionItems([]ai.Completion{
		{Text: "Println()", Detail: "AI"},
		{Text: "\n", Detail: "empty"},
		{Text: "Printf(\"%s\\n\", value)", Detail: "AI"},
	}, "fmt.P")
	want := []struct {
		label, filter, insert, detail string
	}{
		{label: "Println()", filter: "fmt.PPrintln()", insert: "Println()", detail: "AI"},
		{label: "Printf(\"%s\\n\", value)", filter: "fmt.PPrintf(\"%s\\n\", value)", insert: "Printf(\"%s\\n\", value)", detail: "AI"},
	}
	if len(got) != len(want) {
		t.Fatalf("completion count = %d, want %d", len(got), len(want))
	}
	for i, item := range got {
		actual := struct {
			label, filter, insert, detail string
		}{item.Label, item.FilterText, item.InsertText, item.Detail}
		if !reflect.DeepEqual(actual, want[i]) {
			t.Errorf("completion[%d] = %+v, want %+v", i, actual, want[i])
		}
	}
}
