package ui

import (
	"testing"

	"github.com/eugenioenko/ttt/internal/i18n"
)

func TestPaletteHelpTopicsResolveSelectedLanguage(t *testing.T) {
	t.Cleanup(func() { i18n.SetCatalog(nil) })
	catalog, err := i18n.ParseCatalog([]byte(`{"help.workspaceMap":"Mapa del espacio de trabajo","help.understandFoldersTabsAndEditorGroups":"Carpetas y pestañas"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	i18n.SetCatalog(catalog)
	topics, _ := buildPaletteHelpItems(nil)
	if topics[0].Label != "Mapa del espacio de trabajo" || topics[0].Description != "Carpetas y pestañas" {
		t.Fatalf("topic copy was cached in English: %+v", topics[0])
	}
	if ok, _ := scorePaletteHelpItem("Carpetas", topics[0]); !ok {
		t.Fatal("translated description was not included in help search")
	}
	if topic, ok := paletteHelpTopicByID("workspace"); !ok || !paletteTopicMatchesCommand(topic, PaletteItem{ID: "file.new"}) {
		t.Fatal("translated topic lost its command routing")
	}
}
