#!/bin/sh
set -e

die() {
  printf 'ttt herdr plugin: %s\n' "$1" >&2
  exit 1
}

# A TOML string value, and nothing else on the line: only whitespace or a comment
# may follow the closing quote, so "split" garbage is a config error, not "split".
toml_value() {
  printf '%s' "$1" | sed -n \
    -e 's/^[[:space:]]*"\([^"]*\)"[[:space:]]*$/\1/p' \
    -e 's/^[[:space:]]*"\([^"]*\)"[[:space:]]*#.*$/\1/p'
}

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
  # The layout knobs live in the plugin config, and only the actions read them: a
  # keybinding that calls `plugin pane open` itself gets the manifest placement unless
  # its own command passes herdr's placement flags.
  # Unset keys ask herdr for nothing, which leaves the manifest in charge.
  placement=""
  direction=""
  config_file=""
  if [ -n "${HERDR_PLUGIN_CONFIG_DIR:-}" ]; then
    config_file="$HERDR_PLUGIN_CONFIG_DIR/config.toml"
  elif config_dir=$("${HERDR_BIN_PATH:-herdr}" plugin config-dir ttt.editor 2>/dev/null); then
    config_file="$config_dir/config.toml"
  fi

  if [ -f "$config_file" ]; then
    # An unknown key, an unquoted value, or trailing text after the value says so
    # (see `herdr plugin log list`) instead of quietly keeping the default placement.
    while IFS= read -r line || [ -n "$line" ]; do
      line=$(printf '%s' "$line" | sed 's/^[[:space:]]*//')
      case "$line" in '' | '#'*) continue ;; esac
      key=$(printf '%s' "${line%%=*}" | tr -d '[:space:]')
      value=$(toml_value "${line#*=}")
      case "$key" in
        open_placement)
          case "$value" in
            tab | split | overlay | zoomed) placement=$value ;;
            '') die "config $config_file: open_placement needs a quoted value like \"split\", with nothing after the closing quote" ;;
            *) die "config $config_file: open_placement must be tab, split, overlay or zoomed" ;;
          esac
          ;;
        open_direction)
          case "$value" in
            right | down) direction=$value ;;
            '') die "config $config_file: open_direction needs a quoted value like \"right\", with nothing after the closing quote" ;;
            *) die "config $config_file: open_direction must be right or down" ;;
          esac
          ;;
        *) die "config $config_file: unknown key '$key' (expected open_placement, open_direction)" ;;
      esac
    done < "$config_file"

    if [ -n "$direction" ]; then
      case "$placement" in
        split | zoomed) ;;
        *) die "config $config_file: open_direction needs open_placement set to split or zoomed" ;;
      esac
    fi
  fi

  set -- --plugin ttt.editor --entrypoint editor \
    --env "TTT_TARGET_DIR=$target" --focus
  if [ -n "$placement" ]; then
    set -- "$@" --placement "$placement"
  fi
  # herdr rejects --direction and --target-pane for any other placement.
  case "$placement" in
    split | zoomed)
      if [ -n "$direction" ]; then
        set -- "$@" --direction "$direction"
      fi
      # HERDR_PANE_ID is the invocation context's focused pane; workspace contexts
      # can omit it, and herdr then splits its own focused pane.
      if [ -n "${HERDR_PANE_ID:-}" ]; then
        set -- "$@" --target-pane "$HERDR_PANE_ID"
      fi
      ;;
  esac
  exec "${HERDR_BIN_PATH:-herdr}" plugin pane open "$@"
fi

# herdr routes right-click to its own pane menu by default, which hides ttt's
# context menus; routing is per-pane runtime state, so set it before exec.
if [ -n "${HERDR_PLUGIN_ENTRYPOINT_ID:-}" ]; then
  "${HERDR_BIN_PATH:-herdr}" pane input --current --right-click pane >/dev/null 2>&1 || true
fi

# Pane / interactive context: cd into the project so herdr's cwd-following sees
# the pane living there rather than in the plugin root.
if [ -n "$target" ] && [ -d "$target" ]; then
  cd "$target"
  exec ttt
fi

exec ttt "$target"
