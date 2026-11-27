#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
rustc --remap-path-prefix "$PWD=." -C debuginfo=2 -C opt-level=0 -C link-arg=-Wl,--compress-debug-sections=zlib main.rs -o ../../bin/linux-amd64/rust-counter
