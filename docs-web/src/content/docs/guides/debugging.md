---
title: Debugging
description: Launch a program, attach to a running process, and inspect it from the Debug sidebar.
---

Open **Show Debug** from the command palette. The Debug sidebar is available before you start a session. TTT communicates with external debuggers using the Debug Adapter Protocol (DAP).

## Set up an adapter

Install the debugger executable for your language and add its command to `debug.adapters` in your user settings. This registry starts empty. Adapter configuration plugins can supply these settings. For example, with GDB built with DAP support on your PATH, add:

```json
{
  "debug": {
    "adapters": {
      "gdb": {
        "command": ["gdb", "-q", "-nx", "-i", "dap"],
        "languages": ["c", "cpp"]
      }
    }
  }
}
```

The adapter name (here, `gdb`) is the `type` used in launch configurations. The compatibility suite also exercises CodeLLDB with C, C++ and Rust, LLVM lldb-dap with C, Delve with Go, js-debug with JavaScript and source-mapped TypeScript, debugpy with Python, NetCoreDbg with C#, and PHP Debug with Xdebug. Adapter-specific hooks and transport options are described in the [settings reference](/reference/settings/#debug-adapters).

## Launch a program

Choose **Configure** in the sidebar to create and open `.ttt/launch.json`. For a C program built with debug symbols as `bin/example`, use:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Debug example",
      "type": "gdb",
      "request": "launch",
      "program": "${workspaceFolder}/bin/example"
    }
  ]
}
```

Choose **Run & Debug** to run a workspace target. If several are configured, TTT opens a target picker. TTT reads `.vscode/launch.json` when the workspace has no `.ttt/launch.json`. Build compiled targets first. TTT saves source changes before launching and does not run VS Code build tasks.

**Debug File** uses the adapter’s `fileLaunch` recipe for the active file. It works independently of the selected workspace target and is enabled when the adapter declares the file’s language and supplies a recipe.

## Breakpoints and session controls

Click the left half of the line-number gutter to toggle a breakpoint, or use **Toggle Breakpoint** from the command palette. Breakpoints apply to the adapter’s configured languages. TTT remembers breakpoint lines per workspace. A hollow marker indicates that a breakpoint needs confirmation or the adapter has yet to verify it. The Breakpoints section shows any location or explanation returned by the adapter. When the program stops, the execution marker shows the current line.

Use **F5** to start debugging or continue a paused session and **Shift+F5** to stop. Select a frame in **Call Stack** to inspect its **Variables**. Expand a value to see its children, or right-click it to copy its name or value. Drag the dividers to resize the sidebar sections. Use **F10** to step over, **F11** to step in, and **Shift+F11** to step out. The sidebar also provides pause, continue, and stop controls.

While debugging, the floating toolbar provides stepping and stop controls. Use its menu or the Debug sidebar menu to dock it in any corner or hide it. When multiple threads are paused, select a thread in Call Stack to inspect it.

The Output panel shows output sent by the adapter under `debug:<target name>`. A program launched in a terminal can use that terminal for input and output.

## Attach to a running program

Add a target with `"request": "attach"`. GDB accepts `"pid": "${command:pickProcess}"` to show the process picker or a numeric PID to select a known process. Other adapters use `processId`. Address attach uses adapter-specific fields, such as debugpy’s `connect` object. A PID attach requires the operating system to permit debugging that process. Choose **Detach** to leave an attached program running.

## Debug TTT itself

Install Delve and configure the `go` adapter with the `delve` provider. Open this repository and choose **Run & Debug → Launch ttt**. The checked-in target starts a separate interactive editor in an external terminal, using `.config/ttt-debug` in the workspace for its settings.
