# Debug adapter compatibility

On Linux x86-64, build ttt with `make build` and make the debugger in the case's `settings` available on PATH. Run `node --test tests/dap/matrix/*.test.mjs`. The tests use the real editor's `--exec` interface and a temporary workspace and settings directory.

Each JSON file in `cases/` provides the adapter settings, launch target, source fixture, binary, breakpoint line, and installation scripts. CI runs the scripts with `DAP_TOOLS` pointing at its tool directory and adds `$DAP_TOOLS/bin` to PATH. The C fixture's `build.sh` reproduces its committed debug binary with relative source paths.
