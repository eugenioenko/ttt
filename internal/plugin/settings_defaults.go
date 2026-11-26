package plugin

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

func settingDigest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func (m *Manager) savedSettings() (SettingsAPI, error) {
	if api, ok := m.settingsAPI.(interface{ SavedSettings() (SettingsAPI, error) }); ok {
		return api.SavedSettings()
	}
	if m.settingsAPI == nil {
		return nil, errors.New("settings API unavailable")
	}
	return m.settingsAPI, nil
}

func (m *Manager) callInstallHook(p *Plugin) (error, error) {
	added := map[string]string{}
	p.installDefault = func(key string, value any) error {
		return m.installSettingDefault(p.Name, key, value, added)
	}
	defer func() { p.installDefault = nil }()
	if err := p.CallOnInstall(); err != nil {
		return err, m.removeDefaults(added)
	}
	return nil, nil
}

func (m *Manager) installSettingDefault(name, key string, value any, added map[string]string) error {
	api, err := m.savedSettings()
	if err != nil {
		return err
	}
	if _, exists := api.Get(key); exists {
		return nil
	}
	digest, err := settingDigest(value)
	if err != nil {
		return err
	}
	claims := map[string]string{key: digest}
	emptyObject := fmt.Sprintf("%x", sha256.Sum256([]byte("{}")))
	parts := strings.Split(key, ".")
	for i := 1; i < len(parts); i++ {
		parent := strings.Join(parts[:i], ".")
		if current, exists := api.Get(parent); exists {
			if _, object := current.(map[string]any); !object {
				return nil
			}
		} else {
			claims[parent] = emptyObject
		}
	}
	entry := m.registry.Find(name)
	if entry == nil {
		return fmt.Errorf("plugin %q has no registry entry", name)
	}
	previous := entry.InstalledDefaults
	entry.InstalledDefaults = make(map[string]string, len(previous)+len(claims))
	maps.Copy(entry.InstalledDefaults, previous)
	maps.Copy(entry.InstalledDefaults, claims)
	// Write ownership first so an interrupted install can still be cleaned up.
	if err := m.registry.Save(); err != nil {
		entry.InstalledDefaults = previous
		return err
	}
	if err := api.Set(key, value); err != nil {
		entry.InstalledDefaults = previous
		return errors.Join(err, m.registry.Save())
	}
	maps.Copy(added, claims)
	return nil
}

func (m *Manager) removeDefaults(defaults map[string]string) error {
	keys := slices.Sorted(maps.Keys(defaults))
	slices.SortStableFunc(keys, func(a, b string) int { return strings.Count(b, ".") - strings.Count(a, ".") })
	for _, key := range keys {
		written := defaults[key]
		api, err := m.savedSettings()
		if err != nil {
			return err
		}
		current, exists := api.Get(key)
		if !exists {
			continue
		}
		digest, err := settingDigest(current)
		if err != nil {
			return err
		}
		if digest == written {
			if err = api.Set(key, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
