#!/bin/sh
set -e

if command -v ttt >/dev/null 2>&1; then
  echo "ttt is already installed: $(ttt --version)"
  exit 0
fi

echo "Installing ttt..."
curl -sSfL https://raw.githubusercontent.com/eugenioenko/ttt/main/install.sh | sh

if ! command -v ttt >/dev/null 2>&1; then
  echo "ttt was installed but is not on PATH. Add the install directory to your PATH." >&2
  exit 1
fi

echo "ttt installed: $(ttt --version)"
