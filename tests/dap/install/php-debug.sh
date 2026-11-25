#!/usr/bin/env bash
set -euo pipefail
php --ri xdebug >/dev/null || { echo 'Install PHP CLI with the Xdebug extension.' >&2; exit 1; }
asset=$(mktemp)
trap 'rm -f "$asset"' EXIT
curl -fsSL --retry 3 https://github.com/xdebug/vscode-php-debug/releases/download/v1.40.2/php-debug-1.40.2.vsix -o "$asset"
echo "17631993fe800083a2fc89a6782c8bf58a79603ad201ddff60d9b59ba2345d5f  $asset" | sha256sum -c -
unzip -oq "$asset" 'extension/*' -d "$DAP_TOOLS/php-debug"
cat > "$DAP_TOOLS/bin/php-debug-adapter" <<'SH'
#!/usr/bin/env bash
exec node "$DAP_TOOLS/php-debug/extension/out/phpDebug.js" "$@"
SH
chmod +x "$DAP_TOOLS/bin/php-debug-adapter"
