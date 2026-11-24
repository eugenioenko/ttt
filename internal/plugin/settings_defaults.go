package plugin

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
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

func (m *Manager) callInstallHook(p *Plugin) error {
	added := map[string]string{}
	p.installDefault = func(key string, value any) error {
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
		entry := m.registry.Find(p.Name)
		if entry == nil {
			return fmt.Errorf("plugin %q has no registry entry", p.Name)
		}
		if entry.InstalledDefaults == nil {
			entry.InstalledDefaults = map[string]string{}
		}
		entry.InstalledDefaults[key] = digest
		// Record ownership before writing so an interrupted install is still removable.
		if err = m.registry.Save(); err != nil {
			return err
		}
		if err = api.Set(key, value); err != nil {
			return err
		}
		added[key] = digest
		return nil
	}
	defer func() { p.installDefault = nil }()
	if err := p.CallOnInstall(); err != nil {
		return errors.Join(err, m.removeDefaults(added))
	}
	return nil
}

func (m *Manager) removeDefaults(defaults map[string]string) error {
	for key, written := range defaults {
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
