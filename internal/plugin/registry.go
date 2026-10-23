package plugin

import (
	"encoding/json"
	"errors"
	"os"
)

// grantsVersion is recorded with each approval. From localAccessGrants on,
// network.http reaches local services too, through true or through a listed
// hostname that resolves to one, so a network.http granted before that has
// to be asked for again.
const (
	localAccessGrants = 2
	grantsVersion     = localAccessGrants
)

type RegistryEntry struct {
	Name          string        `json:"name"`
	DisplayName   string        `json:"displayName,omitempty"`
	Repo          string        `json:"repo,omitempty"`
	Path          string        `json:"path,omitempty"`
	Version       string        `json:"version"`
	Enabled       bool          `json:"enabled"`
	Permissions   PermissionSet `json:"permissions"`
	GrantsVersion int           `json:"grantsVersion,omitempty"`
}

// Title is the human-facing name: DisplayName when set, else the unique Name.
func (e RegistryEntry) Title() string {
	if e.DisplayName != "" {
		return e.DisplayName
	}
	return e.Name
}

type Registry struct {
	Entries  []RegistryEntry
	path     string
	saveHook func() error
}

func LoadRegistry(path string) (*Registry, error) {
	r := &Registry{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return r, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, &r.Entries); err != nil {
		return nil, err
	}
	for i := range r.Entries {
		e := &r.Entries[i]
		if e.GrantsVersion < localAccessGrants {
			e.Permissions.NetworkHTTP = NetworkHTTP{}
		}
		e.GrantsVersion = grantsVersion
	}
	return r, nil
}

func (r *Registry) Save() error {
	if r.saveHook != nil {
		if err := r.saveHook(); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(r.Entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0644)
}

func (r *Registry) Find(name string) *RegistryEntry {
	for i := range r.Entries {
		if r.Entries[i].Name == name {
			return &r.Entries[i]
		}
	}
	return nil
}

func (r *Registry) UpdatePermissions(name string, perms PermissionSet) {
	entry := r.Find(name)
	if entry != nil {
		entry.Permissions = perms
	}
}

func (r *Registry) SetEnabled(name string, enabled bool) {
	entry := r.Find(name)
	if entry != nil {
		entry.Enabled = enabled
	}
}

func (r *Registry) Remove(name string) {
	for i := range r.Entries {
		if r.Entries[i].Name == name {
			r.Entries = append(r.Entries[:i], r.Entries[i+1:]...)
			return
		}
	}
}

func (r *Registry) AddOrUpdate(name, displayName, repo, path, version string, perms PermissionSet) {
	entry := r.Find(name)
	if entry != nil {
		entry.DisplayName = displayName
		entry.Repo = repo
		entry.Path = path
		entry.Version = version
		entry.Permissions = perms
		entry.Enabled = true
		entry.GrantsVersion = grantsVersion
		return
	}
	r.Entries = append(r.Entries, RegistryEntry{
		Name:          name,
		DisplayName:   displayName,
		Repo:          repo,
		Path:          path,
		Version:       version,
		Enabled:       true,
		Permissions:   perms,
		GrantsVersion: grantsVersion,
	})
}
