package i18n

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestCatalogFallbackAndFreshness(t *testing.T) {
	t.Cleanup(func() { SetCatalog(nil) })
	hash := sha256.Sum256([]byte(english["general.save"]))
	hashes, _ := json.Marshal(map[string]string{"general.save": hex.EncodeToString(hash[:]), "general.cancel": "outdated"})
	catalog, err := ParseCatalog([]byte(`{"general.save":"保存","general.cancel":"取消","future.key":"future"}`), hashes)
	if err != nil {
		t.Fatal(err)
	}
	SetCatalog(catalog)
	if Text("general.save") != "保存" || Text("general.cancel") != "Cancel" || Text("general.close") != "Close" {
		t.Fatal("missing or outdated messages did not fall back to English")
	}
	SetCatalog(nil)
	if Text("general.save") != "Save" {
		t.Fatal("English reset failed")
	}
}

func TestCatalogRejectsBrokenMessagesAndRetainsValidOnes(t *testing.T) {
	t.Cleanup(func() { SetCatalog(nil) })
	catalog, err := ParseCatalog([]byte(`{"status.failedToOpenTerminal":"错误: {other}","general.save":"Guardar","general.cancel":"","general.close":"{broken"}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	SetCatalog(catalog)
	if Text("general.save") != "Guardar" || Text("general.cancel") != "Cancel" || Text("general.close") != "Close" {
		t.Fatal("per-message fallback failed")
	}
	if Format("status.failedToOpenTerminal", Args{"error": "x"}) != "Failed to open terminal: x" {
		t.Fatal("invalid placeholders did not fall back")
	}
}

func TestDecodeCatalogRejectsMalformedObjects(t *testing.T) {
	for _, data := range []string{`null`, `[]`, `{"a":null}`, `{"a":12}`, `{"a":"one","a":"two"}`, `{"a":"one"} {}`, `{"a":"one"`} {
		if _, err := DecodeMessages([]byte(data)); err == nil {
			t.Errorf("accepted %s", data)
		}
	}
}

func TestDeclaredEmptySourceHashesAreRejected(t *testing.T) {
	messages := []byte(`{"general.save":"Guardar"}`)
	if _, err := ParseCatalog(messages, nil); err != nil {
		t.Fatal("omitting source hashes must remain supported", err)
	}
	if _, err := ParseCatalog(messages, []byte{}); err == nil {
		t.Fatal("empty declared hashes bypassed freshness validation")
	}
}

func TestNormalizeLocale(t *testing.T) {
	for input, want := range map[string]string{"": "en", "es": "es", "zh-cn": "zh-CN", "pt-BR": "pt-BR"} {
		if got, err := NormalizeLocale(input); err != nil || got != want {
			t.Errorf("NormalizeLocale(%q) = %q, %v", input, got, err)
		}
	}
	for _, input := range []string{"../es", "und", "not a locale", "es/extra"} {
		if _, err := NormalizeLocale(input); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
}
