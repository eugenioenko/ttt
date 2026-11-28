package app

import (
	"fmt"
	"strings"

	"github.com/eugenioenko/ttt/internal/config"

	"github.com/gdamore/tcell/v3"
)

// pluginInput is input a plugin synthesized. The event loop drops it while a
// plugin approval dialog is open, so no plugin can grant permissions.
type pluginInput struct {
	Events    []tcell.Event
	Lifecycle *execRequestLifecycle
	Allowed   func() bool
}

// postPluginInput sends events as one interrupt: PostEvent falls back to a
// goroutine when the queue is full, which would reorder separate posts.
func (a *App) postPluginInput(allowed func() bool, events ...tcell.Event) {
	_ = a.Screen.PostEvent(tcell.NewEventInterrupt(&pluginInput{Events: events, Allowed: allowed}))
}

func (a *App) simulatePluginKeys(combo string, allowed func() bool) error {
	steps, err := config.ParseKeyString(combo)
	if err != nil {
		return err
	}
	events := make([]tcell.Event, 0, len(steps))
	for _, step := range steps {
		key, mod, ch := comboToTcell(step)
		events = append(events, tcell.NewEventKey(key, keyEventStr(ch), mod))
	}
	a.postPluginInput(allowed, events...)
	return nil
}

const maxPluginTypeBytes = 64 << 10

func pluginTextEvents(text string) ([]tcell.Event, error) {
	if len(text) > maxPluginTypeBytes {
		return nil, fmt.Errorf("type input exceeds %d bytes; use automation paste for bulk text", maxPluginTypeBytes)
	}
	var events []tcell.Event
	for _, r := range strings.ReplaceAll(text, "\r\n", "\n") {
		if r == '\n' {
			events = append(events, tcell.NewEventKey(tcell.KeyEnter, "", tcell.ModNone))
		} else {
			events = append(events, tcell.NewEventKey(tcell.KeyRune, string(r), tcell.ModNone))
		}
	}
	return events, nil
}

func (a *App) simulatePluginText(text string, allowed func() bool) error {
	events, err := pluginTextEvents(text)
	if err != nil {
		return err
	}
	if len(events) > 0 {
		a.postPluginInput(allowed, events...)
	}
	return nil
}

func (a *App) acceptsPluginInput() bool {
	return len(a.PendingPluginApprovals) == 0
}
