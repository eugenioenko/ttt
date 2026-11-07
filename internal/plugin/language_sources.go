package plugin

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/eugenioenko/ttt/internal/i18n"
)

func validSourceCommit(commit string) bool {
	if len(commit) != 40 {
		return false
	}
	for _, ch := range commit {
		if !strings.ContainsRune("0123456789abcdef", ch) {
			return false
		}
	}
	return true
}

func LanguageSourcePath(commit string) string {
	return filepath.Join("languages", "sources", commit+".json")
}

func ReadLanguageSource(configDir, commit string) ([]byte, error) {
	if !validSourceCommit(commit) {
		return nil, fmt.Errorf("invalid English source commit %q", commit)
	}
	root, err := os.OpenRoot(configDir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	file, err := root.Open(LanguageSourcePath(commit))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return readLanguageData(file)
}

func languageHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}

func loadLanguageSource(ctx context.Context, client *http.Client, configDir, commit string) ([]byte, error) {
	if !validSourceCommit(commit) || configDir == "" {
		return nil, fmt.Errorf("invalid English source cache configuration")
	}
	if cached, err := ReadLanguageSource(configDir, commit); err == nil {
		if _, err := i18n.DecodeMessages(cached); err == nil {
			return cached, nil
		}
	}
	data, err := fetchLanguageData(ctx, client, "https://raw.githubusercontent.com/eugenioenko/ttt/"+commit+"/internal/i18n/en.json")
	if err != nil {
		return nil, err
	}
	if _, err := i18n.DecodeMessages(data); err != nil {
		return nil, fmt.Errorf("English source: %w", err)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(configDir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	for _, dir := range []string{"languages", filepath.Join("languages", "sources")} {
		if err := root.Mkdir(dir, 0755); err != nil && !os.IsExist(err) {
			return nil, err
		}
		info, err := root.Lstat(dir)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("English source directory must not be a symlink")
		}
	}
	stage := filepath.Join("languages", "sources", ".tmp-"+rand.Text())
	defer root.Remove(stage)
	if err := root.WriteFile(stage, data, 0644); err != nil {
		return nil, err
	}
	if err := root.Rename(stage, LanguageSourcePath(commit)); err != nil {
		return nil, err
	}
	return data, nil
}

func prepareLanguageSources(configDir string, manifest Manifest) error {
	for _, contribution := range manifest.Languages {
		if contribution.SourceCommit != "" {
			if _, err := loadLanguageSource(context.Background(), languageHTTPClient(), configDir, contribution.SourceCommit); err != nil {
				return err
			}
		}
	}
	return nil
}
