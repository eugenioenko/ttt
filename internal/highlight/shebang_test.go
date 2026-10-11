package highlight

import "testing"

func TestNewForLinesShebang(t *testing.T) {
	for _, tc := range []struct {
		line     string
		language string
	}{
		{"#!/bin/bash", "Shell"},
		{"#!/bin/sh", "Shell"},
		{"#!/bin/zsh", "Shell"},
		{"#!/bin/dash", "Shell"},
		{"#!/usr/bin/env bash", "Shell"},
		{"#! /usr/bin/env\tpython3\r", "Python"},
		{"#!/usr/bin/python3.12 -u", "Python"},
		{"#!/usr/bin/env python3", "Python"},
		{"#!/usr/bin/env -S python3 -u", "Python"},
		{"#!/usr/bin/env -i MODE=test python3", "Python"},
		{"#!/usr/bin/env --split-string node --trace-warnings", "JavaScript"},
		{"#!/usr/bin/env node", "JavaScript"},
		{"#!/usr/bin/nodejs", "JavaScript"},
		{"#!/usr/bin/ruby", "Ruby"},
		{"", ""},
		{"#!", ""},
		{"#!/usr/bin/env", ""},
		{"#!/usr/bin/env -S", ""},
		{"#!/usr/bin/env -u bash python3", ""},
		{"#!/usr/bin/unknown-ttt-interpreter", ""},
		{" #!/bin/bash", ""},
		{"# /bin/bash", ""},
		{"echo '#!/bin/bash'", ""},
	} {
		t.Run(tc.line, func(t *testing.T) {
			h := NewForLines("/tmp/backup", []string{tc.line})
			if tc.language == "" {
				if h != nil {
					t.Fatalf("unexpected language %q", h.Language())
				}
				return
			}
			if h == nil || h.Language() != tc.language {
				t.Fatalf("highlighter = %v, want %s", h, tc.language)
			}
		})
	}
}

func TestNewForLinesFilenamePriority(t *testing.T) {
	for _, name := range []string{"main.go", "main.go.bak", "Dockerfile", "Gemfile"} {
		want := New(name).Language()
		h := NewForLines(name, []string{"#!/usr/bin/env python3"})
		if h == nil || h.Language() != want {
			t.Fatalf("%s highlighter = %v, want %s", name, h, want)
		}
	}
	if h := NewForLines("backup.unknown-ttt", []string{"#!/usr/bin/env python3"}); h == nil || h.Language() != "Python" {
		t.Fatalf("unknown extension highlighter = %v, want Python", h)
	}
	if NewForLines("backup", nil) != nil || NewForLines("backup", []string{"", "#!/bin/bash"}) != nil {
		t.Fatal("only the first line should determine the fallback")
	}
}

func TestNewForLinesExternalFilenamePriority(t *testing.T) {
	if errs := setExternal(t, ExternalGrammar{
		Path: writeGrammar(t, "foo.json", fooGrammar), FileTypes: []string{"backup"},
	}); len(errs) > 0 {
		t.Fatal(errs)
	}
	h := NewForLines("backup", []string{"#!/bin/bash"})
	if h == nil || h.Language() != "Foo" {
		t.Fatalf("highlighter = %v, want Foo", h)
	}
}
