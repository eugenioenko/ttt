package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
	"github.com/gdamore/tcell/v3"
)

func TestComboToTcellShiftTabMatchesLegacyBacktab(t *testing.T) {
	// Expected mods hardcoded so a dropped modifier in comboToTcell fails the test.
	cases := []struct {
		combo string
		mod   tcell.ModMask
	}{
		{"shift+tab", tcell.ModShift},
		{"ctrl+shift+tab", tcell.ModCtrl | tcell.ModShift},
	}
	for _, c := range cases {
		steps, err := config.ParseKeyString(c.combo)
		if err != nil {
			t.Fatal(err)
		}
		key, mod, _ := comboToTcell(steps[0])
		ev := tcell.NewEventKey(tcell.KeyTab, "", c.mod)
		if key != ev.Key() || mod != ev.Modifiers() {
			t.Errorf("%s: binding = (%v, %v), event = (%v, %v)", c.combo, key, mod, ev.Key(), ev.Modifiers())
		}
	}
}
