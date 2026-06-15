# loom Focus Mode Wiring Design

**Date:** 2026-06-11
**Status:** Approved for planning
**Scope:** Render the already-designed Focus Mode + Status Rail layout, plus timestamped command feedback, a command-log overlay, and cursor-follow list scrolling.

## Context

The "Focus Mode + Status Rail" layout was approved (see `2026-06-11-loom-focus-status-rail-design.md`) and partially built: `topBar()`, `workflowTabs()`, `branchSummary()`, `commandStateText()`, `statusRailContent()`, and `renderStatusRail()` exist in `view.go` with passing tests. None of them are reachable from `View()`, which still renders the older three-panel stacked layout. This pass wires the designed layout into the render path and resolves the parts the prior spec left open.

This is not a new layout direction. The `internal/git` package does not change.

## Decisions

These were settled during brainstorming and drive the work:

1. **Layout:** finish wiring Focus Mode + Status Rail. Only the focused workflow's list renders in full; the other two appear as counts.
2. **Rail fallback:** auto-hide only. Below a width threshold the rail is dropped and command state shows in the top bar. No manual toggle, no rail-visibility field.
3. **Recent commands:** show timestamps (`HH:MM`). `cmdLog` carries a time per entry.
4. **Dead `x` key:** repurpose as a full command-log overlay. `showLog` finally drives a render path.
5. **List scrolling:** add cursor-follow scrolling so the selected row stays visible, with an overflow hint.

## Layout

`View()` assembles four regions, replacing the current three-panel stack.

```text
 main ↑2 ↓0 · [1 Files 3] 2 Branches 4 3 Commits 50 · Ready    top bar (1 row)
┌──────────────┬──────────────────────────┬───────────────┐
│ Files 3      │ Diff: view.go (unstaged) │ Status Rail   │
│ ▒ + view.go  │ @@ -10,6 +10,9 @@        │ Workflow …    │   body
│   panels.go  │ +const topBarHeight = 1  │ Command …     │
│ ? notes.md   │ -old line                │ Recent …      │
│              │ +new line                │ Selected …    │
└──────────────┴──────────────────────────┴───────────────┘
 space stage · d discard · c commit · ? help · q quit          footer (1 row)
```

### Top bar (1 row)

Renders `topBar()`: branch name with `(no branch)` fallback, ahead/behind counts, workflow tabs with counts, and a short command state (`Ready`, `Working...`, `Error`). When the rail is hidden, the top bar is the home for command state so nothing is lost.

### Body

Three regions when wide, two when narrow:

- **Active workflow list** (left): the focused workflow only — Files, Branches, or Commits — using the existing portable ASCII markers and the focused border style. The other two workflows are not rendered as lists; their counts remain in the top bar and rail.
- **Preview pane** (center): unchanged behavior — selected file diff, branch log, or commit detail in the existing `viewport`. Always the largest reading area.
- **Status rail** (right): read-only, rendered by `renderStatusRail()`.

### Footer (1 row)

The existing contextual `footerActions()` by focus and mode. Compact key hints only.

### Column widths

- Rail: fixed `statusRailWidth` (30), only when shown.
- List: clamped — minimum ~24 columns, roughly one quarter of the width.
- Preview: the remainder. It must stay the largest region.

## Responsive Fallback

A single width threshold governs the rail (the existing `minRailWindowWide = 110` constant):

- **Width ≥ 110:** list ‖ preview ‖ rail.
- **Width < 110:** list ‖ preview. The rail is dropped; command state stays visible in the top bar.

Fallback priority below that remains: keep the active list usable, keep the preview usable, keep the footer within terminal height. No manual rail toggle and no mouse dependency.

## Status Rail Content

Unchanged from the approved rail design — this pass only gives `renderStatusRail()` a render slot. Stable section order:

```text
Status Rail

Workflow
Files: 3 changed
Branches: 4 local
Commits: 50 loaded

Command
Ready
Last: 10:21 git fetch

Recent
10:22 git commit
10:21 git add README.md
10:20 git fetch

Selected
unstaged file
actions: stage, discard
```

## Command Feedback With Timestamps

- Change `cmdLog []string` to `cmdLog []cmdEntry`, where `cmdEntry` holds `at time.Time` and `text string`.
- `update.go` appends `cmdEntry{at: time.Now(), text: msg.cmd}` when a command completes.
- The time is stored, not pre-formatted. A pure formatter renders `HH:MM text` at view time so it stays testable with an injected time.
- The rail "Command" (`Last:`) and "Recent" (last 3, newest first) sections render the timestamped form, as does the command-log overlay.

Consistent with the Focus Mode direction, routine command feedback lives in the rail, not the footer. The footer carries key hints only. When the rail is hidden (narrow width), the top bar carries command **state** (`Ready`/`Working...`/`Error`); on error, the footer may still show a short error summary so failures are not hidden. This is not a notification framework — the only new data is a timestamp per command.

## Command-Log Overlay

The previously dead `x` key (which flipped `showLog`, a field nothing read) is repurposed:

- `x` toggles a full-screen overlay built like the existing help overlay.
- The overlay shows the full command history, timestamped, newest first.
- It closes on `x` or `esc`, and is mutually exclusive with the help overlay.
- `showLog` now drives this render path instead of being dead state.

## List Scrolling

The active workflow list scrolls so the selection stays visible:

- Add `scroll map[Panel]int` to the model, updated when the cursor moves within a panel, clamping the offset so the cursor stays inside the visible window.
- A pure windowing helper in `panels.go` takes `(lines, cursor, offset, height)` and returns the visible slice plus an overflow indicator. When rows are clipped, the list shows an `… +N more` hint.
- Manual offset math; no new dependency (not `bubbles/list`). The helper is pure so it can be unit-tested without a terminal.

## Component Boundaries

Work stays inside `internal/ui`. `internal/git` does not change.

| File | Change |
|------|--------|
| `view.go` | Focus Mode `View()` assembly; top-bar wiring; rail show/hide at width 110; overlay render; footer |
| `panels.go` | pure list-windowing helper + overflow hint; existing row formatting and styles kept |
| `model.go` | `cmdLog` becomes `[]cmdEntry`; add `scroll map[Panel]int`; `showLog` now meaningful |
| `update.go` | append timestamped entry; update scroll offset on cursor move; `x` toggles the overlay |
| `internal/git` | untouched |

## Error Handling

Existing git errors continue flowing through `errMsg`/`gitDoneMsg`. Errors remain visible in the rail (and in the top bar when the rail is hidden) until replaced by a successful command or a newer error. Confirmed destructive actions are unchanged. Non-crashing behavior is preserved.

## Testing

Prefer pure-function and state-derived tests over full-screen golden snapshots.

- **List windowing:** cursor at top, middle, and bottom; overflow hint present when clipped; no hint and no scroll when the list fits.
- **Timestamp formatting:** rail "Recent" and footer "last:" render `HH:MM text` from an injected fixed time, not `time.Now()`.
- **Responsive rail:** hidden below width 110, shown at/above 110; layout stays within terminal height and the footer remains usable.
- **Overlay:** `x` toggles it; it renders the full timestamped history; `esc`/`x` close it; it is mutually exclusive with help.
- **Existing helpers:** `topBar`, `workflowTabs`, `statusRailContent`, and selected-context tests continue passing.
- **Existing `cmdLog` tests:** updated to the new `cmdEntry` type.

## Non-Goals

- No new git commands.
- No hunk or line staging.
- No command palette.
- No mouse support.
- No theming or config system.
- No rich commit graph.
- No full remote command-output viewer.
