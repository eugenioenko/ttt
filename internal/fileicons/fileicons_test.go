package fileicons

import "testing"

func TestForFileMatchesExactFileNameBeforeExtension(t *testing.T) {
	// go.mod has its own entry; a plain .mod file does not share it.
	if got, want := ForFile("go.mod"), byFilename["go.mod"]; got != want {
		t.Fatalf("go.mod = %+v, want filename entry %+v", got, want)
	}
	if got := ForFile("Makefile"); got != byFilename["makefile"] {
		t.Fatalf("Makefile = %+v, want case-insensitive filename entry %+v", got, byFilename["makefile"])
	}
}

func TestForFilePrefersLongestDottedSuffix(t *testing.T) {
	if got, want := ForFile("button.test.ts"), byExtension["test.ts"]; got != want {
		t.Fatalf("button.test.ts = %+v, want test.ts entry %+v", got, want)
	}
	if got, want := ForFile("main.go"), byExtension["go"]; got != want {
		t.Fatalf("main.go = %+v, want go entry %+v", got, want)
	}
	if got, want := ForFile("README.MD"), byFilename["readme.md"]; got != want {
		t.Fatalf("README.MD = %+v, want readme.md entry %+v", got, want)
	}
	if got, want := ForFile("notes.MD"), byExtension["md"]; got != want {
		t.Fatalf("notes.MD = %+v, want md entry %+v", got, want)
	}
}

func TestForFileFallsBackToGenericIcon(t *testing.T) {
	for _, name := range []string{"", "no-extension", "archive.unknownext", "trailing."} {
		if got := ForFile(name); got != defaultFile {
			t.Errorf("ForFile(%q) = %+v, want default %+v", name, got, defaultFile)
		}
	}
}

func TestForFolderSwitchesOnExpansion(t *testing.T) {
	if ForFolder(false) == ForFolder(true) {
		t.Fatal("open and closed folders share a glyph")
	}
	if ForFolder(false).Glyph == "" || ForFolder(true).Glyph == "" {
		t.Fatal("folder glyph is empty")
	}
}

func TestGeneratedTablesAreWellFormed(t *testing.T) {
	for name, table := range map[string]map[string]Icon{"byFilename": byFilename, "byExtension": byExtension} {
		if len(table) < 100 {
			t.Errorf("%s has %d entries; the generator output looks truncated", name, len(table))
		}
		for key, icon := range table {
			if icon.Glyph == "" {
				t.Errorf("%s[%q] has no glyph", name, key)
			}
			if icon.Color > ColorMagenta {
				t.Errorf("%s[%q] has unknown color %d", name, key, icon.Color)
			}
		}
	}
}
