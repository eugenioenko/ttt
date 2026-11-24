package providers

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/dap"
	"github.com/eugenioenko/ttt/internal/dap/providers/debugpy"
	"github.com/eugenioenko/ttt/internal/dap/providers/delve"
	"github.com/eugenioenko/ttt/internal/dap/providers/jsdebug"
)

type MissingAdapter struct{ Type, Plugin string }

func (e MissingAdapter) Error() string { return fmt.Sprintf("unconfigured debug adapter %q", e.Type) }

var registryPlugins = map[string]string{
	"go": "dap-go", "node": "dap-js-debug", "pwa-node": "dap-js-debug",
	"debugpy": "dap-debugpy", "lldb": "dap-codelldb", "gdb": "dap-gdb",
}

func For(t dap.Target, adapters map[string]dap.Adapter) (dap.Provider, error) {
	a, ok := adapters[t.Type]
	if !ok {
		return nil, MissingAdapter{Type: t.Type, Plugin: registryPlugins[t.Type]}
	}
	switch a.Provider {
	case "", "generic":
		return a, nil
	case "delve":
		return delve.Provider{Adapter: a}, nil
	case "jsdebug":
		return jsdebug.Provider{Adapter: a}, nil
	case "debugpy":
		return debugpy.Provider{Adapter: a}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q for adapter %q", a.Provider, t.Type)
	}
}
