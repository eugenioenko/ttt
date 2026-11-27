#!/usr/bin/env bash
set -euo pipefail
asset=$(mktemp)
trap 'rm -f "$asset"' EXIT
curl -fsSL --retry 3 https://github.com/vadimcn/codelldb/releases/download/v1.12.3/codelldb-linux-x64.vsix -o "$asset"
echo "1cd7f386598022b51a5b93b9ffa23e812b23f519cfe1833384ec4bef4bfd1be1  $asset" | sha256sum -c -
unzip -oq "$asset" 'extension/adapter/*' 'extension/lldb/*' -d "$DAP_TOOLS/codelldb"
ln -sf "$DAP_TOOLS/codelldb/extension/adapter/codelldb" "$DAP_TOOLS/bin/codelldb"
