# RFC: Tab label length limits

```
Status: Draft
Issue: #1285
Date: 2026-11-07
```

## Summary

Long tab names take up most of the tab bar. A set of 16 realistic tabs needs 375 columns today, so a 120-column terminal shows about four of them.

This RFC proposes cutting inactive tab names to **12 terminal columns**, keeping the start of the name and its last extension (`CONTRIBU….md`). The active tab shows its full name, up to 32 columns. When two cut names would look the same, both are shown uncut. File type decorations and diff suffixes do not count toward the limit.

## Motivation

ttt shows no file name anywhere except the tab: there is no breadcrumb, no window title and no file name in the status bar. Tabs therefore show the full base name, however long it is. A few long names push everything else off screen:

```
                         ╭────────────────────────────────────────────────────╮
    定ファイルの説明.md │  CONTRIBUTING_GUIDELINES_FOR_NEW_MAINTAINERS.md ✕ │  index.ts (staged)  ● untitled      ⋮
╭────────────────────────╯                                                    ╰────────────────────────────────────────╮
```

*120 columns, 14 tabs open. Three and a half are visible; the rest are behind the scroll arrows.*

## Inventory

### How a label is built

`syncTabs` in `internal/ui/editor_group.go` turns each `editorTab` into a `ui.Tab`. The name is the tab's `Title` if set, otherwise its `FilePath`, plus ` (diff)` when the tab is showing an editable diff. `tabLabelParts` in `internal/ui/tabbar_widget.go` then takes `filepath.Base` of that name and adds the decorations. `drawTabLabel` draws the result.

### Name sources

| Tab kind | Label | Source |
|---|---|---|
| File | base name: `main.go`, `.env`, `Makefile`, `types.d.ts`, `設定ファイルの説明.md` | `FilePath` |
| Untitled buffer | `untitled`, `untitled-2`, … | `nextUntitledName` (`editor_group.go`) |
| Open Changes (editable diff on a file tab) | `main.go (diff)` | `syncTabs` appends ` (diff)` |
| Working tree diff from the Changes panel | `main.go (diff)` | `callbacks.go`, `diffOpenResult.TabName` |
| Staged diff | `index.ts (staged)` | same, ` (staged)` suffix |
| File diff from a commit | `main.go @ 825eb7d8` | `callbacks.go`, `diffOpenResult.Title` |
| Commit Details | `Commit 825eb7d8` | `changes.commit` in `en.json` |
| Current Changes | `Current Changes`, or `Changes · repo` in a multi-root workspace | `currentChangesTitle` |
| Pull request | `PR #1058: feat(plugin): ttt.rpc for agent control` | `status.pr` in `en.json`, the PR title from GitHub |
| Settings, Welcome | `Settings`, `Welcome` | `general.settings`, `welcome.welcome` |
| Plugin tabs and read-only plugin buffers | any title the plugin passes | `OpenPluginTab`, `OpenBufferReadOnly` |

Translated titles can be much longer than the English ones. `Current Changes` is 15 columns; a German translation such as `Aktuelle Änderungen` is 19.

### Decorations

Each label is `" " + [dirty] + [read-only] + [icon] + name + [read-only text] + [close or pin] + " "`. The active tab also gets two border columns.

| Decoration | Nerd Font mode | Plain mode | Columns |
|---|---|---|---|
| Dirty marker | `● ` | `● ` | 2 |
| Read-only | lock glyph + space, before the icon | ` (readonly)` after the name | 2 / 11 |
| File icon | glyph + space | none | 2 / 0 |
| Close (active, unpinned) | ` ✕` | ` ✕` | 2 |
| Pin (active, pinned) | ` ` + pin glyph | ` ♦` | 2 |

```
 ╭──────────────────────────╮
 │ LICENSE.txt (readonly) ♦ │ main.go
╭╯                          ╰───────────
```

*Plain mode, pinned read-only tab: the read-only text alone is 11 columns.*

### Findings along the way

1. **Titles are passed through `filepath.Base`.** That's correct for file paths, but it also runs on PR titles, plugin titles and commit titles. A PR named `fix(ui/tabs): clamp width` shows as `tabs): clamp width`.
2. **The ` (diff)` and ` (staged)` suffixes are hardcoded English** (`callbacks.go:426`, `editor_group.go:3135`). They are also part of the tab's identity (`TabName`), so the label and the key can't change independently.
3. **Inactive pinned tabs show no pin.** The pin glyph replaces the close button, which only the active tab has. Pinned tabs are recognizable only by sitting first.
4. **Duplicate names are indistinguishable.** `src/api/index.ts` and `src/components/settings/index.ts` both show `index.ts`. Nothing adds a folder hint.

## Options

All options cut only the name. The decorations and the ` (diff)`/` (staged)` suffix stay intact. Width is measured with `textwidth`, so a fullwidth CJK rune counts as two columns and is never split.

- **End:** keep the start: `CONTRIBUTIN…`.
- **Middle:** keep the start and the end: `CONTRI…RS.md`.
- **Stem:** keep the start of the stem and the last extension: `CONTRIBU….md`. If there's no stem (`.env`) or the extension is wider than half the limit, fall back to End.

Columns used by the same 16 tabs (the 14 above plus a PR and a Commit Details tab), inactive, Nerd Font icons:

| Limit | None | End | Middle | Stem |
|---|---|---|---|---|
| 8 | 375 | 202 | 203 | 203 |
| 10 | 375 | 220 | 219 | 221 |
| 12 | 375 | 235 | 236 | 236 |

The strategies use the same space; the choice is about which part of the name stays readable.

Screenshots: 120 columns, the same 14 tabs, `CONTRIBUTING_GUIDELINES_FOR_NEW_MAINTAINERS.md` active. The tab row scrolls to keep the active tab in view, so the left edge differs slightly between shots.

**End**

```
8   pr…   user_pr…   types.d…   .env   Makefile   設定フ… │  CONTRIB… ✕ │  index.ts (staged)  ● untitled      ⋮
10  ser_prof…   types.d.ts   .env   Makefile   設定ファ… │  CONTRIBUT… ✕ │  index.ts (staged)  ● untitled      ⋮
12  ofil…   types.d.ts   .env   Makefile   設定ファイ… │  CONTRIBUTIN… ✕ │  index.ts (staged)  ● untitled      ⋮
```

Loses the extension, so `types.d…` gives no hint of the file type in plain mode.

**Middle**

```
8   ts   user….ts   type….ts   .env   Makefile   設定….md │  CONT….md ✕ │  index.ts (staged)  ● untitled      ⋮
10  user_…t.ts   types.d.ts   .env   Makefile   設定….md │  CONTR…S.md ✕ │  index.ts (staged)  ● untitled      ⋮
12  t.ts   types.d.ts   .env   Makefile   設定フ…明.md │  CONTRI…RS.md ✕ │  index.ts (staged)  ● untitled      ⋮
```

Keeps the extension, but spends columns on the last few stem characters (`…RS.md`, `…明.md`), which rarely help recognize a file.

**Stem**

```
8   ts   user….ts   type….ts   .env   Makefile   設定….md │  CONT….md ✕ │  index.ts (staged)  ● untitled      ⋮
10  er_p….ts   types.d.ts   .env   Makefile   設定フ….md │  CONTRI….md ✕ │  index.ts (staged)  ● untitled      ⋮
12  ….ts   types.d.ts   .env   Makefile   設定ファ….md │  CONTRIBU….md ✕ │  index.ts (staged)  ● untitled      ⋮
```

Keeps the most of the start, which is what people read first, and always shows the type.

**Stem, 12 columns, active tab uncut**

```
   .ts   types.d.ts   .env   Makefile   設定ファ….md │  CONTRIBUTING_GUIDELINES_FOR_NEW_MAINTAINERS.md ✕ │    ⋮
```

Since the tab is the only place the name appears, the active tab should show it. Uncapped, though, one long name takes 52 of 120 columns, so the active tab needs a cap too.

### Problems every strategy shares

- **Collisions.** `user_profile_settings_controller.test.ts` and `user_profile_settings_service.test.ts` cut to the same label under every strategy and limit (`user_pro….ts` with Stem at 12). Their names only differ after 22 columns.
- **Duplicates.** `index.ts` and `index.ts` are identical before any cutting (finding 4).
- **Non-file titles.** `PR #1058: feat(plugin): ttt.rpc for agent control` has no meaningful extension; `filepath.Ext` returns `.rpc for agent control`. Stem falls back to End (`PR #1058: f…`), which is right, but it's better to choose End explicitly for tabs that aren't files.

## Recommendation

1. **Limit: 12 columns** for the name of an inactive tab. 8 cuts common names like `README.md` and `package.json`, and 10 still cuts `package.json` while saving only 15 more columns across 16 tabs.
2. **Strategy: Stem** for file tabs, **End** for titled tabs (PR, commit, plugin, settings).
3. **Active tab: uncut up to 32 columns**, cut with the same strategy beyond that. The clicked tab grows to the right, so it stays under the pointer; only the tabs to its right shift.
4. **Collisions: show them uncut.** After cutting, if two tabs in the group have the same label, both use the active-tab rule instead (up to 32 columns). This keeps the rule local and predictable. Exact duplicates are out of scope (see Open questions).
5. **Decorations and suffixes don't count.** The limit applies to the name only, so tab widths stay predictable: a dirty inactive file tab is at most `1 + 2 + 2 + 12 + 1 = 18` columns. The ` (diff)` and ` (staged)` suffixes stay whole.

### Implementation notes

- Split the label from the identity: `ui.Tab` gets a separate `Detail` field for ` (diff)`, ` (staged)` and ` @ <sha>`, and an `IsFile` flag. Only file tabs go through `filepath.Base`, which also fixes finding 1. Move the suffix text to `en.json` (finding 2).
- Add `shortenTabName(name string, isFile bool, maxCols int) string` next to `tabLabelParts`, built on `textwidth.Rune`, never splitting a fullwidth rune.
- The collision pass needs every label, so it runs in `Render` before the span loop, not in `tabLabelParts`.
- Click hit-testing already reads `tabSpans` from the last `Render`, so it picks up the shorter widths without changes.
- Tests:
  - unit tests for `shortenTabName`: ASCII, CJK at the cut point, dotfiles, multi-part extensions (`.test.ts`, `.d.ts`), no extension, a title with `/`;
  - the collision rule;
  - an E2E tab bar snapshot at 120 columns.

## Open questions

1. **Exact duplicates.** Add a muted parent folder (`index.ts · api`) the way VS Code does? That's a separate change with its own width rules.
2. **Hover to show the full name** (from #1285's follow-up). With cut names, a hover tooltip or a status bar hint for the hovered tab becomes more useful.
3. **Plain-mode read-only text.** ` (readonly)` is 11 columns. Replace it with a short marker in plain mode, as Nerd Font mode does with the lock glyph?
4. **Pinned inactive tabs** (finding 3). Should they show the pin glyph, and does it count toward the limit?
5. **A setting.** Expose `tabs.maxNameWidth` (0 to turn cutting off), or keep it fixed until someone asks?
6. **Integrated terminal tabs**, which are moving into the editor tab bar, have labels like `bash` or a command line. They should probably use the End rule like other titled tabs.
