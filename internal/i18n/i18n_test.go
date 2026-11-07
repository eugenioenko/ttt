package i18n

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestText(t *testing.T) {
	if got := Text("general.cancel"); got != "Cancel" {
		t.Fatalf("Text = %q", got)
	}
	if got := Text("missing.message"); got != "missing.message" {
		t.Fatalf("unknown message = %q", got)
	}
	if got := Format("status.failedToOpenTerminal", Args{"error": "permission denied"}); got != "Failed to open terminal: permission denied" {
		t.Fatalf("Format = %q", got)
	}
}

func TestInterpolate(t *testing.T) {
	for _, tt := range []struct {
		name, message, want string
		args                Args
	}{
		{"reordered", "{filename}: saved {count} lines", "例子.go: saved 12 lines", Args{"count": 12, "filename": "例子.go"}},
		{"repeated", "{name}, {name}", "hello, hello", Args{"name": "hello"}},
		{"literal values", "{filename}: {error}", "{error}: 50% full", Args{"filename": "{error}", "error": "50% full"}},
		{"missing argument", "Saved {filename}", "Saved {filename}", nil},
		{"extra argument", "Saved", "Saved", Args{"unused": true}},
		{"unclosed brace", "Saved {filename", "Saved {filename", Args{"filename": "file"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := interpolate(tt.message, tt.args); got != tt.want {
				t.Fatalf("interpolate = %q, want %q", got, tt.want)
			}
		})
	}
}

var messageID = regexp.MustCompile(`^[a-z][a-zA-Z0-9]*\.[a-z][a-zA-Z0-9]*$`)
var placeholder = regexp.MustCompile(`\{([a-zA-Z][a-zA-Z0-9]*)\}`)

func placeholders(message string) []string {
	var names []string
	for _, match := range placeholder.FindAllStringSubmatch(message, -1) {
		names = append(names, match[1])
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func TestEnglishCatalog(t *testing.T) {
	dec := json.NewDecoder(bytes.NewReader(englishJSON))
	if opening, err := dec.Token(); err != nil || opening != json.Delim('{') {
		t.Fatalf("catalog must be a flat JSON object: %v", err)
	}
	seen := map[string]bool{}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		id := key.(string)
		if seen[id] {
			t.Errorf("duplicate message ID %q", id)
		}
		seen[id] = true
		if !messageID.MatchString(id) {
			t.Errorf("%q must use area.name", id)
		}
		var message string
		if err := dec.Decode(&message); err != nil {
			t.Fatalf("%s must have a string value: %v", id, err)
		}
		if strings.TrimSpace(message) == "" {
			t.Errorf("%s is empty", id)
		}
		if remaining := placeholder.ReplaceAllString(message, ""); strings.ContainsAny(remaining, "{}") {
			t.Errorf("%s has an invalid named placeholder", id)
		}
	}
}

func TestCatalogReferences(t *testing.T) {
	used := map[string]bool{}
	fset := token.NewFileSet()
	for _, root := range []string{"..", "../../cmd"} {
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			checkCatalogReferences(file, fset, used, t.Errorf)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for id := range english {
		if !used[id] {
			t.Errorf("%s is unused; remove stale copy from en.json", id)
		}
	}
}

func checkCatalogReferences(file *ast.File, fset *token.FileSet, used map[string]bool, report func(string, ...any)) {
	alias := ""
	for _, spec := range file.Imports {
		if spec.Path.Value == `"github.com/eugenioenko/ttt/internal/i18n"` {
			alias = "i18n"
			if spec.Name != nil {
				alias = spec.Name.Name
			}
		}
	}
	if alias == "" {
		return
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (selector.Sel.Name != "Text" && selector.Sel.Name != "Format" && selector.Sel.Name != "Key") {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if !ok || receiver.Name != alias {
			return true
		}
		position := fset.Position(call.Pos())
		if len(call.Args) == 0 {
			report("%s: missing message ID", position)
			return true
		}
		id, ok := stringLiteral(call.Args[0])
		if !ok {
			report("%s: use a literal message ID so references can be validated", position)
			return true
		}
		message, ok := english[id]
		if !ok {
			report("%s: message %q is missing from en.json", position, id)
			return true
		}
		used[id] = true
		var supplied []string
		if selector.Sel.Name == "Format" {
			if len(call.Args) != 2 {
				report("%s: Format needs named arguments", position)
				return true
			}
			args, ok := call.Args[1].(*ast.CompositeLit)
			if !ok {
				report("%s: use an i18n.Args literal so placeholders can be validated", position)
				return true
			}
			for _, element := range args.Elts {
				pair, ok := element.(*ast.KeyValueExpr)
				if !ok {
					report("%s: arguments must be named", position)
					continue
				}
				name, ok := stringLiteral(pair.Key)
				if !ok {
					report("%s: argument names must be literals", position)
					continue
				}
				supplied = append(supplied, name)
			}
		}
		slices.Sort(supplied)
		if want := placeholders(message); !slices.Equal(supplied, want) {
			report("%s: %s supplies %v, catalog needs %v", position, id, supplied, want)
		}
		return true
	})
}

func TestStoredKeyReferences(t *testing.T) {
	for _, tt := range []struct {
		name, source, wantError string
	}{
		{"valid table", `var labels = map[string]i18n.Message{"save": i18n.Key("general.save")}`, ""},
		{"missing table key", `var labels = map[string]i18n.Message{"save": i18n.Key("missing.message")}`, "missing from en.json"},
		{"dynamic key", `var label = i18n.Key(key)`, "literal message ID"},
		{"unformatted key", `var label = i18n.Key("status.failedToOpenTerminal")`, "catalog needs [error]"},
		{"missing direct key", `var label = i18n.Text("missing.message")`, "missing from en.json"},
		{"missing format argument", `var label = i18n.Format("status.failedToOpenTerminal", i18n.Args{})`, "catalog needs [error]"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "table.go", `package example
import "github.com/eugenioenko/ttt/internal/i18n"
`+tt.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			used := map[string]bool{}
			var errors []string
			checkCatalogReferences(file, fset, used, func(format string, args ...any) {
				errors = append(errors, fmt.Sprintf(format, args...))
			})
			if tt.wantError == "" {
				if len(errors) != 0 || !used["general.save"] {
					t.Fatalf("used=%v errors=%v", used, errors)
				}
			} else if len(errors) != 1 || !strings.Contains(errors[0], tt.wantError) {
				t.Fatalf("errors=%v, want %q", errors, tt.wantError)
			}
		})
	}
}

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	return value, err == nil
}
