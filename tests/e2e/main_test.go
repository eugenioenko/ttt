package e2e

import (
	"os"
	"testing"

	"github.com/eugenioenko/ttt/internal/gittest"
)

func TestMain(m *testing.M) {
	gittest.DisableAutoMaintenance()
	os.Exit(m.Run())
}
