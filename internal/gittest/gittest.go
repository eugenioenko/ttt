package gittest

import "os"

// DisableAutoMaintenance must run from TestMain in any package whose tests
// commit inside t.TempDir. Since git 2.47, commit detaches `git maintenance run
// --auto`, which writes .git/objects/maintenance.lock after the commit returns
// and races TempDir cleanup ("directory not empty").
func DisableAutoMaintenance() {
	if err := os.Setenv("GIT_CONFIG_COUNT", "1"); err != nil {
		panic(err)
	}
	if err := os.Setenv("GIT_CONFIG_KEY_0", "maintenance.auto"); err != nil {
		panic(err)
	}
	if err := os.Setenv("GIT_CONFIG_VALUE_0", "false"); err != nil {
		panic(err)
	}
}
