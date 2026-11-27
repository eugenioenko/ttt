#!/usr/bin/env bash
set -euo pipefail
GOBIN="$DAP_TOOLS/bin" go install github.com/go-delve/delve/cmd/dlv@v1.26.0
