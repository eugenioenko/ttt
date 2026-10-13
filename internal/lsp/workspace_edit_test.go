package lsp

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func parseWorkspaceEdit(t *testing.T, raw string) []FileEdits {
	t.Helper()
	var edit WorkspaceEdit
	if err := json.Unmarshal([]byte(raw), &edit); err != nil {
		t.Fatal(err)
	}
	edits, err := edit.TextEdits()
	if err != nil {
		t.Fatal(err)
	}
	return edits
}

func textEdit(line, start, end int, text string) TextEdit {
	return TextEdit{
		Range:   Range{Start: Position{Line: line, Character: start}, End: Position{Line: line, Character: end}},
		NewText: text,
	}
}

func TestWorkspaceEditChanges(t *testing.T) {
	got := parseWorkspaceEdit(t, `{"changes":{"file:///a.ts":[
		{"range":{"start":{"line":1,"character":2},"end":{"line":1,"character":5}},"newText":"bar"}]}}`)
	want := []FileEdits{{URI: "file:///a.ts", Edits: []TextEdit{textEdit(1, 2, 5, "bar")}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("edits = %#v, want %#v", got, want)
	}
}

func TestWorkspaceEditDocumentChanges(t *testing.T) {
	got := parseWorkspaceEdit(t, `{"documentChanges":[
		{"textDocument":{"uri":"file:///a.go","version":3},"edits":[
			{"range":{"start":{"line":0,"character":5},"end":{"line":0,"character":8}},"newText":"bar"}]},
		{"textDocument":{"uri":"file:///b.go","version":null},"edits":[
			{"range":{"start":{"line":2,"character":0},"end":{"line":2,"character":3}},"newText":"bar","annotationId":"rename"}]},
		{"textDocument":{"uri":"file:///a.go","version":3},"edits":[
			{"range":{"start":{"line":4,"character":1},"end":{"line":4,"character":4}},"newText":"bar"}]}]}`)
	want := []FileEdits{
		{URI: "file:///a.go", Edits: []TextEdit{textEdit(0, 5, 8, "bar")}},
		{URI: "file:///b.go", Edits: []TextEdit{textEdit(2, 0, 3, "bar")}},
		{URI: "file:///a.go", Edits: []TextEdit{textEdit(4, 1, 4, "bar")}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("edits = %#v, want %#v", got, want)
	}
}

func TestWorkspaceEditDocumentChangesTakePrecedence(t *testing.T) {
	got := parseWorkspaceEdit(t, `{
		"changes":{"file:///stale.go":[
			{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":1}},"newText":"x"}]},
		"documentChanges":[{"textDocument":{"uri":"file:///a.go","version":1},"edits":[
			{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":3}},"newText":"bar"}]}]}`)
	want := []FileEdits{{URI: "file:///a.go", Edits: []TextEdit{textEdit(0, 0, 3, "bar")}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("edits = %#v, want %#v", got, want)
	}
}

func TestWorkspaceEditFileOperation(t *testing.T) {
	var we WorkspaceEdit
	err := json.Unmarshal([]byte(`{"documentChanges":[
		{"textDocument":{"uri":"file:///a.go","version":1},"edits":[
			{"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":3}},"newText":"bar"}]},
		{"kind":"rename","oldUri":"file:///a.go","newUri":"file:///b.go"}]}`), &we)
	if err != nil {
		t.Fatal(err)
	}
	edits, err := we.TextEdits()
	if !errors.Is(err, ErrFileOperations) {
		t.Fatalf("err = %v, want ErrFileOperations", err)
	}
	if edits != nil {
		t.Fatalf("edits = %#v, want none", edits)
	}
}
