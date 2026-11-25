# DAP compatibility

The `dap-tests` CI job runs the real editor and real debuggers directly on Linux/amd64. Each JSON case drives the editor through `--exec`, sets a source breakpoint, reads a local in the Debug sidebar, steps, and continues to exit.

## Run locally

Use a Linux/amd64 checkout with Go, Node 24, Bash, curl, tar, unzip, Python 3 with venv, and pnpm. The additional PID attach scenarios use lsof and Linux ptrace. Run the following from the repository root:

```sh
make build
node tests/dap/run.mjs --install
node tests/dap/run.mjs --run
pnpm --dir tests/functional install --frozen-lockfile
node tests/dap/run.mjs --scenarios
```

For a single debugger, select cases by name on both commands:

```sh
node tests/dap/run.mjs --install rust-codelldb
node tests/dap/run.mjs --run rust-codelldb
```

Installation is explicit. The runner executes each selected Bash installer once, even when several cases share it. The scripts write under `$DAP_TOOLS`, defaulting to `~/.cache/ttt-dap` (or `$XDG_CACHE_HOME/ttt-dap`). Runs prepend its `bin` directory to PATH and use private temporary workspaces and settings. You can skip installation if the case's adapter commands already work on your PATH. The Python case uses the installed `dap-python` launcher.

## Add a case

Copy a file in `cases/` and give it a unique `name`. The runner discovers it automatically.

- `requires` lists installer paths relative to this directory. Reuse an existing script or add one in `install/`.
- `source` names the file ttt opens. Its entire fixture directory is copied to the workspace root, including dependencies and source maps.
- Optional `binary` names a prebuilt debuggee under `bin/linux-amd64/`. The runner copies it to the workspace root using its basename.
- `settings` is the ttt user settings object. `launch` is one ordinary launch.json configuration.
- `steps` gives the ordered actions and assertions. Breakpoint and stop lines are one-based. Each stop waits for a paused session, a matching stack location, and the expected locals, then checks the execution marker in a snapshot. Exit can also require output from the named debug session.

C and Rust share the CodeLLDB installer. GDB and CodeLLDB share the C source and executable. Rebuild native fixtures with the adjacent `build.sh` on Linux/amd64 when their sources change. Both use relative debug paths so breakpoints resolve in a fresh workspace. Go exercises Delve package launch. TypeScript fixtures include emitted JavaScript and source maps; regenerate with `tsc --target es2020 --sourceMap main.ts` in `ts-counter`, or `tsc --target es2020 --sourceMap --outDir dist app.ts` in `ts-sourcemap`. Running the suite requires neither rustc, a C compiler for the native fixtures, nor tsc.

The existing black-box scenarios in `scenarios/` exercise additional behavior such as PID/address attach, detach, terminal reverse requests, workspace selection, and process cleanup. They share the functional suite's editor driver and Vitest installation. Only the DAP job runs these debugger scenarios.
