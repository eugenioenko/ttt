# Debug adapter compatibility

Run on Linux x86-64. Build ttt with `make build`. Each JSON file in `cases/` defines adapter settings, workspace launch arguments, source and binary fixtures, a breakpoint, and the lines and variables expected after stepping. The suite runs the compiled editor with `--exec` and isolated settings. CI installs and runs the same tests directly on its Linux runner.

To install the full suite locally, start with Node.js, Go, Python with venv support, PHP CLI with Xdebug 3.2 or newer, and `lsof`. From the repository root:

```bash
export DAP_TOOLS="$HOME/.cache/ttt-dap"
mkdir -p "$DAP_TOOLS/bin"
for script in $(jq -r '.requires[]' tests/dap/cases/*.json | sort -u); do
  bash "tests/dap/$script"
done
export PATH="$DAP_TOOLS/bin:$PATH"
node --test tests/dap/matrix/*.test.mjs
```

To run a single adapter, install the scripts in its case's `requires` list in order and select it by name. For example, with CodeLLDB and GDB installed:

```bash
DAP_CASES='["rust-codelldb"]' node --test tests/dap/matrix/*.test.mjs
```

The shared launch scenario checks saved source, gutter breakpoints, local variables, step over/in/out, output, and normal exit. The external-terminal Delve case uses a stand-in terminal launcher and verifies Stop. Lifecycle cases check that Stop and editor quit terminate a launched service. Attach cases inspect and step the target, detach, and verify its heartbeat advances. An unsupported attach mode carries a displayed skip reason. Xdebug initiates connections to PHP Debug, and NetCoreDbg attaches to a CLR process by PID. The PHP and C++ cases also check successive Xdebug requests and expanded object members, respectively.

The C, C++, Rust and .NET debug fixtures in `bin/linux-amd64/` have sources and rebuild scripts under `fixtures/`. Rebuilding them is optional. The .NET portable PDB uses `/proc/self/cwd` for stable source paths, so its case starts the editor in the fixture directory. The TypeScript fixtures include JavaScript and source maps. Running the suite uses neither the Rust nor .NET SDK nor the TypeScript compiler.
