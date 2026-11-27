package ui

import (
	"os"
	"testing"

	"github.com/eugenioenko/ttt/internal/gittest"
)

func TestMain(m *testing.M) {
	gittest.DisableAutoMaintenance()
	InitText()
	os.Exit(m.Run())
}
