#!/bin/sh
set -e

json_str() {
  printf '%s' "$HERDR_PLUGIN_CONTEXT_JSON" \
    | grep -o "\"$1\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" \
    | head -1 \
    | sed "s/.*\"$1\"[[:space:]]*:[[:space:]]*\"//" \
    | sed 's/"$//'
}

dir=""
if [ -n "${HERDR_PLUGIN_CONTEXT_JSON:-}" ]; then
  dir=$(json_str checkout_path)
  [ -z "$dir" ] && dir=$(json_str focused_pane_cwd)
  [ -z "$dir" ] && dir=$(json_str workspace_cwd)
fi

target="${TTT_TARGET_DIR:-${dir:-.}}"

# Actions run headless (no PTY); exec'ing the TUI here panics on /dev/tty.
# Re-dispatch to the pane entrypoint, which herdr runs with a PTY. The target
# dir goes through --env, not --cwd: herdr spawns plugin commands from the
# plugin root and the manifest command is a relative path, so --cwd would make
# the script unfindable.
if [ -z "${HERDR_PLUGIN_ENTRYPOINT_ID:-}" ] && ! { [ -t 0 ] && [ -t 1 ]; }; then
  exec "${HERDR_BIN_PATH:-herdr}" plugin pane open \
    --plugin ttt.editor \
    --entrypoint editor \
    --env "TTT_TARGET_DIR=$target" \
    --focus
fi

# Pane / interactive context: cd into the project so herdr's cwd-following sees
# the pane living there rather than in the plugin root.
if [ -n "$target" ] && [ -d "$target" ]; then
  cd "$target"
  exec ttt
fi

exec ttt "$target"
