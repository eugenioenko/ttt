#!/bin/sh
set -eu
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
cd "$dir"
mkdir -p .ttt config
export TTT_CONFIG_DIR="$dir/config"
cat > main.c <<'C'
int main(void) {
    int value = 41;
    value++;
    value += 2;
    return value == 44 ? 0 : 1;
}
C
cc -g -O0 "$dir/main.c" -o example
cat > config/settings.json <<'JSON'
{"debug":{"adapters":{
  "lldb":{"command":["codelldb","--port","${port}"],"transport":"server","languages":["c"]},
  "gdb":{"command":["gdb","-q","-nx","-i","dap"],"transport":"stdio","languages":["c"]}
}}}
JSON
for adapter in lldb gdb; do
  cat > .ttt/launch.json <<JSON
{"configurations":[{"name":"Native","type":"$adapter","request":"launch","program":"\${workspaceFolder}/example","cwd":"\${workspaceFolder}"}]}
JSON
  ttt --size 140x40 --exec 'key down; key down; key down; exec "Toggle Breakpoint"; exec "Start Debugging"; wait-for "value = 42"; exec "Debug: Step Over"; wait-for "value = 44"; exec "Debug: Continue"; panel output; wait-for "Debug session ended"; quit' "$dir" "$dir/main.c"
  echo "$adapter passed the C debugging fixture"
done
