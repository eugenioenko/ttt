package plugin

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v3"
	lua "github.com/yuin/gopher-lua"
)

type SidebarRegistration struct {
	ID     string
	Title  string
	Widget *PluginPanelWidget
}

type BottomRegistration struct {
	ID     string
	Title  string
	Widget *PluginPanelWidget
}

type Manager struct {
	plugins      []*Plugin
	registry     *Registry
	pluginsDir   string
	registryPath string
	extraDirs    []string

	SidebarPanels []SidebarRegistration
	BottomPanels  []BottomRegistration

	editorAPI   EditorAPI
	settingsAPI SettingsAPI
	networkAPI  NetworkAPI
	systemAPI   SystemAPI
	fsFactory   func(pluginDir string) FilesystemAPI
	logFactory  func(pluginName string) func(level, message string)
}

func NewManager(pluginsDir, registryPath string, extraDirs ...string) *Manager {
	return &Manager{pluginsDir: pluginsDir, registryPath: registryPath, extraDirs: extraDirs}
}

func (m *Manager) LoadAll() []*Plugin {
	os.MkdirAll(m.pluginsDir, 0755)

	regPath := m.registryPath
	reg, err := LoadRegistry(regPath)
	if err != nil {
		slog.Error("load plugin registry", "error", err)
		reg = &Registry{path: regPath}
	}
	m.registry = reg

	var needsApproval []*Plugin
	seen := map[string]bool{}

	dirs := append([]string{m.pluginsDir}, m.extraDirs...)
	for _, pluginDir := range dirs {
		entries, err := os.ReadDir(pluginDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			dir := filepath.Join(pluginDir, entry.Name())
			manifest, err := LoadManifest(dir)
			if err != nil {
				slog.Warn("skip plugin", "dir", entry.Name(), "error", err)
				continue
			}

			if seen[manifest.Name] {
				continue
			}
			seen[manifest.Name] = true

			p := &Plugin{
				Name:     manifest.Name,
				Dir:      dir,
				Manifest: manifest,
			}

			regEntry := m.registry.Find(manifest.Name)
			if regEntry == nil {
				needsApproval = append(needsApproval, p)
				continue
			}

			p.Repo = regEntry.Repo
			p.RepoPath = regEntry.Path

			if !regEntry.Enabled {
				continue
			}

			diff := DiffPermissions(regEntry.Permissions, manifest.Permissions)
			if !diff.IsEmpty() {
				regEntry.Enabled = false
				needsApproval = append(needsApproval, p)
				continue
			}

			p.Granted = regEntry.Permissions

			// Init reports compile failures through p.Log, so wire it first.
			if m.logFactory != nil {
				p.Log = m.logFactory(p.Name)
			}
			if err := p.Init(); err != nil {
				continue
			}

			m.plugins = append(m.plugins, p)
			m.collectRegistrations(p)
		}
	}

	return needsApproval
}

func (m *Manager) collectRegistrations(p *Plugin) {
	id := "plugin." + p.Name

	filtered := m.SidebarPanels[:0]
	for _, reg := range m.SidebarPanels {
		if reg.ID != id {
			filtered = append(filtered, reg)
		}
	}
	m.SidebarPanels = filtered

	filteredBottom := m.BottomPanels[:0]
	for _, reg := range m.BottomPanels {
		if reg.ID != id {
			filteredBottom = append(filteredBottom, reg)
		}
	}
	m.BottomPanels = filteredBottom

	if p.SidebarTitle != "" && p.RenderFunc != nil {
		m.SidebarPanels = append(m.SidebarPanels, SidebarRegistration{
			ID:     id,
			Title:  p.SidebarTitle,
			Widget: NewPluginPanelWidget(p, p.RenderFunc, p.EventFunc),
		})
	}
	if p.BottomTitle != "" && p.BottomRenderFunc != nil {
		m.BottomPanels = append(m.BottomPanels, BottomRegistration{
			ID:     id,
			Title:  p.BottomTitle,
			Widget: NewPluginPanelWidget(p, p.BottomRenderFunc, p.BottomEventFunc),
		})
	}
}

func (m *Manager) ApproveAndLoad(p *Plugin) error {
	p.Granted = p.Manifest.Permissions

	m.registry.AddOrUpdate(
		p.Manifest.Name,
		p.Manifest.DisplayName,
		p.Repo,
		p.RepoPath,
		p.Manifest.Version,
		p.Manifest.Permissions,
	)
	if err := m.registry.Save(); err != nil {
		slog.Error("save plugin registry", "error", err)
	}

	m.wireAPIs(p)

	if err := p.Init(); err != nil {
		return err
	}

	p.CallOnInstall()

	m.plugins = append(m.plugins, p)
	m.collectRegistrations(p)
	return nil
}

func (m *Manager) RegisterDebugPlugin(p *Plugin) {
	for i, existing := range m.plugins {
		if existing.Name == p.Name {
			existing.Destroy()
			m.plugins[i] = p
			m.collectRegistrations(p)
			return
		}
	}
	m.plugins = append(m.plugins, p)
	m.collectRegistrations(p)
}

func (m *Manager) SetEditorAPI(api EditorAPI) {
	m.editorAPI = api
	for _, p := range m.plugins {
		p.Editor = api
	}
}

func (m *Manager) SetFilesystemAPI(factory func(pluginDir string) FilesystemAPI) {
	m.fsFactory = factory
	for _, p := range m.plugins {
		p.Filesystem = factory(p.Dir)
	}
}

func (m *Manager) SetSystemAPI(api SystemAPI) {
	m.systemAPI = api
	for _, p := range m.plugins {
		p.System = api
	}
}

func (m *Manager) SetNetworkAPI(api NetworkAPI) {
	m.networkAPI = api
	for _, p := range m.plugins {
		p.Network = api
	}
}

func (m *Manager) SetSettingsAPI(api SettingsAPI) {
	m.settingsAPI = api
	for _, p := range m.plugins {
		p.Settings = api
	}
}

func (m *Manager) SetLogFactory(factory func(pluginName string) func(level, message string)) {
	m.logFactory = factory
	for _, p := range m.plugins {
		p.Log = factory(p.Name)
	}
}

func (m *Manager) wireAPIs(p *Plugin) {
	if m.editorAPI != nil {
		p.Editor = m.editorAPI
	}
	if m.settingsAPI != nil {
		p.Settings = m.settingsAPI
	}
	if m.networkAPI != nil {
		p.Network = m.networkAPI
	}
	if m.systemAPI != nil {
		p.System = m.systemAPI
	}
	if m.fsFactory != nil {
		p.Filesystem = m.fsFactory(p.Dir)
	}
	if m.logFactory != nil {
		p.Log = m.logFactory(p.Name)
	}
}

func (m *Manager) DispatchKeyEvent(ev *tcell.EventKey) bool {
	for _, p := range m.plugins {
		if p.State == nil || len(p.EventListeners["key.press"]) == 0 {
			continue
		}
		tbl := keyEventToLua(p.State, ev)
		if p.DispatchKeyEvent(tbl) {
			return true
		}
	}
	return false
}

func (m *Manager) DispatchEvent(name string, args ...interface{}) {
	for _, p := range m.plugins {
		if p.State == nil || len(p.EventListeners[name]) == 0 {
			continue
		}
		var largs []lua.LValue
		for _, a := range args {
			switch v := a.(type) {
			case string:
				largs = append(largs, lua.LString(v))
			case int:
				largs = append(largs, lua.LNumber(v))
			case bool:
				largs = append(largs, lua.LBool(v))
			}
		}
		p.DispatchEvent(name, largs...)
	}
}

func (m *Manager) Install(repoURL, repoPath string) (*Plugin, error) {
	if !strings.HasPrefix(repoURL, "https://") {
		return nil, fmt.Errorf("only https:// URLs are allowed for plugin install")
	}

	if repoPath != "" {
		return m.installFromSubdir(repoURL, repoPath)
	}

	name := filepath.Base(repoURL)
	name = strings.TrimSuffix(name, ".git")
	if name == "" || name == "." {
		return nil, fmt.Errorf("invalid repository URL")
	}

	targetDir := filepath.Join(m.pluginsDir, name)
	if _, err := os.Stat(targetDir); err == nil {
		return nil, fmt.Errorf("plugin %q already exists", name)
	}

	cmd := exec.Command("git", "clone", repoURL, targetDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git clone failed: %s: %s", err, strings.TrimSpace(string(out)))
	}

	manifest, err := LoadManifest(targetDir)
	if err != nil {
		os.RemoveAll(targetDir)
		return nil, fmt.Errorf("invalid plugin: %w", err)
	}

	return &Plugin{
		Name:     manifest.Name,
		Dir:      targetDir,
		Repo:     repoURL,
		Manifest: manifest,
	}, nil
}

func (m *Manager) installFromSubdir(repoURL, repoPath string) (*Plugin, error) {
	tmpDir, err := os.MkdirTemp("", "ttt-plugin-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("git", "clone", "--depth", "1", repoURL, tmpDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git clone failed: %s: %s", err, strings.TrimSpace(string(out)))
	}

	srcDir := filepath.Join(tmpDir, repoPath)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("path %q not found in repository", repoPath)
	}

	manifest, err := LoadManifest(srcDir)
	if err != nil {
		return nil, fmt.Errorf("invalid plugin: %w", err)
	}

	targetDir := filepath.Join(m.pluginsDir, manifest.Name)
	if _, err := os.Stat(targetDir); err == nil {
		return nil, fmt.Errorf("plugin %q already exists", manifest.Name)
	}

	if err := copyDir(srcDir, targetDir); err != nil {
		os.RemoveAll(targetDir)
		return nil, fmt.Errorf("copy plugin: %w", err)
	}

	return &Plugin{
		Name:     manifest.Name,
		Dir:      targetDir,
		Repo:     repoURL,
		RepoPath: repoPath,
		Manifest: manifest,
	}, nil
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) Uninstall(name string) error {
	for i, p := range m.plugins {
		if p.Name == name {
			if p.UninstallFunc != nil {
				if err := p.CallLuaFunc(p.UninstallFunc); err != nil {
					slog.Error("plugin on_uninstall", "plugin", name, "error", err)
				}
			}
			p.Destroy()
			m.plugins = append(m.plugins[:i], m.plugins[i+1:]...)
			break
		}
	}

	dir := filepath.Join(m.pluginsDir, name)
	if err := os.RemoveAll(dir); err != nil {
		slog.Error("remove plugin directory", "error", err)
	}

	m.registry.Remove(name)
	if err := m.registry.Save(); err != nil {
		slog.Error("save plugin registry", "error", err)
	}

	for i, reg := range m.SidebarPanels {
		if reg.ID == "plugin."+name {
			m.SidebarPanels = append(m.SidebarPanels[:i], m.SidebarPanels[i+1:]...)
			break
		}
	}
	for i, reg := range m.BottomPanels {
		if reg.ID == "plugin."+name {
			m.BottomPanels = append(m.BottomPanels[:i], m.BottomPanels[i+1:]...)
			break
		}
	}

	return nil
}

// DenyPlugin permanently stops a pending plugin from prompting for approval.
// A plugin ttt freshly cloned into pluginsDir (not yet in the registry) is
// deleted outright so it leaves no trace. Anything else — a workspace-local
// plugin, or one already tracked in the registry — is recorded as disabled so
// LoadAll skips it on the next launch instead of re-prompting forever (#358).
func (m *Manager) DenyPlugin(p *Plugin) error {
	if p == nil {
		return nil
	}
	var entry *RegistryEntry
	if m.registry != nil {
		entry = m.registry.Find(p.Name)
	}
	if entry == nil && withinDir(m.pluginsDir, p.Dir) {
		if err := os.RemoveAll(p.Dir); err != nil {
			return fmt.Errorf("remove denied plugin: %w", err)
		}
		return nil
	}
	if m.registry == nil {
		return nil
	}
	m.registry.AddOrUpdate(p.Manifest.Name, p.Manifest.DisplayName, p.Repo, p.RepoPath, p.Manifest.Version, p.Manifest.Permissions)
	m.registry.SetEnabled(p.Manifest.Name, false)
	return m.registry.Save()
}

// withinDir reports whether child lives inside parent (not equal to it, not an
// escape via ..). Used to confine deny-deletion to ttt's own plugins dir.
func withinDir(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (m *Manager) Update(name string) (*Plugin, bool, error) {
	dir := filepath.Join(m.pluginsDir, name)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, false, fmt.Errorf("plugin %q not found", name)
	}

	regEntry := m.registry.Find(name)

	if regEntry != nil && regEntry.Path != "" {
		return m.updateFromSubdir(name, dir, regEntry)
	}

	cmd := exec.Command("git", "-C", dir, "pull")
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, false, fmt.Errorf("git pull failed: %s: %s", err, strings.TrimSpace(string(out)))
	}

	newManifest, err := LoadManifest(dir)
	if err != nil {
		return nil, false, fmt.Errorf("invalid manifest after update: %w", err)
	}

	if regEntry == nil {
		p := &Plugin{Name: newManifest.Name, Dir: dir, Manifest: newManifest}
		return p, true, nil
	}

	diff := DiffPermissions(regEntry.Permissions, newManifest.Permissions)
	if !diff.IsEmpty() {
		for i, p := range m.plugins {
			if p.Name == name {
				p.Destroy()
				m.plugins = append(m.plugins[:i], m.plugins[i+1:]...)
				break
			}
		}
		regEntry.Enabled = false
		m.registry.Save()
		p := &Plugin{Name: newManifest.Name, Dir: dir, Manifest: newManifest}
		return p, true, nil
	}

	for _, p := range m.plugins {
		if p.Name == name {
			p.Destroy()
			p.Manifest = newManifest
			p.Granted = regEntry.Permissions
			if err := p.Init(); err != nil {
				return nil, false, err
			}
			m.collectRegistrations(p)
			regEntry.Version = newManifest.Version
			m.registry.Save()
			return p, false, nil
		}
	}

	return nil, false, nil
}

func (m *Manager) updateFromSubdir(name, dir string, regEntry *RegistryEntry) (*Plugin, bool, error) {
	if regEntry.Repo == "" {
		return nil, false, fmt.Errorf("plugin %q has no repository URL", name)
	}

	tmpDir, err := os.MkdirTemp("", "ttt-plugin-update-*")
	if err != nil {
		return nil, false, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("git", "clone", "--depth", "1", regEntry.Repo, tmpDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, false, fmt.Errorf("git clone failed: %s: %s", err, strings.TrimSpace(string(out)))
	}

	srcDir := filepath.Join(tmpDir, regEntry.Path)
	newManifest, err := LoadManifest(srcDir)
	if err != nil {
		return nil, false, fmt.Errorf("invalid manifest after update: %w", err)
	}

	os.RemoveAll(dir)
	if err := copyDir(srcDir, dir); err != nil {
		return nil, false, fmt.Errorf("copy plugin: %w", err)
	}

	diff := DiffPermissions(regEntry.Permissions, newManifest.Permissions)
	if !diff.IsEmpty() {
		for i, p := range m.plugins {
			if p.Name == name {
				p.Destroy()
				m.plugins = append(m.plugins[:i], m.plugins[i+1:]...)
				break
			}
		}
		regEntry.Enabled = false
		m.registry.Save()
		p := &Plugin{Name: newManifest.Name, Dir: dir, Repo: regEntry.Repo, RepoPath: regEntry.Path, Manifest: newManifest}
		return p, true, nil
	}

	for _, p := range m.plugins {
		if p.Name == name {
			p.Destroy()
			p.Manifest = newManifest
			p.Granted = regEntry.Permissions
			if err := p.Init(); err != nil {
				return nil, false, err
			}
			m.collectRegistrations(p)
			regEntry.Version = newManifest.Version
			m.registry.Save()
			return p, false, nil
		}
	}

	return nil, false, nil
}

func (m *Manager) SetEnabled(name string, enabled bool) (*Plugin, error) {
	m.registry.SetEnabled(name, enabled)
	if err := m.registry.Save(); err != nil {
		return nil, err
	}

	if !enabled {
		for i, p := range m.plugins {
			if p.Name == name {
				p.Destroy()
				m.plugins = append(m.plugins[:i], m.plugins[i+1:]...)
				break
			}
		}
		return nil, nil
	}

	dir := filepath.Join(m.pluginsDir, name)
	manifest, err := LoadManifest(dir)
	if err != nil {
		return nil, err
	}

	regEntry := m.registry.Find(name)
	if regEntry == nil {
		return nil, fmt.Errorf("plugin %q not in registry", name)
	}

	p := &Plugin{
		Name:     manifest.Name,
		Dir:      dir,
		Manifest: manifest,
		Granted:  regEntry.Permissions,
	}
	if err := p.Init(); err != nil {
		return nil, err
	}

	m.plugins = append(m.plugins, p)
	m.collectRegistrations(p)
	return p, nil
}

func (m *Manager) Reload(name string) (*Plugin, error) {
	var old *Plugin
	var idx int
	for i, p := range m.plugins {
		if p.Name == name {
			old = p
			idx = i
			break
		}
	}
	if old == nil {
		return nil, fmt.Errorf("plugin %q not loaded", name)
	}

	granted := old.Granted
	repo := old.Repo
	dir := old.Dir
	logFn := old.Log

	old.Destroy()

	manifest, err := LoadManifest(dir)
	if err != nil {
		m.plugins = append(m.plugins[:idx], m.plugins[idx+1:]...)
		return nil, fmt.Errorf("reload manifest: %w", err)
	}

	p := &Plugin{
		Name:     manifest.Name,
		Dir:      dir,
		Repo:     repo,
		Manifest: manifest,
		Granted:  granted,
		Log:      logFn,
	}

	if err := p.Init(); err != nil {
		m.plugins = append(m.plugins[:idx], m.plugins[idx+1:]...)
		return nil, fmt.Errorf("reload init: %w", err)
	}

	m.plugins[idx] = p

	for i := len(m.SidebarPanels) - 1; i >= 0; i-- {
		if m.SidebarPanels[i].ID == "plugin."+name {
			m.SidebarPanels = append(m.SidebarPanels[:i], m.SidebarPanels[i+1:]...)
		}
	}
	for i := len(m.BottomPanels) - 1; i >= 0; i-- {
		if m.BottomPanels[i].ID == "plugin."+name {
			m.BottomPanels = append(m.BottomPanels[:i], m.BottomPanels[i+1:]...)
		}
	}
	m.collectRegistrations(p)

	return p, nil
}

func (m *Manager) PluginsDir() string {
	return m.pluginsDir
}

func (m *Manager) Registry() *Registry {
	return m.registry
}

func (m *Manager) InstalledPluginNames() []string {
	if m.registry == nil {
		return nil
	}
	var names []string
	for _, e := range m.registry.Entries {
		names = append(names, e.Name)
	}
	return names
}

func (m *Manager) FindPlugin(name string) *Plugin {
	for _, p := range m.plugins {
		if p.Name == name {
			return p
		}
	}
	return nil
}

func (m *Manager) Shutdown() {
	for _, p := range m.plugins {
		p.Destroy()
	}
}

func (m *Manager) Plugins() []*Plugin {
	return m.plugins
}

func (m *Manager) PluginCount() int {
	return len(m.plugins)
}
