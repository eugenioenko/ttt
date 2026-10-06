package highlight

import (
	"fmt"
	"os"
	"strings"
	"sync"

	textmate "github.com/eugenioenko/textmate-go"
	"github.com/eugenioenko/textmate-go/grammars"
)

type ExternalGrammar struct {
	Path      string
	Language  string
	FileTypes []string
}

type grammarRef struct {
	scopeName string
	language  string
}

type grammarCatalog struct {
	registry  *textmate.Registry
	fileTypes map[string]grammarRef
	languages map[string]grammarRef
}

var (
	catalogMu sync.RWMutex
	catalog   = newCatalog(nil, nil, nil)
)

func currentCatalog() *grammarCatalog {
	catalogMu.RLock()
	defer catalogMu.RUnlock()
	return catalog
}

// SetExternalGrammars replaces the grammars loaded from disk. Earlier entries
// take precedence over later ones, and all of them over the embedded set.
// Invalid files are skipped and reported. Existing Highlighters keep the
// grammar they were built with.
func SetExternalGrammars(list []ExternalGrammar) []error {
	raw := map[string]*textmate.RawGrammar{}
	fileTypes := map[string]grammarRef{}
	languages := map[string]grammarRef{}
	var errs []error
	for _, ext := range list {
		data, err := os.ReadFile(ext.Path)
		if err != nil {
			errs = append(errs, fmt.Errorf("grammar %s: %w", ext.Path, err))
			continue
		}
		g, err := textmate.ParseRawGrammar(data)
		if err != nil {
			errs = append(errs, fmt.Errorf("grammar %s: %w", ext.Path, err))
			continue
		}
		if _, dup := raw[g.ScopeName]; dup {
			continue
		}
		raw[g.ScopeName] = g

		shortName := g.ScopeName[strings.LastIndexByte(g.ScopeName, '.')+1:]
		ref := grammarRef{scopeName: g.ScopeName, language: ext.Language}
		if ref.language == "" {
			ref.language = g.Name
		}
		if ref.language == "" {
			ref.language = shortName
		}
		types := ext.FileTypes
		if len(types) == 0 {
			types = g.FileTypes
		}
		for _, ft := range types {
			addRef(fileTypes, strings.TrimPrefix(ft, "."), ref)
		}
		addRef(languages, ref.language, ref)
		addRef(languages, shortName, ref)
		for _, ft := range types {
			addRef(languages, strings.TrimPrefix(ft, "."), ref)
		}
	}

	next := newCatalog(raw, fileTypes, languages)
	catalogMu.Lock()
	catalog = next
	catalogMu.Unlock()
	return errs
}

func addRef(m map[string]grammarRef, key string, ref grammarRef) {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return
	}
	if _, ok := m[key]; !ok {
		m[key] = ref
	}
}

func newCatalog(raw map[string]*textmate.RawGrammar, fileTypes, languages map[string]grammarRef) *grammarCatalog {
	load := func(scopeName string) (*textmate.RawGrammar, error) {
		if g := raw[scopeName]; g != nil {
			return g, nil
		}
		return grammars.Load(scopeName)
	}
	return &grammarCatalog{
		registry:  textmate.NewRegistry(textmate.RegistryOptions{LoadGrammar: load}),
		fileTypes: fileTypes,
		languages: languages,
	}
}

func (c *grammarCatalog) forFilename(name string) (grammarRef, bool) {
	for {
		if ref, ok := c.externalForFilename(name); ok {
			return ref, true
		}
		if info, ok := grammars.InfoForFilename(name); ok {
			return refFromInfo(info), true
		}
		if id, ok := extraFilenames[strings.ToLower(name)]; ok {
			info, ok := grammars.InfoForID(id)
			return refFromInfo(info), ok
		}
		trimmed := name
		for _, suffix := range backupSuffixes {
			trimmed = strings.TrimSuffix(trimmed, suffix)
		}
		if trimmed == name || trimmed == "" {
			return grammarRef{}, false
		}
		name = trimmed
	}
}

func (c *grammarCatalog) externalForFilename(name string) (grammarRef, bool) {
	if len(c.fileTypes) == 0 {
		return grammarRef{}, false
	}
	base := strings.ToLower(name)
	if ref, ok := c.fileTypes[base]; ok {
		return ref, true
	}
	trimmed := strings.TrimPrefix(base, ".")
	for {
		if ref, ok := c.fileTypes[trimmed]; ok {
			return ref, true
		}
		dot := strings.IndexByte(trimmed, '.')
		if dot < 0 {
			return grammarRef{}, false
		}
		trimmed = trimmed[dot+1:]
	}
}

func (c *grammarCatalog) forLanguage(lang string) (grammarRef, bool) {
	if ref, ok := c.languages[strings.ToLower(strings.TrimSpace(lang))]; ok {
		return ref, true
	}
	info, ok := grammars.InfoForID(lang)
	if !ok {
		info, ok = grammars.InfoForAlias(lang)
	}
	return refFromInfo(info), ok
}

func refFromInfo(info grammars.GrammarInfo) grammarRef {
	return grammarRef{scopeName: info.ScopeName, language: info.DisplayName}
}
