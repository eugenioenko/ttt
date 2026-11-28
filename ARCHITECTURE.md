# Architecture

This document describes ttt's architecture and its dependency boundaries. It is
the source of truth for package ownership. `AGENTS.md` summarizes the
constraints agents most often need while working in the repository.

## Current shape

ttt is an acyclic Go application, but it is not a linear layer stack. The
application and presentation packages compose several independent foundations:

```text
cmd/ttt
  -> app
     -> ui -> widgets
     -> plugin -> widgets
     -> scm (git, jj), github, lsp, search, terminal, watcher,
        workspace, trash, update
     -> render, term, view, textwidth, highlight

render, view -> term
highlight    -> term, core/buffer
markdown     -> highlight, term, textwidth
widgets      -> term, textwidth, markdown, image, icons, i18n,
                core/clipboard, core/selection
git, jj      -> scm
lsp          -> jsonrpc, config
```

Architectural pressure is concentrated in `internal/app`, `internal/ui`, and
`internal/widgets`, which hold most of the production code. The domain and
service packages are bounded and reusable. This favors incremental cleanup of
blurred boundaries over a rewrite.

## Dependency zones

The zones describe ownership and dependency direction. A zone need not map to
exactly one package.

### Domain

Packages: `internal/core/*`.

Owns editor algorithms and state: buffers, cursors, selections, undo, folds,
multicursors, and diff algorithms. Domain code must not start processes, access
terminal state, render widgets, or coordinate application lifecycle.
`internal/core/architecture_test.go` enforces part of this: core must not import
`highlight`, `term`, `render`, `view`, `ui`, `widgets`, or tcell.
`core/clipboard` starts processes and is an existing exception, not a pattern.

### Shared foundations

Packages: `internal/config` and `internal/filewrite`.

`internal/config` owns settings, keybindings, themes, and `.editorconfig`. It is
imported by services (`lsp`), presentation, and the application.

`internal/filewrite` owns atomic file replacement (temp-file staging, sync,
replace, cleanup) for buffer and config, using only the standard library.
Callers own serialization, symlink resolution, metadata, retries, and recovery.

### Services

Packages: `internal/scm`, `internal/git`, `internal/jj`, `internal/github`,
`internal/dap`, `internal/jsonrpc`, `internal/lsp`, `internal/search`, `internal/terminal`,
`internal/watcher`, `internal/workspace`, `internal/trash`, and
`internal/update`. Test helpers: `internal/gittest`, `internal/jjtest`.

Own external-process and external-state integration. Services expose typed
operations and results without owning widgets. Long-running work must have
explicit cancellation, identity, shutdown, and stale-result behavior.

#### Source control

`internal/scm` defines provider contracts; `internal/git` and `internal/jj` are
the built-in adapters, selected by `scm.Providers`.

- Discovery, status, identity, and staging go through the provider. Changes are
  typed and keep their provider group identity; the Changes panel keeps its
  staged/unstaged projection without converting back to Git status.
- `scm.BaselineReader` feeds editor gutters and editable diffs. Missing,
  ignored, and conflicted content are explicit states; failed reads are errors.
- `scm.ChangeReader` feeds single-file diffs and aggregate Current Changes.
- `scm.HistoryReader` and `scm.RevisionReader` feed history and Commit Details
  with opaque entry and revision IDs. Historical content is loaded lazily.
- Optional `scm.Committer` and `scm.Discarder` cover providers without staging.
- Provider-specific details (Git index/HEAD resolution, conflict sides, rename
  and binary patch metadata) stay inside the adapter.
- `RepositoryState` in `internal/app` owns async lifecycle: repository objects
  are captured on the main thread, and background reads keep cancellation,
  coalescing, and stale-result guards.

Remaining work:

- Pull, push, sync, new branch, blame, GitHub PRs/permalinks, and worktree tools
  stay Git-specific until a provider needs a shared contract.
- Provider-defined groups, labels, and actions, and plugin-supplied providers,
  are future host work.

### Presentation kernel

Packages: `internal/term`, `internal/render`, `internal/view`,
`internal/textwidth`, `internal/widgets`, `internal/markdown`,
`internal/highlight`, `internal/i18n`, `internal/asynctask`, `internal/image`,
`internal/icons`, and `internal/fileicons`.

Owns screen cells, styles, width measurement, rendering, layout, focus,
scrolling, reusable interaction primitives, and syntax-to-style projection.
`internal/widgets` holds primitives with more than one product or plugin
consumer; it is not a destination for every visually reusable product surface.

`internal/i18n` owns the embedded English catalog, runtime catalogs, placeholder
validation, and named interpolation. The application loads the selected catalog
before constructing widgets; the plugin host owns language-pack contributions
and commit-pinned catalog downloads, which never execute Lua.

`internal/asynctask` provides terminal-independent scheduling with injected
execution and delivery. Widgets may use it directly; the application owns
service coordination and task shutdown.

### Product presentation

Package: `internal/ui`.

Owns editor, diff, search, terminal, tab, panel, and dialog surfaces. It may
depend on the presentation kernel and domain models. It should receive service
results and emit intents rather than own subprocess or filesystem lifecycle;
existing direct I/O is migration work, not precedent.

Render-time geometry is the single source of truth for hit testing and
interaction routing.

### Application

Packages: `internal/app` and `internal/command`.

Owns composition, commands, async lifecycle, cancellation, request identity, and
service coordination. An App subsystem owner is valuable only when it owns the
behavior and lifecycle behind a narrow API; moving fields into nested structs
while other code still mutates them is not decomposition.

`app.NewTask` registers tasks for shutdown. `App.Init` wires UI delivery for
existing and future tasks. Uninitialized/headless Apps, including the E2E
harness, run tasks synchronously.

`internal/app/callbacks.go` is reserved for callback registration: it connects
widgets to application handlers. New callbacks must keep feature implementations
in their feature files (for example, `search_replace.go`, `diff_open.go`, `pr.go`,
and `changes_menus.go`). Keep small forwarding closures with the registrations;
place new menu construction, file operations, and async lifecycle behavior in
named handlers in the owning feature files.

Existing exceptions remain in the registration closures: editor-tab, Explorer,
sidebar overflow, bottom-panel, and terminal menus, plus panel and focus routing.
The mechanical handler split preserved these inline bodies; extracting them into
their owning feature files is separate cleanup. They are not precedent for new
feature logic in `callbacks.go`.

### Plugin host

Package: `internal/plugin`.

Owns the Lua runtime, permissions, registry, descriptors, callbacks, and
adapters into the presentation kernel. Public Lua behavior is a compatibility
boundary: internal widget changes must preserve plugin semantics or ship an
explicit migration.

### Platform

Package: `cmd/ttt`.

Owns process startup and concrete composition. Breadth here is not a defect;
product behavior and reusable algorithms do not belong here.

## Known boundary violations

These describe current code that conflicts with the zones. They do not authorize
similar dependencies.

| Violation                                                                                                                                                                                                                                   | Rule until fixed                                            | Exit condition                                                                    |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------- | --------------------------------------------------------------------------------- |
| `internal/ui` still owns I/O: `search_widget.go` spawns `rg` (argument building and parsing already live in `internal/search`), `dirloader.go` reads directories and calls `git` for ignored files, and `editor_group_save.go` reads directories | Add no new process or filesystem lifecycle to `internal/ui` | UI receives typed results and emits intents through an application or service API |
| `internal/ui` imports `internal/git` directly (`dirloader.go`, `changes_helpers.go`)                                                                                                                                                        | New UI code uses `scm` types, not `git`                     | No `internal/git` import in `internal/ui`                                         |
| Two scrollbar implementations: `internal/ui/scrollbar.go` and `internal/widgets/scrollbar.go` (issue #1462)                                                                                                                                 | Fix shared invariants in both until consolidated            | One implementation remains                                                        |
| App async state is spread across `App` and feature files (language features, terminal, plugin/settings)                                                                                                                                     | Add new async state only behind an explicit owner           | Those subsystems own their timers, cancellation, identities, and shutdown         |

## Architecture decisions

### tcell ownership

`internal/term` owns the screen abstraction and concrete screen. tcell event
types are intentionally used by the presentation kernel, product UI, and narrow
application/platform event wiring. Domain and service packages must not import
tcell. A custom event abstraction is worth reconsidering only for another
backend, a deterministic boundary tcell prevents, or measured migration value
(issue #495).

### Highlight ownership

`internal/highlight` owns grammar selection, line caches, and projection from
TextMate scopes to `term.Style` behind the `Highlighter` and `Span` API.
Tokenization uses [textmate-go](https://github.com/eugenioenko/textmate-go),
whose rule stack carries across lines, so multi-line constructs need no special
handling. Its incremental `Document` keeps states above an edit and reuses the
unchanged tail once state converges.

### Generic widgets and product surfaces

A component moves from `internal/ui` to `internal/widgets` only when its state
and behavior can be expressed without product policy or service lifecycle.

### Shared input

`widgets.InputWidget` is the single input implementation for product surfaces
and plugins. Owners configure actions and callbacks and route focus; the input
owns editing, selection, history, scrolling, rendering, and hit testing.

### Shared diff

Every diff tab and each file in Commit Details use `ui.DiffEditorWidget` with a
`DiffOverlay`. Surfaces with different interaction or lifecycle stay separate
product surfaces over shared models, not one highly conditional widget.

### Application owners

A subsystem owner must establish:

- a typed API for commands and results;
- ownership of timers, goroutines, cancellation, and shutdown;
- request identity and stale-result rules;
- a single mutation boundary on the main event loop where UI state is involved;
- tests at the smallest deterministic boundary that proves those contracts.

`RepositoryState` is the reference direction, not a template to copy
mechanically.
