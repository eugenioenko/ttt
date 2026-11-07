package plugin

import (
	lua "github.com/yuin/gopher-lua"
)

func newTestPluginBase(perms PermissionSet) (*Plugin, func()) {
	p := &Plugin{
		Name:    "test",
		Granted: perms,
	}
	p.EventListeners = make(map[string][]*lua.LFunction)
	L := NewSandbox()
	p.State = L
	setupTTTModule(L, p)
	setupEditorModule(L, p)
	setupDiagnosticsModule(L, p)
	setupDecorationsModule(L, p)
	setupFsModule(L, p)
	setupSystemModule(L, p)
	setupTerminalModule(L, p)
	setupNetModule(L, p)
	setupRPCModule(L, p)
	setupAutomationModule(L, p)
	setupEventsModule(L, p)
	return p, func() { L.Close() }
}
