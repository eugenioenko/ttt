package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/eugenioenko/ttt/internal/i18n"
)

const maxLanguageDataBytes = 2 << 20

func ReadLanguageData(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readLanguageData(file)
}

func readLanguageData(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxLanguageDataBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxLanguageDataBytes {
		return nil, fmt.Errorf("language data exceeds %d bytes", maxLanguageDataBytes)
	}
	return data, nil
}

func releaseNumbers(version string) ([3]int, bool) {
	var result [3]int
	version = strings.TrimPrefix(version, "v")
	version, _, _ = strings.Cut(version, "-")
	version, _, _ = strings.Cut(version, "+")
	parts := strings.Split(version, ".")
	if len(parts) != len(result) {
		return result, false
	}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return result, false
		}
		result[i] = n
	}
	return result, true
}

func LanguageNeedsUpdate(editorVersion, catalogVersion string) bool {
	editor, valid := releaseNumbers(editorVersion)
	if !valid {
		return false
	}
	catalog, valid := releaseNumbers(catalogVersion)
	if !valid {
		return true
	}
	for i, part := range editor {
		if part != catalog[i] {
			return part > catalog[i]
		}
	}
	return false
}

type LanguageUpdateSource struct {
	Name, Repo, RepoPath string
	Languages            []Language
}

type LanguageUpdate struct {
	Languages []Language
	Files     map[string][]byte
}

func languageRepository(repo string) (string, error) {
	var repository string
	for _, prefix := range []string{"https://github.com/", "git@github.com:"} {
		if strings.HasPrefix(repo, prefix) {
			repository = strings.TrimPrefix(repo, prefix)
			break
		}
	}
	repository = strings.TrimSuffix(strings.TrimSuffix(repository, "/"), ".git")
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("language updates require a public GitHub repository")
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("invalid language repository")
		}
		for _, ch := range part {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || strings.ContainsRune("._-", ch)) {
				return "", fmt.Errorf("invalid language repository")
			}
		}
	}
	return repository, nil
}

func fetchLanguageData(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("language download returned status %d", response.StatusCode)
	}
	return readLanguageData(response.Body)
}

func FetchLanguageUpdate(ctx context.Context, client *http.Client, source LanguageUpdateSource, editorVersion string) (LanguageUpdate, error) {
	repository, err := languageRepository(source.Repo)
	if err != nil {
		return LanguageUpdate{}, err
	}
	if source.RepoPath != "" && (!filepath.IsLocal(source.RepoPath) || strings.Contains(source.RepoPath, "\\")) {
		return LanguageUpdate{}, fmt.Errorf("invalid language repository path")
	}
	data, err := fetchLanguageData(ctx, client, "https://api.github.com/repos/"+repository+"/commits/HEAD")
	if err != nil {
		return LanguageUpdate{}, err
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(data, &commit); err != nil {
		return LanguageUpdate{}, err
	}
	if len(commit.SHA) != 40 {
		return LanguageUpdate{}, fmt.Errorf("invalid repository commit")
	}
	for _, ch := range commit.SHA {
		if !strings.ContainsRune("0123456789abcdef", ch) {
			return LanguageUpdate{}, fmt.Errorf("invalid repository commit")
		}
	}
	base := "https://raw.githubusercontent.com/" + repository + "/" + commit.SHA + "/"
	data, err = fetchLanguageData(ctx, client, base+path.Join(source.RepoPath, "plugin.ttt.json"))
	if err != nil {
		return LanguageUpdate{}, err
	}
	var manifest struct {
		Languages []Language `json:"languages"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return LanguageUpdate{}, err
	}
	available := make(map[string]Language)
	for _, contribution := range manifest.Languages {
		locale, err := i18n.NormalizeLocale(contribution.Locale)
		if err != nil || contribution.Locale == "" || locale == "en" || contribution.Name == "" || !filepath.IsLocal(contribution.Path) || strings.Contains(contribution.Path, "\\") || (contribution.SourceHashes != "" && (!filepath.IsLocal(contribution.SourceHashes) || strings.Contains(contribution.SourceHashes, "\\"))) {
			return LanguageUpdate{}, fmt.Errorf("invalid language update manifest")
		}
		if _, exists := available[locale]; exists {
			return LanguageUpdate{}, fmt.Errorf("duplicate language update locale %q", locale)
		}
		contribution.Locale = locale
		available[locale] = contribution
	}
	result := LanguageUpdate{Files: make(map[string][]byte)}
	for _, installed := range source.Languages {
		contribution, exists := available[installed.Locale]
		if !exists {
			return LanguageUpdate{}, fmt.Errorf("updated manifest no longer provides %s", installed.Locale)
		}
		if _, valid := releaseNumbers(contribution.TTTVersion); !valid || LanguageNeedsUpdate(editorVersion, contribution.TTTVersion) {
			return LanguageUpdate{}, nil
		}
		data, err := fetchLanguageData(ctx, client, base+path.Join(source.RepoPath, contribution.Path))
		if err != nil {
			return LanguageUpdate{}, err
		}
		var hashes []byte
		if contribution.SourceHashes != "" {
			hashes, err = fetchLanguageData(ctx, client, base+path.Join(source.RepoPath, contribution.SourceHashes))
			if err != nil {
				return LanguageUpdate{}, err
			}
		}
		if _, err := i18n.ParseCatalog(data, hashes); err != nil {
			return LanguageUpdate{}, err
		}
		result.Files[installed.Locale+".json"] = data
		if contribution.SourceHashes != "" {
			result.Files[installed.Locale+".source.json"] = hashes
		}
		result.Languages = append(result.Languages, contribution)
	}
	return result, nil
}
