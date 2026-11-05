package app

import (
	"reflect"
	"testing"
)

func TestOpenURLCommand(t *testing.T) {
	for _, raw := range []string{"", "file:///tmp/file", "javascript:alert(1)", "ftp://example.com", "https:relative", "https:///missing-host", "https://bad\nURL"} {
		if cmd := openURLCommand(raw, "windows"); cmd != nil {
			t.Errorf("accepted unsupported URL %q", raw)
		}
	}
	raw := "https://a.test/&calc.exe?q=hello&lang=en"
	for _, tt := range []struct {
		platform string
		args     []string
	}{
		{"windows", []string{"rundll32.exe", "url.dll,FileProtocolHandler", raw}},
		{"darwin", []string{"open", raw}},
		{"linux", []string{"xdg-open", raw}},
	} {
		cmd := openURLCommand(raw, tt.platform)
		if cmd == nil || !reflect.DeepEqual(cmd.Args, tt.args) {
			t.Errorf("%s command = %v, want %v", tt.platform, cmd, tt.args)
		}
	}
}
