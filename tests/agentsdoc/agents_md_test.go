// Package agentsdoc_test keeps AGENTS.md honest: every file path and Go or
// JavaScript identifier it names in a code span must still exist in the repository.
package agentsdoc_test

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	codeSpanRe   = regexp.MustCompile("`([^`\n]+)`")
	identRe      = regexp.MustCompile(`^(?:[A-Za-z_]\w*\.)*([A-Za-z_]\w*)(\(\))?(\*)?$`)
	pathRe       = regexp.MustCompile(`^[\w.*/-]+$`)
	identTokenRe = regexp.MustCompile(`[A-Za-z_]\w*`)
	pathRoots    = []string{"internal/", "core/", "tests/", "cmd/", "docs-web/", "config/", "scripts/", ".github/"}
	fileExts     = []string{".go", ".md", ".json", ".yaml", ".yml", ".js", ".lua", ".sh"}
	skippedDirs  = map[string]bool{".git": true, ".claude": true, "node_modules": true, "bin": true, "dist": true, "chaos-output": true}
	userFiles    = map[string]bool{"settings.json": true}
	placeholders = map[string]bool{"domain.verbNoun": true}
	nonRepoPaths = []string{"~", "/", "http", ".", "bin/"}
)

func TestAgentsMDReferencesExist(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	goMod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	files, idents := scanRepo(t, root)

	var missing []string
	seen := map[string]bool{}
	for _, m := range codeSpanRe.FindAllStringSubmatch(string(doc), -1) {
		ref := m[1]
		if seen[ref] || placeholders[ref] || strings.Contains(string(goMod), ref) {
			continue
		}
		seen[ref] = true
		switch {
		case isPathRef(ref):
			if !pathExists(files, ref) {
				missing = append(missing, "path "+ref)
			}
		case identRe.MatchString(ref):
			sub := identRe.FindStringSubmatch(ref)
			name, prefix := sub[1], sub[3] == "*"
			if strings.ToLower(name) == name {
				continue
			}
			if !identExists(idents, name, prefix) {
				missing = append(missing, "identifier "+ref)
			}
		}
	}
	for _, ref := range missing {
		t.Errorf("AGENTS.md references %s, which no longer exists; update AGENTS.md", ref)
	}
}

func isPathRef(ref string) bool {
	if !pathRe.MatchString(ref) || userFiles[ref] {
		return false
	}
	for _, p := range nonRepoPaths {
		if strings.HasPrefix(ref, p) {
			return false
		}
	}
	hasExt := false
	for _, ext := range fileExts {
		if strings.HasSuffix(ref, ext) {
			hasExt = true
		}
	}
	if !strings.Contains(ref, "/") {
		return hasExt
	}
	if hasExt || strings.HasSuffix(ref, "/") {
		return true
	}
	for _, r := range pathRoots {
		if strings.HasPrefix(ref, r) {
			return true
		}
	}
	return false
}

// pathExists resolves a reference as written, relative to internal/, or, for a
// bare file name or glob, against any file's base name.
func pathExists(files []string, ref string) bool {
	ref = strings.TrimSuffix(ref, "/")
	if !strings.Contains(ref, "/") {
		for _, f := range files {
			if ok, _ := path.Match(ref, path.Base(f)); ok {
				return true
			}
		}
		return false
	}
	for _, cand := range []string{ref, "internal/" + ref} {
		for _, f := range files {
			if ok, _ := path.Match(cand, f); ok || strings.HasPrefix(f, cand+"/") {
				return true
			}
			if ok, _ := path.Match(cand, path.Dir(f)); ok {
				return true
			}
		}
	}
	return false
}

func identExists(idents map[string]bool, name string, prefix bool) bool {
	if !prefix {
		return idents[name]
	}
	for id := range idents {
		if strings.HasPrefix(id, name) {
			return true
		}
	}
	return false
}

func scanRepo(t *testing.T, root string) ([]string, map[string]bool) {
	t.Helper()
	var files []string
	idents := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && skippedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		if strings.HasSuffix(p, ".go") || strings.HasSuffix(p, ".js") {
			src, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			for _, id := range identTokenRe.FindAllString(string(src), -1) {
				idents[id] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files, idents
}
