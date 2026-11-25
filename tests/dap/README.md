# DAP compatibility

The `dap-tests` CI job runs the real editor and real debuggers directly on Linux/amd64. Each JSON case drives the editor through `--exec`, runs the shared launch, stepping, attach and process-lifecycle tests in `matrix/`. Each result names its language and debugger.

## Run locally

Use a Linux/amd64 checkout with Go, Node 24, Bash, curl, tar, unzip, Python 3 with venv, and pnpm. The additional PID attach scenarios use lsof and Linux ptrace. Run the following from the repository root:

```sh
make build
node tests/dap/run.mjs --install
pnpm --dir tests/functional install --frozen-lockfile
node tests/dap/run.mjs --run
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
- Optional `binary` names a prebuilt executable or a directory of runtime files under `bin/linux-amd64/`. The runner copies it to the workspace root using its basename.
- `settings` is the ttt user settings object. `launch` is one ordinary launch.json configuration.
- `baseline` supplies the expected line and locals at the initial breakpoint, after stepping over, inside the called function, and after stepping out, plus the expected program output. Every adapter runs that sequence.
- `service` provides a long-running fixture, its command, optional build command or binary, breakpoint and next line, and expected locals. It writes its PID to `pid`, prints `TARGET_READY`, and advances a numeric `heartbeat` file. The lifecycle tests use its `lifecycle` launch arguments to pause it and verify termination on Stop and Quit.
- `pidAttach` and `addressAttach` specify attach arguments. The harness supplies a numeric `${pid}` or `${port}`. An address listener reports its port through `portFile` or a capture group in `portPattern`. Native cases use gdbserver. Set `continueOnAttach` for listeners that initially stop the target.
- Every attach mode must inspect locals, step, detach, and observe the heartbeat advancing. Declare `{"skip": "reason"}` for an unsupported attach mode or picker. The report prints the reason. The generic provider currently supports configured native PIDs. The interactive process picker is available with the Go, Python, and JS provider hooks.

C, C++, and Rust share the CodeLLDB installer. GDB, CodeLLDB, and LLVM lldb-dap share the C source and executable. Rebuild native fixtures with the adjacent `build.sh` on Linux/amd64 when their sources change. Both use relative debug paths so breakpoints resolve in a fresh workspace. NetCoreDbg uses the checked-in .NET 8 assembly, its portable PDB and runtime metadata. Its PDB requires exact absolute source paths. The C# case uses `/proc/self/cwd` as its workspace path so every isolated Linux test sees the same source path used at build time. Rebuild the assembly with `fixtures/csharp/build.sh` and the .NET 8 SDK. CI only installs the .NET runtime. Go exercises Delve package launch. TypeScript fixtures include emitted JavaScript and source maps; regenerate with `tsc --target es2020 --sourceMap main.ts` in `ts-counter`, or `tsc --target es2020 --sourceMap --outDir dist app.ts` in `ts-sourcemap`. Running the suite requires neither rustc, a C compiler for the native fixtures, nor tsc.

The PHP matrix uses PHP CLI with Xdebug 3.2 or newer and the pinned PHP Debug adapter. Its launch recipe disables the extra stop for inspecting function return values so Step Out lands back in the caller. Verify the local prerequisite with `php --ri xdebug`. The PHP case uses `service.stopAtBreakpoint` to exercise Stop and Quit on Xdebug versions without asynchronous pause support. Two successive PHP CLI processes also hit the same breakpoint in a single adapter session.

The additional black-box scenarios in `scenarios/` exercise additional behavior such as terminal reverse requests, workspace selection, source maps, and failure recovery. They share the functional suite's editor driver and Vitest installation. Only the DAP job runs these debugger scenarios.
