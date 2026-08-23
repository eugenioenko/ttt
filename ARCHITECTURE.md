# Architecture

This document describes ttt's current architecture, its intended dependency
boundaries, and the incremental program for improving them. It is the source of
truth for package ownership. `CLAUDE.md` summarizes the constraints agents most
often need while working in the repository.

## Current shape

ttt is an acyclic Go application, but it is not a linear
`core -> view -> render -> term -> ui` stack. The application and presentation
packages compose several independent foundations:

```text
cmd/ttt
  -> app
     -> ui -> widgets
     -> plugin -> widgets
     -> git, github, lsp, terminal, watcher, workspace
     -> render, term, view, textwidth

render -> term
view   -> term
widgets -> term, textwidth, markdown, core/clipboard, core/selection
core/highlight -> term (known boundary violation)
```

The main architectural pressure is concentrated in `internal/app`,
`internal/ui`, and `internal/widgets`. The lower domain and service packages are
generally bounded and reusable. This favors incremental replacement of blurred
boundaries over a greenfield rewrite.

### Baseline

Recorded on 2026-08-23 at `65fc238`:

| Package | Production Go files | Production LOC | Direct internal package dependencies | Direct tcell importers |
|---|---:|---:|---:|---:|
| `internal/app` | 54 | 17,710 | 19 | 22 |
| `internal/ui` | 59 | 19,393 | 17 | 37 |
| `internal/widgets` | 30 | 6,608 | 5 | 25 |
| `internal/plugin` | 25 | 5,898 | 3 | 5 |

`internal/app` and `internal/ui` contain 37,103 of 57,962 production Go
lines under `internal` (64.0%). These measurements indicate concentration, not
an automatic mandate to split packages. Re-record them after each completed
tranche using the structural review commands below.

Codemap 4.4.2 reported complete Go scan coverage for this baseline. That means
the configured files were scanned; it does not mean file-level importer data is
complete for same-package Go references.

## Dependency zones

The zones below describe ownership and dependency direction. They are not a
requirement that every zone map to exactly one package.

### Domain

Packages: `internal/core/*`.

Owns editor algorithms and state transitions: buffers, cursors, selections,
undo, folds, multicursors, and diff algorithms. Domain code must not start
processes, access terminal state, render widgets, or coordinate application
lifecycle.

`internal/core/highlight` currently returns `term.Style`. This is a known,
bounded violation, not an allowed dependency direction. New domain APIs must
not introduce additional presentation dependencies. The planned correction
will either expose semantic token kinds from the domain or reclassify
highlighting outside `internal/core`.

### Services

Packages: `internal/git`, `internal/github`, `internal/lsp`,
`internal/terminal`, `internal/watcher`, and `internal/workspace`.

Owns external-process and external-state integration. Services expose typed
operations and results without owning product widgets. Long-running work must
have explicit cancellation, identity, shutdown, and stale-result behavior.

### Presentation kernel

Packages: `internal/term`, `internal/render`, `internal/view`,
`internal/textwidth`, `internal/widgets`, and `internal/markdown`.

Owns screen cells, styles, terminal-width measurement, rendering, layout,
focus, scrolling, and reusable interaction primitives. `internal/widgets`
contains primitives with more than one product or plugin consumer; it is not a
destination for every visually reusable product surface.

`tcell` is the accepted event and terminal implementation dependency for this
zone. Replacing it with one-for-one local event wrappers would add indirection
without creating a meaningful backend boundary.

### Product presentation

Package: `internal/ui`.

Owns editor, diff, search, terminal, tab, panel, and dialog surfaces. Product UI
may depend on the presentation kernel and domain models. It should receive
service results and emit intents rather than own subprocess or filesystem
lifecycle. Existing direct I/O is migration work, not precedent for new UI I/O.

Direct `tcell` event use is allowed here. Render-time geometry remains the
single source of truth for hit testing and interaction routing.

### Application

Packages: `internal/app` and `internal/command`.

Owns composition, commands, async lifecycle, cancellation, request identity,
and service coordination. An App subsystem owner is valuable only when it owns
the behavior and lifecycle behind a narrow API. Moving fields into nested
structs while unrelated methods continue to mutate them is not decomposition.

Application code may route `tcell` events where it composes the event loop and
UI, but new feature state should not depend directly on terminal event types.

### Plugin host

Package: `internal/plugin`.

Owns the Lua runtime, permissions, registry, descriptors, callbacks, and
adapters into the presentation kernel. The public Lua behavior is a
compatibility boundary. Internal widget consolidation must preserve plugin
semantics or introduce an explicit compatibility migration.

### Platform

Package: `cmd/ttt`.

Owns process startup and concrete composition. It may import multiple internal
packages to wire the application. Breadth at the composition root is not by
itself an architectural defect; product behavior and reusable algorithms do
not belong there.

## Known boundary violations

Known violations describe current code that conflicts with the target zones.
They do not authorize similar dependencies. Each violation must have a bounded
cleanup lane, characterization evidence, and an exit condition.

| Violation | Freeze rule | Cleanup lane | Exit condition |
|---|---|---|---|
| `internal/core/highlight` returns `term.Style` | Add no new domain-to-presentation dependencies | HL1 -> HL2 | Domain emits semantic highlighting or the package is reclassified outside `core` |
| Product UI directly owns some search subprocess and filesystem discovery work | Add no new process or filesystem lifecycle to `internal/ui` | IO1 -> IO2 | UI receives typed results and emits intents through an application or service API |
| App state and lifecycle are spread across the root `App` and feature files | Add new async state only behind an explicit owner or with a named follow-up owner | AL, AT, AP | Selected subsystems own their timers, cancellation, identities, mutation boundary, and shutdown |
| Input, scrollbar, and diff projection behavior is duplicated | Fix shared invariants in every affected path until consolidation is complete | S1/S2, I1/I2/I3, D1-D4 | One canonical model remains for each shared contract and old paths are deleted |

Direct `tcell` use in presentation packages is not listed as a violation. It is
an explicit boundary decision below. Issue #495 is resolved by documenting and
enforcing that decision rather than constructing a one-for-one event wrapper.

## Architecture decisions

### tcell ownership

`internal/term` owns the screen abstraction and concrete screen
implementation. `tcell` event types are also intentionally used by the
presentation kernel, product UI, and narrow application/platform event wiring.
Domain and service packages must not import `tcell`.

This is the current decision for issue #495. A custom event abstraction should
be reconsidered only if another backend, a deterministic boundary that tcell
prevents, or measured migration value justifies it.

### Generic widgets and product surfaces

`internal/widgets` owns reusable interaction and layout primitives.
`internal/ui` owns product-specific composites. A component moves to
`internal/widgets` only when its state model and behavior can be expressed
without importing product policy or service lifecycle.

### Shared diff behavior

Diff surfaces should share pure projection, layout, styling, and anchor models
where their contracts agree. They should remain separate product surfaces when
their interaction or lifecycle differs. The target is shared models with thin
surface adapters, not one highly conditional diff widget.

### Application owners

Subsystem owners must establish all of the following:

- a typed API for commands and results;
- ownership of timers, goroutines, cancellation, and shutdown;
- request identity and stale-result rules;
- a single mutation boundary on the main event loop where UI state is involved;
- tests at the smallest deterministic boundary that proves those contracts.

`RepositoryState` is the reference direction. It is not a template that must be
copied mechanically.

### Refactors and UX

User-facing work starts from the intended interaction and presentation.
Architecture refactors start from characterization tests and preserve existing
UX unless the PR explicitly declares a behavior change.

## Verification strategy

Choose the smallest deterministic layer that proves the intended invariant:

- Unit tests prove pure algorithms, state models, parsers, and lifecycle
  helpers.
- Go E2E tests prove composed App/UI behavior on `term.SimScreen`.
- Functional tests prove real-binary behavior through `--exec`.
- Live PTY integration tests prove terminal byte encoding, terminal modes, and
  external-process behavior that simulation cannot establish.
- Real language-server tests prove vendor compatibility; deterministic fake
  servers prove protocol and application semantics.

Do not duplicate the same assertion at every layer. Add broader coverage when
it proves a distinct boundary. During development, run focused tests for the
affected contracts; before merge, CI remains the broad regression gate.

## Architecture convergence program

Execution is tracked in issue #165. Each item should link its implementation
PR, adversarial review result, and merge commit.

### Preparation gate

Structural repairs begin only after the relevant preparation work is complete:

- P0: publish the truthful architecture contract and remove contradictory
  contributor guidance;
- P1: record the baseline dependency graph and package concentration with
  Codemap and `go list`;
- P2: maintain issue #165 as the dependency-aware execution tracker;
- P3: characterize each shared behavior before changing its implementation;
- P4: define the canonical model, ownership boundary, and deletion target for
  that repair lane.

P0-P2 are program-wide prerequisites. P3-P4 are lane-specific gates and can be
completed in parallel. They are not a reason to serialize unrelated work.

```text
P0 Truthful contract -> P1 Baseline -> P2 Execution tracker

P2
|-- [P3/P4] S1 Canonical scrollbar API
|              `-- S2 Migrate consumers and delete duplicates
|
|-- [P3/P4] I1 Shared text-input state model
|              `-- I2 Adapt internal/widgets input
|                    `-- I3 Adapt internal/ui inputs
|                          `-- AP Plugin/settings App owner
|
|-- [P3/P4] AL LanguageFeaturesState owner
|-- [P3/P4] AT TerminalState owner
|
|-- [P3/P4] HL1 Highlight/style characterization
|              `-- HL2 Semantic highlight boundary correction
|
|-- [P3/P4] D1 Diff-surface characterization
|              `-- D2 Pure shared projection/layout model
|
|-- [P3/P4] IO1 Search/file-discovery ownership characterization
|              `-- IO2 Move execution behind application/service APIs
|
|-- [P3/P4] T1 Theme/style/catalog validation
`-- [P3/P4] H1 PTY readiness fixture correction for issue #533

S2 + D2
  `-- D3  Migrate DiffViewWidget
        `-- D4  Migrate CommitDetailWidget and remove duplicate projection
```

### Parallel work

After P0-P2, the scrollbar, input-model, highlight, diff-foundation,
service-I/O, theme/catalog, and issue #533 lanes may prepare and proceed
independently. Language and terminal App owners are also semantically
independent, but should be serialized when they overlap `app.go`, construction,
callbacks, or shutdown wiring. Plugin/settings ownership follows input
migration. Diff-surface migration follows both the scrollbar and
shared-projection foundations.

Use separate PRs or small dependency stacks. Merge through `main`; do not use a
long-lived integration branch.

### Exit criteria

The program is complete when:

- documentation and imports agree on the dependency zones;
- duplicate scrollbar and input state machines are removed;
- shared diff projection is consumed by both diff surfaces without merging
  their distinct product behavior;
- selected App subsystems own their lifecycle rather than only grouping fields;
- UI-owned process and filesystem work has moved behind application or service
  APIs;
- issue #533 has deterministic behavior tests and a bounded real-PTY smoke;
- Codemap and `go list` show no new prohibited dependency directions;
- user-visible behavior and the plugin API remain compatible unless separately
  approved.

## Rewrite checkpoint

A larger presentation-shell experiment is not currently justified. Reconsider
only after at least three completed strangler tranches if App fan-out and async
state continue growing, UI cannot relinquish service I/O, old and new paths
require repeated duplicate fixes, or a plugin compatibility break is already
planned.

Any experiment is limited to ten working days and one narrow path. It must
prove behavioral parity, preserve public plugin semantics, delete the replaced
state machine, and reduce dependency or lifecycle complexity. Otherwise stop
and continue the incremental program.

## Structural review commands

Codemap is useful for cross-package direction and concentration, but it cannot
establish same-package Go coupling or runtime lifecycle behavior. Pair it with
source inspection and Go's package graph.

```sh
codemap skill show explore
codemap --depth 3 --only go .
codemap --deps --json --only go .
codemap --importers internal/textwidth/textwidth.go --json
codemap blast-radius --json --ref main .
go list -deps ./...
```

Treat zero file importers in a multi-file Go package as unknown, not safe.

Regenerate the baseline table with the following definitions. Production Go
files exclude `*_test.go`; LOC is physical `wc -l`; direct dependencies are
unique imports below this module's `internal/` path; direct tcell importers are
production files containing the tcell v3 import path.

```sh
module_path=$(go list -m)

for package_path in internal/app internal/ui internal/widgets internal/plugin; do
  production_files=$(rg --files "$package_path" -g '*.go' -g '!*_test.go')
  file_count=$(printf '%s\n' "$production_files" | wc -l)
  line_count=$(printf '%s\n' "$production_files" | xargs wc -l | tail -1 | awk '{print $1}')
  dependency_count=$(go list -json "./$package_path" | jq --arg prefix "$module_path/internal/" '[.Imports[] | select(startswith($prefix))] | unique | length')
  tcell_count=$(rg -l 'github.com/gdamore/tcell/v3' "$package_path" -g '*.go' -g '!*_test.go' | wc -l)
  printf '%s files=%s loc=%s internal_deps=%s tcell_importers=%s\n' "$package_path" "$file_count" "$line_count" "$dependency_count" "$tcell_count"
done

app_ui_loc=$(for package_path in internal/app internal/ui; do rg --files "$package_path" -g '*.go' -g '!*_test.go'; done | xargs wc -l | tail -1 | awk '{print $1}')
internal_loc=$(rg --files internal -g '*.go' -g '!*_test.go' | xargs wc -l | tail -1 | awk '{print $1}')
awk -v numerator="$app_ui_loc" -v denominator="$internal_loc" 'BEGIN { printf "app_ui=%d internal=%d percent=%.1f\n", numerator, denominator, 100*numerator/denominator }'
```
