package i18n

import (
	"bytes"
	"encoding/json"
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
				return nil
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || (selector.Sel.Name != "Text" && selector.Sel.Name != "Format") {
					return true
				}
				receiver, ok := selector.X.(*ast.Ident)
				if !ok || receiver.Name != alias {
					return true
				}
				position := fset.Position(call.Pos())
				if len(call.Args) == 0 {
					t.Errorf("%s: missing message ID", position)
					return true
				}
				id, ok := stringLiteral(call.Args[0])
				if !ok {
					t.Errorf("%s: use a literal message ID so references can be validated", position)
					return true
				}
				message, ok := english[id]
				if !ok {
					t.Errorf("%s: message %q is missing from en.json", position, id)
					return true
				}
				used[id] = true
				var supplied []string
				if selector.Sel.Name == "Format" {
					if len(call.Args) != 2 {
						t.Errorf("%s: Format needs named arguments", position)
						return true
					}
					args, ok := call.Args[1].(*ast.CompositeLit)
					if !ok {
						t.Errorf("%s: use an i18n.Args literal so placeholders can be validated", position)
						return true
					}
					for _, element := range args.Elts {
						pair, ok := element.(*ast.KeyValueExpr)
						if !ok {
							t.Errorf("%s: arguments must be named", position)
							continue
						}
						name, ok := stringLiteral(pair.Key)
						if !ok {
							t.Errorf("%s: argument names must be literals", position)
							continue
						}
						supplied = append(supplied, name)
					}
				}
				slices.Sort(supplied)
				if want := placeholders(message); !slices.Equal(supplied, want) {
					t.Errorf("%s: %s supplies %v, catalog needs %v", position, id, supplied, want)
				}
				return true
			})
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

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	return value, err == nil
}
