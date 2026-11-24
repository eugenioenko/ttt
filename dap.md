# Debugging in ttt

ttt supports one active launch or attach session for Go, Node.js/TypeScript, or Python. Define a named target in `.ttt/launch.json`. Each workspace folder can instead use `.vscode/launch.json` when its native file is absent.

## Using the debugger

Open **View → Debug** to see the sidebar before starting a session. Choose **Start Debugging** there to run a configured target.

Click the bookmark gutter to toggle a breakpoint. If a plugin handles gutter clicks, select the Debug sidebar first or use **Toggle Breakpoint**. Run **Start Debugging** and choose a target if several are configured. ttt saves modified file-backed tabs in the selected folder before starting the adapter. At a stop, use the Debug sidebar to expand variables, select a stack frame, and step or continue. Output appears under `debug:<target name>`. Use **Stop Debugging** for a launched program and **Detach Debugger** for an independently started program.

| Provider | Setup | Launch | Attach by address |
| --- | --- | --- | --- |
| Delve (`go`) | Put `dlv` on PATH. | Set `program` to the Go target. | Set `mode` to `remote` and supply the headless server's `host` and `port`. Start Delve with `--accept-multiclient` to retain the target on detach. |
| js-debug (`node`, `pwa-node`) | Put Node on PATH and set `TTT_JS_DEBUG_SERVER` to the standalone adapter's `src/dapDebugServer.js`. | Point `program` at the JavaScript entry point. For TypeScript, emit source maps and set `outFiles` to match generated JavaScript. | Set `address` and `port` to the Node inspector. An optional numeric `debugServer` selects a separate, already-running DAP server on loopback. |
| debugpy (`debugpy`) | Install debugpy in the selected interpreter. ttt defaults to the workspace's `.venv`, then Python on PATH. | Specify exactly one of `module` or `program`. | Supply the debugpy listener's `host` and `port` inside `connect`. |

All three support a positive `processId` or `"${command:pickProcess}"` for local attach. Node PID attach verifies the process owns the inspector port using lsof on Unix or netstat on Windows. The default inspector port is 9229. The full adapter matrix has been exercised on Linux.

### Configuration fields

Every target requires `name`, `type`, and `request`. Paths can use `${workspaceFolder}`. Configuration files accept comments and trailing commas. The optional `presentation` metadata is accepted.

| Provider | Additional launch fields | Additional attach fields |
| --- | --- | --- |
| Delve | `program`, `console`, `cwd`, `mode`, `args`, `env`, `buildFlags`, `stopOnEntry` | `mode`, `substitutePath`, and either `processId` or `host` and `port` |
| js-debug | `program`, `cwd`, `args`, `env`, `runtimeExecutable`, `runtimeArgs`, `console` | `processId`, `address`, `port`, `cwd`, `localRoot`, `remoteRoot` |
| debugpy | `program`, `module`, `cwd`, `python`, `args`, `env`, `justMyCode`, `stopOnEntry`, `console`, `redirectOutput` | `justMyCode`, `pathMappings`, and either `connect` or `processId` with optional `python` |

Both js-debug workflows also accept `debugServer`, `outFiles`, `sourceMaps`, `skipFiles`, `stopOnEntry`, and `timeout`. Go launches can use `console: "externalTerminal"` to open a separate terminal window with keyboard input. The repository’s **Launch ttt** target uses this mode. Node and Python launches use `internalConsole`.

## Real-editor verification

The Linux real-adapter run covers the following nine combinations. Each row uses the compiled editor, a source breakpoint, a known local and a step. The attach tests read a heartbeat written by the target after Detach, which proves that the independent program resumes execution. TypeScript coverage additionally checks stepping in the original source file and selecting the caller at its TypeScript call site.

| Adapter | Named launch | Picked and configured PID | Listening endpoint |
| --- | --- | --- | --- |
| Delve | Passed, including save, caller inspection and repeat launch | Both passed | Passed with a headless multi-client server |
| js-debug | Passed with compiled TypeScript and source maps | Both passed | Passed with an inspector endpoint and separately with an external DAP server |
| debugpy | Passed with a module and a virtual-environment-only dependency | Both passed | Passed with an independent debugpy listener |

To repeat the matrix, build ttt, place Delve, Node and tsc on PATH, set TTT_JS_DEBUG_SERVER to the standalone server script, and set TTT_TEST_DEBUGPY to a Python executable containing debugpy. From tests/functional run `pnpm exec vitest run debugger`. Check that the summary reports every test passed with zero skips. OS process-debugging permissions apply to attach.

The Linux release check runs 22 real-editor tests, including the nine workflows, PID picker and configured PID cases, workspace selection, failed attach and cancellation, and Stop and editor quit with each adapter. The TypeScript launch test also expands a nested value. The Python launch test steps into a callee and back into the caller. The Go suite covers failed saves for both request types, split gutters, late inspection replies and both DAP startup reply orderings. Run `make test` and `make lint` for the repository gates.

## The first debugging session

A developer opens a Go workspace, sets a breakpoint in a saved file, and chooses **Start Debugging**. If the workspace has one launch target, the command names and launches it. If there are several, the developer picks one. At a stop, the editor reveals the executing line, keeps keyboard focus, and shows the call stack and locals. Selecting another frame opens its source and shows its locals. Step Over updates both views. Continue clears the execution marker. After the program exits, the developer can change a breakpoint and run the same target again.

Build this loop for one active user session at a time. Ship Go with Delve, Node.js JavaScript/TypeScript with standalone vscode-js-debug, and Python with debugpy. Include step in, step out, pause when supported, stop, adapter output, and a clear startup error with the failed target name. An installed adapter is a prerequisite for the first implementation. Watches, expression evaluation, conditional breakpoints, interactive input for Node and Python, remote paths, and concurrent sessions are later increments.

## Choosing a target

A target has a display name, owning workspace folder, adapter ID, a launch or attach request, and adapter-specific arguments. The adapter definition describes how ttt starts and connects to the debugger. Keeping those records distinct lets two Go commands in one repository share Delve while using different programs, arguments, and working directories.

Offer named targets scoped to workspace folders and remember the last selected target for the current workspace. Show folder names in the picker when names collide. Resolve relative target paths and working directories against the owning folder. After selection, editing a file in a different folder must not silently change the target. If there are no targets, explain how to define one. An explicit start action launches the program. Opening a workspace only reads configuration.

Use `.ttt/launch.json` as the native project file. Lookup order per workspace folder is the native file first, then `.vscode/launch.json` when the native file is absent. Report parsing errors against the selected file. Both use a VS Code-style configurations array. For example:

```json
{
  "version": "0.2.0",
  "configurations": [
    { "name": "Run API", "type": "go", "request": "launch", "program": "${workspaceFolder}/cmd/api" },
    { "name": "Attach to API", "type": "go", "request": "attach", "mode": "local", "processId": 12345 }
  ]
}
```

The PID illustrates attaching to a running process. Also offer a local process picker at session start so the configuration can be reused when the PID changes. Read JSONC and expand workspaceFolder for the owning folder. Publish supported substitutions, adapter types and fields. Explain unsupported editor features such as preLaunchTask before starting the target. Keep adapter executable selection in user-controlled settings or an approved provider.

## Language and attach requirements

| Workflow | Go (Delve) | Node.js JavaScript and TypeScript (js-debug) | Python (debugpy) |
| --- | --- | --- | --- |
| Launch | Start Delve and launch the configured Go target. | Start standalone js-debug and launch the Node entry point. For compiled TypeScript, use source maps to bind breakpoints and open frames in the original .ts files. | Launch a script (program) or package entry point (module) under the selected interpreter. |
| Attach by process | Pick a local process or supply a PID. Send a local attach request through dlv dap. | Pick a local Node process or supply its PID. Resolve it using the supported js-debug attach flow for the platform. | Pick a running Python process or supply its PID to debugpy. Surface platform attach errors. |
| Attach by address | Connect to an existing headless Delve debugging a target and send remote attach. | Attach through js-debug to a listening Node inspector address. Also support connecting ttt to an externally started js-debug DAP server and configuring its target. | Connect ttt to the adapter endpoint opened by debugpy.listen() or the debugpy CLI and issue attach. |

Represent the DAP server endpoint separately from the target inspector endpoint. A Node inspector port speaks a different protocol. A configuration may need both addresses when js-debug is already running. ttt closes its own connections on detach and only reaps processes it started. Verify externally managed Delve detach behavior with a server configured to retain the target.

Treat the initial Node target and any adapter-required child debug sessions as one user-visible session. Verify standalone js-debug's target-session startup before finalizing the event and breakpoint routing. A successful handshake with its initial server is insufficient evidence that ttt can inspect a TypeScript frame.

Adapter discovery and launch defaults can ship as language plugins if useful. All three providers are first-release requirements whichever packaging is chosen. Browser JavaScript debugging is a separate scope decision.

## Ownership

- **internal/dap** owns DAP messages, adapter process and transport, capabilities, request correlation, session startup and teardown. Expose typed events and cancellable operations. Processes and protocol state belong in the services zone.
- **internal/app** owns launch selection, breakpoint intent across tabs and sessions, the active session and selected thread/frame, and routing results onto the event loop. Give each session an identity and each stop a generation so late replies cannot repaint after continue, frame selection, or restart.
- **internal/ui** draws gutter markers, execution location and a Debug panel from application state. Use existing tree/list widgets, source navigation, theme styles, icons, and translated copy. Route adapter diagnostics and DAP output events through the Output panel under a session-specific source.
- **Language support** supplies adapter discovery and launch defaults. Derive the provider contract from the Delve, js-debug and debugpy implementations before specifying a public plugin API. A future plugin contribution must respect plugin execution permissions and unload semantics.

## Starting and ending a session

Share Content-Length framing with internal/jsonrpc. Its current Codec.Receive decodes a JSON-RPC message. Expose the framed payload below that decoder, preserving size limits and existing LSP and plugin behavior. DAP owns its request, response, and event envelopes.

1. Resolve the target, start an owned adapter or select the configured external DAP server, connect, and install event handlers before reading. Support the startup mechanism used by Delve, including discovering its local TCP endpoint. Keep adapter stderr available for startup diagnostics.
2. Send initialize and record capabilities. Send the configured launch or attach request asynchronously. When initialized arrives, send the latest set of breakpoints for each source, then configurationDone if supported. An adapter can delay its reply until configuration completes. The reader and the UI must continue processing during this exchange.
3. At a stopped event, select the reported thread or request threads if it was omitted. Fetch its stack, then scopes for the selected frame. Fetch variables as scopes and children expand. Keep the executing location distinct from a frame the developer selects for inspection. Track whether all threads stopped or only the reported thread.
4. Stop ends a launched program. Detach disconnects from an attached program and leaves it running. Track ownership of the adapter process separately from ownership of the debuggee, including when startup fails.
5. On resume, invalidate affected stack frames and variable handles. On termination, release pending requests, close transport, reap the owned adapter, and clear session decorations. Record an exited event's exit code separately from the terminated event. Handle duplicate events, adapter EOF, startup cancellation, user Stop, and editor quit through one idempotent cleanup path. Bound startup and shutdown, including a child which never reports its address.

Keep transport reads off the UI goroutine. Post results to the main event loop, and reject stale results there. Capability flags control optional requests and actions. Answer unsupported reverse requests with a failure. Advertise runInTerminal only when ttt can honor its full lifecycle. For the first target verify where actual program stdout/stderr goes with Delve. DAP output events alone do not guarantee capture of program output.

## Breakpoints and source identity

Store requested breakpoints per workspace and source path. Give each an identity independent of its requested line. Keep adapter IDs, resolved locations, verification and explanatory messages per session. A filled red dot marks a breakpoint before launch or once verified. A hollow red ring marks an unverified breakpoint during a session or a line needing confirmation, and a right-pointing execution chevron remains recognizable when both occupy the same row. Surface the adapter's reason when a breakpoint cannot bind.

Send the complete desired set for a source on change, including an empty set when removing its last breakpoint. Serialize or coalesce updates per source so a late reply cannot restore a removed marker. Keep a requested breakpoint when the adapter relocates it. Drop only the session's binding when debugging ends.

Render the same breakpoint in splits showing the same file. Apply buffer changes once per buffer, including undo/redo. ChangesSince records line splices and has bounded history. A missed history window or replacement of a whole file needs an explicit revalidation policy. For the first version, mark ambiguous locations pending and ask the user to confirm them before sending. Treat untitled buffers, deleted files, Save As, and external reloads explicitly. Propose retaining intent across runs in the open workspace. Persistence across editor restarts needs a separate storage decision.

Save modified source before launch or attach. Proposed scope is file-backed buffers in the selected workspace folder. Use the normal editor save path and abort startup on failure or cancellation. Saving on attach cannot rebuild the running process. Changes made since its build can still make source lines disagree. Mark affected breakpoint bindings pending when source changes during a session.

## Controls and layout

The Debug sidebar offers the configuration picker, Start or Attach, and session controls. The top two rows are reserved for the session buttons. Drag the horizontal dividers between Variables, Call Stack, and Breakpoints to resize those sections for the current editor instance. Each section scrolls independently.

- **Variables** shows scopes and expandable values for the selected frame. Clear stopped-state values on resume.
- **Call Stack** groups frames by thread. Selecting a frame reveals its source and refreshes Variables.
- **Breakpoints** lists requested locations and binding status even with the source tab closed. Selecting one navigates to its source. Keep the list available between sessions.

Reveal the sidebar on start and keep editor keyboard focus on a breakpoint hit. Keep Stop or Detach available during startup. Program output belongs in the bottom Output panel when supplied by the adapter. For an attached program whose output belongs to another terminal, indicate where to look when known.

The Debug view uses the existing bookmark column to toggle breakpoints. The current execution line shows a solid yellow pointer (⏵). Themes can set `editor.debugExecution` and `editor.debugBreakpoint`. It takes visual precedence over a breakpoint, and a breakpoint takes precedence over a bookmark. Removing a breakpoint reveals the underlying bookmark. Outside the Debug view, plugins listening for gutter clicks take priority. With no listener, clicking the bookmark column in a source file toggles a breakpoint. **Toggle Breakpoint** works from any view.

Expose start, continue, pause, stop, stepping and toggle-breakpoint commands through the normal command registry. F10 already opens the File menu. Prototype stepping through the palette and choose defaults after checking terminal focus, existing bindings, and discoverability. Existing debug.* developer tools already use distinct command IDs. Product debugger commands must be available in ordinary builds.

## Implementation and evidence

1. Build a fake adapter and session service. Verify both launch reply orderings, clearing breakpoints, cancellation during startup, adapter exit during a request, and cleanup. Exercise shared framing against existing LSP and plugin tests.
2. Exercise every cell of the workflow table through the editor's target resolver and session manager with real adapters. Set a breakpoint, inspect a known local, step, then end the session. Check launch output, cleanup and re-launch. After attach, verify detach leaves the independent target usable. Include a TypeScript fixture with generated JavaScript and source maps. Assert that a breakpoint binds in .ts and that stepping keeps navigation in the original source. Cover PID and address selection, unreachable endpoints and cancellation.
   For Python, start the managed adapter over stdio and exercise TCP through an independently started listener. Launch a module from a virtual environment with a dependency available only in that environment. Verify its interpreter, breakpoint and local value. Exercise cancellation while waiting for attach and ensure a failed PID attach leaves the target alive.
3. Connect editor commands, breakpoint intent, markers and the Debug sidebar. SimScreen tests should prove split consistency, focus at a stop, correct frame navigation, late replies after continue, and selection of the intended target in a multi-folder workspace.
4. Cover file precedence, malformed native config, unsupported substitutions, and a failed save preventing both launch and attach.
5. Compare all three required providers before stabilizing adapter contributions. Record which pieces require language knowledge, which settings users need to override, and whether a declarative provider suffices.

## Optional interoperability check

LLVM’s lldb-dap is a candidate for a small native C fixture. Compile with debug symbols, rewrite the recorded source directory at build time, then configure LLDB source mapping back to the workspace. Check that a breakpoint binds and the frame opens the existing editor buffer. Strip the symbols in a second build and verify that a frame without source can still be selected and execution resumed. Keep this check optional until C/C++ support is explicitly in scope. [LLDB DAP documentation](https://lldb.llvm.org/use/lldbdap.html)

## Release decisions

- **Adapter packaging.** Delve, js-debug, and debugpy run as external processes. Their built-in integrations live in separate packages under `internal/dap/providers`. The registry maps launch configuration types to `dap.Provider`. Its Arguments operation validates and expands configuration before saving. Its Open operation runs after saving and uses the host’s stdio, listening-process, or external TCP transport. Both the editor and the shared DAP session operate without language switches. A future plugin host can implement the same provider interface and enforce its process permissions at registration and startup. A provider needs to validate and resolve configuration within a workspace, discover or start an adapter, and distinguish owned processes from external endpoints. Delve needs remote detach handling, debugpy needs interpreter selection, and js-debug needs inspector resolution plus a second DAP session. An executable and argument template alone cannot cover all three. Keep transport framing, request correlation, breakpoint intent, inspection, and editor controls shared. A public plugin API for adding providers remains a future extension.
- **Configuration.** Explicitly starting a named configuration authorizes its configured program, interpreter, and arguments. Merely opening a folder never starts an adapter. The user selects the standalone js-debug script through TTT_JS_DEBUG_SERVER. The supported interpolation is `${workspaceFolder}`, plus `${command:pickProcess}` in a processId field. Browser targets, pre-launch tasks and embedded debugger terminals are outside this release.
- **Source identity.** Save file-backed buffers owned by the selected folder using normal save checks. An untitled buffer has no source path. A breakpoint belongs to its requested path, including across Save As. Reopening a replacement buffer or overwriting a breakpoint's source line requires confirming its location. An edit during a session clears the affected binding. For attach, the user is responsible for matching local source to the independently built program.
- **Interaction.** Use palette commands and sidebar actions. F10 retains its existing menu binding. The shared bookmark gutter uses distinct pending, verified and executing glyphs. The Breakpoints list retains intent for the current editor lifetime and displays relocation and the adapter's explanation. The selected stack frame controls navigation and variables independently of the execution marker.
- **Output and shutdown.** Delve uses remote output mode. Node and Python launches use the internal console. DAP output and managed-adapter stderr go to Output. A program attached after startup can still write to its original terminal. Real-editor lifecycle tests exercise Pause followed by Stop and Pause followed by editor quit for each adapter, checking disappearance of the launched PID. The failure tests cover stalled attach cancellation, a refused connection on retry, and an independent target responding after PID attach startup fails.

## Design references

- [Python debugpy](https://github.com/microsoft/debugpy) provides the Python adapter. Its [client handler](https://github.com/microsoft/debugpy/blob/main/src/debugpy/adapter/clients.py) distinguishes stdio, sockets and PID attach.

- [Delve DAP modes](https://github.com/go-delve/delve/blob/master/Documentation/api/dap/README.md) specifies local process attach and remote headless sessions.
- [Microsoft js-debug](https://github.com/microsoft/vscode-js-debug) publishes the standalone adapter. Its [configuration types](https://github.com/microsoft/vscode-js-debug/blob/main/src/configuration.ts) describe Node attach and source mapping.

- [DAP lifecycle and sequencing](https://microsoft.github.io/debug-adapter-protocol/overview.html#launch-sequencing) explains when adapters accept breakpoints and when variable handles expire.
- [Zed breakpoint store](https://github.com/zed-industries/zed/blob/main/crates/project/src/debugger/breakpoint_store.rs) and [session](https://github.com/zed-industries/zed/blob/main/crates/project/src/debugger/session.rs) show how project breakpoint intent and adapter bindings can have different lifetimes.
- [VS Code debug session](https://github.com/microsoft/vscode/blob/main/src/vs/workbench/contrib/debug/browser/debugSession.ts) and [adapter manager](https://github.com/microsoft/vscode/blob/main/src/vs/workbench/contrib/debug/browser/debugAdapterManager.ts) show event-driven configuration and adapter selection.
- Historical ttt work is available at origin/feat/dap-support, commits 2cf7ec39 and 516eeb1c. Its Delve fixture is a useful behavioral reference. The old manager and integration test took different startup paths.
