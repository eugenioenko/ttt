#!/bin/sh
set -e

dir="."
if [ -n "$HERDR_PLUGIN_CONTEXT_JSON" ]; then
  worktree=$(printf '%s' "$HERDR_PLUGIN_CONTEXT_JSON" | grep -o '"worktree"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"worktree"[[:space:]]*:[[:space:]]*"//' | sed 's/"$//')
  if [ -n "$worktree" ]; then
    dir="$worktree"
  fi
fi

exec ttt "$dir"
