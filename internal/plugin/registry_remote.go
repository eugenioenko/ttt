package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const DefaultRegistryURL = "https://raw.githubusercontent.com/eugenioenko/ttt/main/community-plugins.json"

type RemoteRegistryEntry struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName,omitempty"`
	Repo        string   `json:"repo"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Version     string   `json:"version,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Path        string   `json:"path,omitempty"`
	Featured    bool     `json:"featured,omitempty"`
}

// Title is the human-facing name shown in the registry UI: DisplayName when the
// entry provides one, otherwise the unique Name identifier.
func (e RemoteRegistryEntry) Title() string {
	if e.DisplayName != "" {
		return e.DisplayName
	}
	return e.Name
}

func FetchRemoteRegistry(url string) ([]RemoteRegistryEntry, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read registry: %w", err)
	}

	var entries []RemoteRegistryEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}

	return entries, nil
}
