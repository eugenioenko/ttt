#!/usr/bin/env bash
set -euo pipefail
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
MAMBA_ROOT_PREFIX="$scratch" "$DAP_TOOLS/bin/micromamba" create -y \
  -p "$DAP_TOOLS/llvm-lldb" --override-channels -c conda-forge 'lldb=20.1.8'
ln -sf "$DAP_TOOLS/llvm-lldb/bin/lldb-dap" "$DAP_TOOLS/bin/lldb-dap"
