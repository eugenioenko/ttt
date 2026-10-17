package plugin

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/eugenioenko/ttt/internal/term"
)

func TestDecorationsSet(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{EditorDecorations: true})
	defer cleanup()

	var gotPath string
	var gotItems []DecorationItem
	p.SetDecorations = func(path string, items []DecorationItem) {
		gotPath, gotItems = path, items
	}

	err := p.State.DoString(`
		local deco = require("ttt.decorations")
		deco.set("/tmp/x.txt", {
			{ line = 3, end_line = 5 },
			{ line = 4, col = 2, end_col = 7 },
			{ line = 6, col = 3, end_line = 8, end_col = 1, style = "highlight" },
			{ line = 9, style = "selected" },
			{ line = 10, end_line = math.huge },
		})
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if gotPath != "/tmp/x.txt" {
		t.Errorf("path = %q", gotPath)
	}
	want := []DecorationItem{
		{StartLine: 2, EndLine: 4, Style: term.StyleHighlight},
		{StartLine: 3, StartCol: 1, EndLine: 3, EndCol: 6, Text: true, Style: term.StyleHighlightStrong},
		{StartLine: 5, StartCol: 2, EndLine: 7, EndCol: 0, Text: true, Style: term.StyleHighlight},
		{StartLine: 8, EndLine: 8, Style: term.StyleSidebarSelected},
		{StartLine: 9, EndLine: math.MaxInt32 - 1, Style: term.StyleHighlight},
	}
	if !reflect.DeepEqual(gotItems, want) {
		t.Errorf("items =\n%+v\nwant\n%+v", gotItems, want)
	}
}

func TestDecorationsSetRejectsBadItems(t *testing.T) {
	cases := map[string]string{
		"{ { col = 1, end_col = 2 } }":                         "line is required",
		"{ { line = 1.5 } }":                                   "line must be a whole number",
		"{ { line = 2, col = \"3\", end_col = 6 } }":           "col must be a whole number",
		"{ { line = 3, end_line = 2 } }":                       "end_line is before line",
		"{ { line = 1, col = 2 } }":                            "col and end_col go together",
		"{ { line = 1, col = 4, end_col = 4 } }":               "text range is empty",
		"{ { line = 1, style = \"no_such\" } }":                "unknown style no_such",
		"{ { line = 1 }, \"not a table\" }":                    "items must be tables",
		"{ { line = 1 }, { line = 2, col = 0, end_col = 3 } }": "item 2: col must be a whole number >= 1",
	}
	for items, want := range cases {
		p, cleanup := newTestPluginBase(PermissionSet{EditorDecorations: true})
		called := false
		p.SetDecorations = func(string, []DecorationItem) { called = true }
		err := p.State.DoString(`require("ttt.decorations").set("/tmp/x.txt", ` + items + `)`)
		cleanup()
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", items, err, want)
		}
		if called {
			t.Errorf("%s: decorations were set despite the error", items)
		}
	}
}

func TestDecorationsClear(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{EditorDecorations: true})
	defer cleanup()

	var got []string
	p.ClearDecorations = func(path string) { got = append(got, path) }

	err := p.State.DoString(`
		local deco = require("ttt.decorations")
		deco.clear("/tmp/x.txt")
		deco.clear()
	`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"/tmp/x.txt", ""}) {
		t.Errorf("clear paths = %q", got)
	}
}

func TestDecorationsWithoutPermission(t *testing.T) {
	p, cleanup := newTestPluginBase(PermissionSet{})
	defer cleanup()

	err := p.State.DoString(`
		local deco = require("ttt.decorations")
		deco.set("/tmp/x.txt", {})
	`)
	if err == nil {
		t.Fatal("expected error when editor.decorations not granted")
	}
}
