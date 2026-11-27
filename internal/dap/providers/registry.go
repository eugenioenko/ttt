package providers

import (
	"github.com/eugenioenko/ttt/internal/dap"
	"github.com/eugenioenko/ttt/internal/dap/providers/debugpy"
	"github.com/eugenioenko/ttt/internal/dap/providers/delve"
	"github.com/eugenioenko/ttt/internal/dap/providers/jsdebug"
)

func For(adapter dap.Adapter) dap.Provider {
	switch adapter.Provider {
	case "delve":
		return delve.Provider{Adapter: adapter}
	case "jsdebug":
		return jsdebug.Provider{Adapter: adapter}
	case "debugpy":
		return debugpy.Provider{Adapter: adapter}
	default:
		return adapter
	}
}
