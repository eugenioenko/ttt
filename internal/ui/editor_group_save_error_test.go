package ui

import (
	"fmt"
	"os"
	"testing"

	"github.com/eugenioenko/ttt/internal/core/buffer"
	"github.com/eugenioenko/ttt/internal/i18n"
)

func TestSaveErrorTextTranslatesRecoveryAndReplacement(t *testing.T) {
	t.Cleanup(func() { i18n.SetCatalog(nil) })
	catalog, err := i18n.ParseCatalog([]byte(`{
		"editorGroup.saveRecoveryKept": "El contenido anterior está en {path}: {error}",
		"editorGroup.saveTargetReplaced": "{path} fue reemplazado al guardar"
	}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	i18n.SetCatalog(catalog)

	recovery := fmt.Errorf("write: %w", &buffer.RecoveryError{Path: "/d/.f.ttt-recovery-1", Err: os.ErrPermission})
	if got, want := saveErrorText(recovery), "El contenido anterior está en /d/.f.ttt-recovery-1: permission denied"; got != want {
		t.Errorf("recovery text = %q, want %q", got, want)
	}
	if got, want := saveErrorText(&buffer.ReplacedError{Path: "/d/f.txt"}), "/d/f.txt fue reemplazado al guardar"; got != want {
		t.Errorf("replaced text = %q, want %q", got, want)
	}
	if got := saveErrorText(os.ErrNotExist); got != os.ErrNotExist.Error() {
		t.Errorf("other errors keep their own text, got %q", got)
	}
}

func TestSaveErrorTextInEnglish(t *testing.T) {
	err := &buffer.RecoveryError{Path: "/d/.f.ttt-recovery-1", Err: os.ErrPermission}
	if got := saveErrorText(err); got != err.Error() {
		t.Errorf("English recovery text = %q, want %q", got, err.Error())
	}
	replaced := &buffer.ReplacedError{Path: "/d/f.txt"}
	if got := saveErrorText(replaced); got != replaced.Error() {
		t.Errorf("English replaced text = %q, want %q", got, replaced.Error())
	}
}
