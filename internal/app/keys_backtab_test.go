package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/gdamore/tcell/v3"
)

func TestComboToTcellShiftTabMatchesLegacyBacktab(t *testing.T) {
	for _, s := range []string{"shift+tab", "ctrl+shift+tab"} {
		steps, err := config.ParseKeyString(s)
		if err != nil {
			t.Fatal(err)
		}
		key, mod, _ := comboToTcell(steps[0])
		ev := tcell.NewEventKey(tcell.KeyTab, "", mod|tcell.ModShift)
		if key != ev.Key() || mod != ev.Modifiers() {
			t.Errorf("%s: binding = (%v, %v), event = (%v, %v)", s, key, mod, ev.Key(), ev.Modifiers())
		}
	}
}
