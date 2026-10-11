package plugin

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// fileHandlerMagicMax bounds magic to the head bytes the editor group already sniffs.
const fileHandlerMagicMax = 512

type FileHandler struct {
	ID         string
	Title      string
	Extensions []string
	Magic      []byte
	Open       *lua.LFunction
}

func parseFileHandlers(L *lua.LState, list *lua.LTable) []FileHandler {
	var handlers []FileHandler
	list.ForEach(func(_ lua.LValue, v lua.LValue) {
		entry, ok := v.(*lua.LTable)
		if !ok {
			L.RaiseError("file_handlers entries must be tables")
		}
		id, ok := L.GetField(entry, "id").(lua.LString)
		if !ok || id == "" {
			L.RaiseError("file handler requires a string id")
		}
		open, ok := L.GetField(entry, "open").(*lua.LFunction)
		if !ok {
			L.RaiseError("file handler %q requires an open function", string(id))
		}
		h := FileHandler{ID: string(id), Title: string(id), Open: open}
		if title, ok := L.GetField(entry, "title").(lua.LString); ok && title != "" {
			h.Title = string(title)
		}
		if exts, ok := L.GetField(entry, "extensions").(*lua.LTable); ok {
			exts.ForEach(func(_ lua.LValue, ev lua.LValue) {
				ext := strings.ToLower(strings.TrimPrefix(lua.LVAsString(ev), "."))
				if ext != "" {
					h.Extensions = append(h.Extensions, ext)
				}
			})
		}
		if magic, ok := L.GetField(entry, "magic").(lua.LString); ok && magic != "" {
			if len(magic) > fileHandlerMagicMax {
				L.RaiseError("file handler %q magic exceeds %d bytes", h.ID, fileHandlerMagicMax)
			}
			h.Magic = []byte(magic)
		}
		if len(h.Extensions) == 0 && len(h.Magic) == 0 {
			L.RaiseError("file handler %q requires extensions or magic", h.ID)
		}
		handlers = append(handlers, h)
	})
	return handlers
}

func (h *FileHandler) matchesMagic(head []byte) bool {
	return len(h.Magic) > 0 && bytes.HasPrefix(head, h.Magic)
}

func (h *FileHandler) matchesExtension(path string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if ext == "" {
		return false
	}
	for _, e := range h.Extensions {
		if e == ext {
			return true
		}
	}
	return false
}

func (m *Manager) MatchFileHandler(path string, head []byte) (*Plugin, *FileHandler) {
	var extPlugin *Plugin
	var extHandler *FileHandler
	for _, p := range m.plugins {
		if p.State == nil || !p.Enabled {
			continue
		}
		for i := range p.FileHandlers {
			h := &p.FileHandlers[i]
			if h.matchesMagic(head) {
				return p, h
			}
			if extHandler == nil && h.matchesExtension(path) {
				extPlugin, extHandler = p, h
			}
		}
	}
	return extPlugin, extHandler
}

var errHandlerNoRender = errors.New("open did not set tab.render")

func (p *Plugin) OpenFileHandler(h *FileHandler, path string) (*PluginPanelWidget, error) {
	if p.State == nil {
		return nil, errors.New("plugin is not loaded")
	}
	tab := p.State.NewTable()
	if err := p.CallLuaFunc(h.Open, lua.LString(path), tab); err != nil {
		return nil, err
	}
	render, ok := p.State.GetField(tab, "render").(*lua.LFunction)
	if !ok {
		p.logError("file handler "+h.ID, errHandlerNoRender)
		return nil, errHandlerNoRender
	}
	event, _ := p.State.GetField(tab, "on_event").(*lua.LFunction)
	return NewPluginPanelWidget(p, render, event), nil
}
