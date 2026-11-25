#!/usr/bin/env bash
set -euo pipefail
# Use a Python-enabled GDB independent of the host distribution's GDB version.
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT
archive="$scratch/micromamba.tar.bz2"
curl -fsSL --retry 3 https://micro.mamba.pm/api/micromamba/linux-64/2.3.3 -o "$archive"
echo "e7274528ceb9c20d048a428d6c22d7e02e268f8ffb762c4c365422347c8b8ba2  $archive" | sha256sum -c -
tar -xjf "$archive" -C "$scratch" bin/micromamba
MAMBA_ROOT_PREFIX="$scratch/mamba" "$scratch/bin/micromamba" create -y \
  -p "$DAP_TOOLS/gdb" --override-channels -c conda-forge 'gdb=16.3'
ln -sf "$DAP_TOOLS/gdb/bin/gdb" "$DAP_TOOLS/bin/gdb"
ln -sf "$DAP_TOOLS/gdb/bin/gdbserver" "$DAP_TOOLS/bin/gdbserver"
