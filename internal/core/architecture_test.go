package core_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var prohibitedPresentationImports = []string{
	"github.com/eugenioenko/ttt/internal/highlight",
	"github.com/eugenioenko/ttt/internal/term",
	"github.com/eugenioenko/ttt/internal/render",
	"github.com/eugenioenko/ttt/internal/view",
	"github.com/eugenioenko/ttt/internal/ui",
	"github.com/eugenioenko/ttt/internal/widgets",
	"github.com/gdamore/tcell/v3",
}

func TestCoreDoesNotImportPresentationPackages(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}

		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range parsed.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			for _, prohibited := range prohibitedPresentationImports {
				if importPath == prohibited || strings.HasPrefix(importPath, prohibited+"/") {
					t.Errorf("%s imports presentation dependency %s", path, importPath)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
