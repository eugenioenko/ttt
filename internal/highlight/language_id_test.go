package highlight

import "testing"

func TestDetectLanguageID(t *testing.T) {
	for _, tc := range []struct{ path, first, want string }{
		{"main.ts", "", "typescript"}, {"README.md", "", "markdown"},
		{"main.go", "", "go"}, {"main.tsx", "", "typescriptreact"},
		{"main.jsx", "", "javascriptreact"}, {"README.md.bak", "", "markdown"},
		{"script", "#!/usr/bin/env python3", "python"}, {"unknown.zzzz", "", ""},
	} {
		if got := DetectLanguageID(tc.path, []string{tc.first}); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.path, got, tc.want)
		}
	}
	setExternal(t, ExternalGrammar{Path: writeGrammar(t, "custom.json", fooGrammar), Language: "markdown", FileTypes: []string{"foo"}})
	if got := DetectLanguageID("a.foo", nil); got != "markdown" {
		t.Fatalf("external language = %q", got)
	}
}
