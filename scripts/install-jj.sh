#!/usr/bin/env bash
# Installs a pinned, checksum-verified jj release for the jj provider tests.
set -euo pipefail

version="v0.47.1"
archive="jj-${version}-x86_64-unknown-linux-musl.tar.gz"
sha256="d4b657f622ee50929c8916466dfa3d6a6c17a0a32b50e7b2c793707ec9ebcad7"
dest="${JJ_INSTALL_DIR:-$HOME/.local/bin}"

if [ "$(uname -s)-$(uname -m)" != "Linux-x86_64" ]; then
  echo "install-jj.sh only supports Linux x86_64" >&2
  exit 1
fi

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
curl -sSfL -o "$work/$archive" "https://github.com/jj-vcs/jj/releases/download/${version}/${archive}"
echo "${sha256}  $work/$archive" | sha256sum -c -
tar -xzf "$work/$archive" -C "$work" ./jj
mkdir -p "$dest"
install -m 755 "$work/jj" "$dest/jj"
if [ -n "${GITHUB_PATH:-}" ]; then
  echo "$dest" >> "$GITHUB_PATH"
fi
"$dest/jj" --version
