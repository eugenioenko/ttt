#!/usr/bin/env bash
set -euo pipefail
archive=$(mktemp)
trap 'rm -f "$archive"' EXIT
curl -fsSL --retry 3 https://github.com/microsoft/vscode-js-debug/releases/download/v1.140.1/js-debug-dap-v1.140.1.tar.gz -o "$archive"
echo "c72fab31bcbfdce6ad1f42adfd28eff8d4681dd3b03b5e2e6f9e72e6f9927faf  $archive" | sha256sum -c -
tar -xzf "$archive" -C "$DAP_TOOLS"
cat > "$DAP_TOOLS/bin/js-debug-adapter" <<'SH'
#!/usr/bin/env bash
exec node "$DAP_TOOLS/js-debug/src/dapDebugServer.js" "$@"
SH
chmod +x "$DAP_TOOLS/bin/js-debug-adapter"
