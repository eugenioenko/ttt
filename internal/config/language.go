package config

type LanguageSettings struct {
	Locale     string                `json:"locale,omitempty"`
	AutoUpdate *bool                 `json:"autoUpdate,omitempty"`
	Packs      []LanguagePackSetting `json:"packs,omitempty"`
}

func (s LanguageSettings) IsAutoUpdateEnabled() bool {
	return s.AutoUpdate == nil || *s.AutoUpdate
}

type LanguagePackSetting struct {
	Locale       string `json:"locale"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	SourceHashes string `json:"sourceHashes,omitempty"`
	TTTVersion   string `json:"tttVersion,omitempty"`
	Plugin       string `json:"plugin,omitempty"`
	Repo         string `json:"repo,omitempty"`
	RepoPath     string `json:"repoPath,omitempty"`
	BundleHash   string `json:"bundleHash,omitempty"`
}
