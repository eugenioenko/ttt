package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/i18n"
)

func TestPaletteHelpTopicsInitializeInSelectedLanguage(t *testing.T) {
	t.Cleanup(func() { i18n.SetCatalog(nil); InitText() })
	catalog, err := i18n.ParseCatalog([]byte(`{"help.workspaceMap":"Mapa del espacio de trabajo","help.understandFoldersTabsAndEditorGroups":"Carpetas y pestañas"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	i18n.SetCatalog(catalog)
	InitText()
	topics, _ := buildPaletteHelpItems(nil)
	if topics[0].Label != "Mapa del espacio de trabajo" || topics[0].Description != "Carpetas y pestañas" {
		t.Fatalf("topic copy was cached in English: %+v", topics[0])
	}
}
