#!/bin/sh
set -e

target="${HERDR_PLUGIN_CLICKED_URL:-}"
if [ -z "$target" ]; then
  exec ttt .
fi

exec ttt "$target"
