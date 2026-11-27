#!/usr/bin/env bash
set -euo pipefail

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
version=$(cat "$repo_dir/.golangci-version")
if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Invalid golangci-lint version: $version" >&2
  exit 1
fi

host_os=$(go env GOHOSTOS)
host_arch=$(go env GOHOSTARCH)
cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}/ttt/golangci-lint/$version/$host_os-$host_arch"
binary_name=golangci-lint
if [[ "$host_os" == windows ]]; then
  binary_name+=.exe
fi
binary="$cache_dir/$binary_name"

if [[ ! -x "$binary" ]]; then
  mkdir -p "$cache_dir"
  install_dir=$(mktemp -d "$cache_dir/.install.XXXXXX")
  trap 'rm -rf "$install_dir"' EXIT
  curl --fail --silent --show-error --location \
    "https://raw.githubusercontent.com/golangci/golangci-lint/$version/install.sh" \
    --output "$install_dir/install.sh"
  sh "$install_dir/install.sh" -b "$install_dir" "$version"
  mv "$install_dir/$binary_name" "$binary"
  rm -rf "$install_dir"
  trap - EXIT
fi

exec "$binary" "$@"
