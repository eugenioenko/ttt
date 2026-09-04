#!/bin/sh
set -e

if command -v ttt >/dev/null 2>&1; then
  echo "ttt is already installed: $(ttt --version)"
  exit 0
fi

echo "Installing ttt..."
curl -sSfL https://raw.githubusercontent.com/eugenioenko/ttt/main/install.sh | sh
