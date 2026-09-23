#!/usr/bin/env bash
set -euo pipefail

if command -v rg >/dev/null 2>&1; then
  echo "ripgrep already installed: $(rg --version | head -n1)"
  exit 0
fi

sudo_cmd=""
if [ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null 2>&1; then
  sudo_cmd="sudo"
fi

if command -v brew >/dev/null 2>&1; then
  brew install ripgrep
elif command -v apt-get >/dev/null 2>&1; then
  $sudo_cmd apt-get update
  $sudo_cmd apt-get install -y ripgrep
elif command -v dnf >/dev/null 2>&1; then
  $sudo_cmd dnf install -y ripgrep
elif command -v pacman >/dev/null 2>&1; then
  $sudo_cmd pacman -S --noconfirm ripgrep
elif command -v apk >/dev/null 2>&1; then
  $sudo_cmd apk add ripgrep
elif command -v zypper >/dev/null 2>&1; then
  $sudo_cmd zypper install -y ripgrep
else
  echo "No supported package manager found. Install ripgrep manually: https://github.com/BurntSushi/ripgrep#installation" >&2
  exit 1
fi

echo "ripgrep installed: $(rg --version | head -n1)"
