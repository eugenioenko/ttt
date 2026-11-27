package e2e

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v3"
)

func TestAboutCreditsScrollToLastName(t *testing.T) {
	raw, err := os.ReadFile("../../internal/app/contributors.json")
	if err != nil {
		t.Fatal(err)
	}
	var credits struct {
		Contributors []string `json:"contributors"`
		Reporters    []string `json:"reporters"`
	}
	if err := json.Unmarshal(raw, &credits); err != nil {
		t.Fatal(err)
	}
	names := append(credits.Contributors, credits.Reporters...)
	first, last := names[0], names[len(names)-1]

	h := newTestHarness(t, 100, 40)
	defer h.stop()
	h.exec("about")
	h.assertContains("Thank you!")
	h.assertContains(first)
	h.assertNotContains(last)

	x, y := -1, -1
	for row, line := range strings.Split(h.screenText(), "\n") {
		if col := displayColumnOf(line, first); col >= 0 {
			x, y = col, row
			break
		}
	}
	for range len(names) {
		h.app.Root.HandleEvent(tcell.NewEventMouse(x, y, tcell.WheelDown, 0))
	}
	h.redraw()
	h.assertContains(last)
	h.assertContains("Thank you!")
}
