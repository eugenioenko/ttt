package app

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/config"
)

func TestSplitGroupOwnsAndReleasesFoldTasks(t *testing.T) {
	config.OverrideConfigDir = t.TempDir()
	t.Cleanup(func() { config.OverrideConfigDir = "" })
	a := buildTestApp(t, config.DefaultSettings())
	t.Cleanup(a.CloseTasks)
	count := len(a.tasks.tasks)
	a.SplitEditor()
	if a.EditorGroup.FoldTask == nil || a.EditorGroup.FoldDelay == nil || len(a.tasks.tasks) != count+2 {
		t.Fatal("split group did not register its own fold computation and delay tasks")
	}
	a.CloseEditorSplit()
	if len(a.tasks.tasks) != count {
		t.Fatal("closing the group leaked its fold tasks")
	}
}
