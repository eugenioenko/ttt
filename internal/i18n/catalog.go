package i18n

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/text/language"
)

type Catalog struct {
	messages map[string]string
}

func NormalizeLocale(locale string) (string, error) {
	if locale == "" {
		return "en", nil
	}
	tag, err := language.Parse(locale)
	if err != nil || tag == language.Und {
		return "", fmt.Errorf("invalid locale %q", locale)
	}
	return tag.String(), nil
}

func DecodeMessages(data []byte) (map[string]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if opening, err := decoder.Token(); err != nil || opening != json.Delim('{') {
		return nil, fmt.Errorf("catalog must be a JSON object")
	}
	messages := make(map[string]string)
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		id := key.(string)
		if _, exists := messages[id]; exists {
			return nil, fmt.Errorf("duplicate message %q", id)
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return nil, err
		}
		var message string
		if len(raw) == 0 || raw[0] != '"' {
			return nil, fmt.Errorf("message %q must be a string", id)
		}
		if err := json.Unmarshal(raw, &message); err != nil {
			return nil, err
		}
		messages[id] = message
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("unexpected data after catalog")
	}
	return messages, nil
}

var namedPlaceholder = regexp.MustCompile(`\{[A-Za-z][A-Za-z0-9]*\}`)

func catalogPlaceholders(message string) []string {
	names := namedPlaceholder.FindAllString(message, -1)
	slices.Sort(names)
	return names
}

func ParseCatalog(data, sourceHashes []byte) (*Catalog, error) {
	messages, err := DecodeMessages(data)
	if err != nil {
		return nil, err
	}
	var hashes map[string]string
	if sourceHashes != nil {
		hashes, err = DecodeMessages(sourceHashes)
		if err != nil {
			return nil, fmt.Errorf("source hashes: %w", err)
		}
	}
	valid := make(map[string]string)
	for id, translated := range messages {
		source, known := english[id]
		if !known || strings.TrimSpace(translated) == "" {
			continue
		}
		if hashes != nil {
			hash := sha256.Sum256([]byte(source))
			if hashes[id] != hex.EncodeToString(hash[:]) {
				continue
			}
		}
		if !slices.Equal(catalogPlaceholders(source), catalogPlaceholders(translated)) {
			continue
		}
		if strings.ContainsAny(namedPlaceholder.ReplaceAllString(translated, ""), "{}") {
			continue
		}
		valid[id] = translated
	}
	return &Catalog{messages: valid}, nil
}
