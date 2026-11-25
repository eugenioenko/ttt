#!/usr/bin/env bash
set -euo pipefail
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
curl -fsSL --retry 3 https://github.com/Samsung/netcoredbg/releases/download/3.1.2-1054/netcoredbg-linux-amd64.tar.gz -o "$scratch/adapter"
echo "4f58ae90aacbccdf85dc7d6a0d8893796dd3750d6867fe31396a6b7bee09c9cb  $scratch/adapter" | sha256sum -c -
tar -xzf "$scratch/adapter" -C "$DAP_TOOLS"
curl -fsSL --retry 3 https://dotnetcli.blob.core.windows.net/dotnet/Runtime/8.0.34/dotnet-runtime-8.0.34-linux-x64.tar.gz -o "$scratch/runtime"
echo "3949c533b04b0f2b8ed7f6a595409806857891de07eca5ff2795c3ffecc5c373c8977ce296ea1124cef0890f8afeb3365a3bb8514d11a91ba79adb5c47c27ddb  $scratch/runtime" | sha512sum -c -
mkdir -p "$DAP_TOOLS/dotnet"
tar -xzf "$scratch/runtime" -C "$DAP_TOOLS/dotnet"
ln -sf "$DAP_TOOLS/dotnet/dotnet" "$DAP_TOOLS/bin/dotnet"
cat > "$DAP_TOOLS/bin/netcoredbg" <<'SH'
#!/usr/bin/env bash
export DOTNET_ROOT="$DAP_TOOLS/dotnet"
exec "$DAP_TOOLS/netcoredbg/netcoredbg" "$@"
SH
chmod +x "$DAP_TOOLS/bin/netcoredbg"
