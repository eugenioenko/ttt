#!/bin/sh
set -e

dir=""
if [ -n "${HERDR_PLUGIN_CONTEXT_JSON:-}" ]; then
  dir=$(printf '%s' "$HERDR_PLUGIN_CONTEXT_JSON" | grep -o '"checkout_path"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"checkout_path"[[:space:]]*:[[:space:]]*"//' | sed 's/"$//')
  if [ -z "$dir" ]; then
    dir=$(printf '%s' "$HERDR_PLUGIN_CONTEXT_JSON" | grep -o '"focused_pane_cwd"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"focused_pane_cwd"[[:space:]]*:[[:space:]]*"//' | sed 's/"$//')
  fi
  if [ -z "$dir" ]; then
    dir=$(printf '%s' "$HERDR_PLUGIN_CONTEXT_JSON" | grep -o '"workspace_cwd"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"workspace_cwd"[[:space:]]*:[[:space:]]*"//' | sed 's/"$//')
  fi
fi

exec ttt "${dir:-.}"
