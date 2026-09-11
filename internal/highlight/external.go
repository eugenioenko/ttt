package highlight

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/eugenioenko/ttt/internal/config"
)

// External lexers add languages chroma does not ship. Every *.xml file in a
// config dir's lexers/ folder is a serialised chroma lexer; registering it
// makes each highlight.New caller (editor tabs, diff views, markdown fences,
// readonly tabs) match the language by filename, with no code change per
// language.
var (
	externalOnce sync.Once

	// externalErrors records rejected files so the caller can surface them in
	// the OUTPUT panel instead of failing silently.
	externalErrors []error
)

// The scan blocks startup before the first frame, so a file big enough to cost
// seconds of parsing is refused rather than parsed.
const maxLexerFileBytes = 4 << 20

// ExternalLexerErrors returns the load failures from the external lexer scan.
// Safe to call after any highlight.New; the scan runs once.
func ExternalLexerErrors() []error {
	externalOnce.Do(loadExternalLexers)
	return externalErrors
}

func loadExternalLexers() {
	externalErrors = loadExternalLexersInto(lexers.GlobalLexerRegistry)
}

// loadExternalLexersInto registers every lexer file found under the config
// dirs into reg. Production passes chroma's global registry; a test can pass
// its own, because chroma has no way to unregister.
func loadExternalLexersInto(reg *chroma.LexerRegistry) []error {
	var errs []error
	seenFile := map[string]bool{}
	claimed := map[string]string{}

	for _, dir := range config.ConfigDirs() {
		lexerDir := filepath.Join(dir, "lexers")
		entries, err := os.ReadDir(lexerDir)
		if err != nil {
			continue
		}
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".xml") {
				continue
			}
			// Earlier config dirs win a filename outright, matching how
			// themes and settings are resolved.
			if seenFile[e.Name()] {
				continue
			}
			seenFile[e.Name()] = true
			names = append(names, e.Name())
		}
		// Deterministic scan order: which file claims a lexer name, and the
		// order of reported errors, must not depend on the directory listing.
		sort.Strings(names)
		for _, name := range names {
			if err := loadExternalLexer(reg, lexerDir, name, claimed); err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", filepath.Join(lexerDir, name), err))
			}
		}
	}
	return errs
}

func loadExternalLexer(reg *chroma.LexerRegistry, dir, name string, claimed map[string]string) error {
	path := filepath.Join(dir, name)
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() > maxLexerFileBytes {
		return fmt.Errorf("%d bytes is over the %d byte limit for a lexer file", info.Size(), maxLexerFileBytes)
	}

	lexer, err := chroma.NewXMLLexer(os.DirFS(dir), name)
	if err != nil {
		return err
	}
	langName := lexer.Config().Name
	if langName == "" {
		return fmt.Errorf("lexer declares no <name>")
	}
	// Two files declaring the same language: the first one scanned keeps it.
	if first, ok := claimed[langName]; ok {
		return fmt.Errorf("name %q is already declared by %s", langName, first)
	}
	// chroma's Register replaces by name, so a file calling itself Go would
	// take over every .go file in the editor without a word.
	if existing := reg.Get(langName); existing != nil {
		return fmt.Errorf("name %q is already taken by the %s lexer", langName, existing.Config().Name)
	}

	// `<using lexer="…"/>` resolves its delegate through the registry and
	// panics without one; Register would attach it only after the probe.
	lexer.SetRegistry(reg)
	if err := validatePushStates(lexer); err != nil {
		return err
	}
	if err := probeLexer(lexer); err != nil {
		return err
	}

	claimed[langName] = path
	reg.Register(lexer)
	return nil
}

// validatePushStates rejects a lexer that pushes a state it never defines.
// Chroma resolves a push target while tokenising and panics on an unknown
// one, so this is the only check that covers rules the probe below never
// reaches.
func validatePushStates(lx *chroma.RegexLexer) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("rules are unusable: %v", r)
		}
	}()
	rules := lx.MustRules()
	for state, list := range rules {
		for _, rule := range list {
			for _, target := range pushTargets(rule.Mutator) {
				if strings.HasPrefix(target, "#") {
					continue
				}
				if _, ok := rules[target]; !ok {
					return fmt.Errorf("state %q pushes unknown state %q", state, target)
				}
			}
		}
	}
	return nil
}

// pushTargets reports the states a mutator pushes. Chroma's push mutator is an
// unexported type, so marshalling it back to XML is the only way to read its
// targets through the public API.
func pushTargets(m chroma.Mutator) []string {
	sm, ok := m.(chroma.SerialisableMutator)
	if !ok {
		return nil
	}
	switch sm.MutatorKind() {
	case "push", "mutators":
	default:
		return nil
	}
	blob, err := xml.Marshal(m)
	if err != nil {
		return nil
	}
	var out []string
	dec := xml.NewDecoder(bytes.NewReader(blob))
	for {
		tok, err := dec.Token()
		if err != nil {
			return out
		}
		start, ok := tok.(xml.StartElement)
		if !ok || !strings.HasPrefix(start.Name.Local, "push") {
			continue
		}
		for _, attr := range start.Attr {
			if attr.Name.Local == "state" {
				out = append(out, attr.Value)
			}
		}
	}
}

// probeLexer rejects a lexer that cannot tokenise its root state at all: a bad
// regex or an unusable rule set surfaces here rather than at the first
// keystroke. It proves nothing about states the probe text never enters.
func probeLexer(lx chroma.Lexer) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("lexer panicked: %v", r)
		}
	}()
	iter, err := lx.Tokenise(nil, "probe\n")
	if err != nil {
		return err
	}
	iter.Tokens()
	return nil
}
