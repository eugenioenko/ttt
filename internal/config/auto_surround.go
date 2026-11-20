package config

import (
	"encoding/json"
	"fmt"
	"slices"
	"unicode"
	"unicode/utf8"
)

type SurroundingPair [2]string

func (p *SurroundingPair) UnmarshalJSON(data []byte) error {
	var pair []string
	if err := json.Unmarshal(data, &pair); err != nil {
		return err
	}
	if len(pair) != 2 {
		return fmt.Errorf("autoSurround.extraPairs entries must contain an opening and closing character")
	}
	for _, text := range pair {
		r, _ := utf8.DecodeRuneInString(text)
		if utf8.RuneCountInString(text) != 1 || !unicode.IsGraphic(r) || unicode.IsSpace(r) {
			return fmt.Errorf("autoSurround.extraPairs requires single, non-whitespace characters: %q", text)
		}
	}
	*p = SurroundingPair{pair[0], pair[1]}
	return nil
}

type AutoSurroundSettings struct {
	Enabled    *bool             `json:"enabled,omitempty"`
	ExtraPairs []SurroundingPair `json:"extraPairs,omitempty"`
}

func (s AutoSurroundSettings) IsEnabled() bool { return s.Enabled == nil || *s.Enabled }

func (e EditorSettings) ResolveAutoSurround(language string) AutoSurroundSettings {
	s := e.AutoSurround
	if o := e.LanguageOverrides[language].AutoSurround; o != nil {
		if o.Enabled != nil {
			s.Enabled = o.Enabled
		}
		s.ExtraPairs = append(slices.Clone(s.ExtraPairs), o.ExtraPairs...)
	}
	return s
}

func (s AutoSurroundSettings) Closer(open rune) rune {
	if !s.IsEnabled() {
		return 0
	}
	switch open {
	case '(':
		return ')'
	case '[':
		return ']'
	case '{':
		return '}'
	case '\'', '"', '`':
		return open
	}
	for i := len(s.ExtraPairs) - 1; i >= 0; i-- {
		pair := s.ExtraPairs[i]
		if pair[0] == string(open) && utf8.RuneCountInString(pair[1]) == 1 {
			close, _ := utf8.DecodeRuneInString(pair[1])
			return close
		}
	}
	return 0
}
