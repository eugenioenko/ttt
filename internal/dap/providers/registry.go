package providers

import (
	"fmt"

	"github.com/eugenioenko/ttt/internal/dap"
	"github.com/eugenioenko/ttt/internal/dap/providers/debugpy"
	"github.com/eugenioenko/ttt/internal/dap/providers/delve"
	"github.com/eugenioenko/ttt/internal/dap/providers/jsdebug"
)

var builtins = map[string]dap.Provider{
	"go":       delve.Provider{},
	"node":     jsdebug.Provider{},
	"pwa-node": jsdebug.Provider{},
	"debugpy":  debugpy.Provider{},
}

func For(t dap.Target) (dap.Provider, error) {
	p, ok := builtins[t.Type]
	if !ok {
		return nil, fmt.Errorf("%s: unsupported debug adapter %q", t.Name, t.Type)
	}
	return p, nil
}
